package agents

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"slices"
	"strings"
	"time"
)

// mcpDriver invokes an MCP (Model Context Protocol) server as a swarm
// peer. Per spec D7 the server exposes a single agreed-upon tool —
// `analyze` (overridable via tool_name) — that takes the preset name,
// severity vocabulary, and input body, and returns a JSON array of
// findings conforming to the preset's severity vocabulary.
//
// Use case: deterministic static analyzers (semgrep, eslint, custom
// in-house tools) participate in the swarm alongside LLM peers. When
// the analyzer agrees with an LLM at issue+ severity, that's high-
// signal — much higher than any single LLM finding.
//
// v0.6 Stage 6 ships stdio transport only; http transport deferred to
// v0.6.x once the protocol stabilizes upstream.
//
// All MCP findings are billed at zero cost (Result.CostUSD = 0).
type mcpDriver struct {
	provider *Provider
}

func (d *mcpDriver) Name() string       { return d.provider.Name }
func (d *mcpDriver) Driver() DriverKind { return DriverMCP }

const (
	mcpDefaultToolName = "analyze"
	mcpStdioTimeout    = 60 * time.Second
)

// AnalyzeArgs is the payload the MCP server's `analyze` tool receives.
// Surfaced as a public type so stub servers in test fixtures can
// import and decode it without redefining the contract.
type AnalyzeArgs struct {
	Preset        string   `json:"preset"`
	SeverityVocab []string `json:"severity_vocab"`
	Input         string   `json:"input"`
}

// Invoke dispatches to the configured MCP server. Stage 6 only
// implements stdio transport; http surfaces as a clear "not
// implemented" error.
func (d *mcpDriver) Invoke(ctx context.Context, prompt string, opts InvokeOpts) (Result, error) {
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	switch d.provider.Transport {
	case "stdio":
		return d.invokeStdio(ctx, prompt)
	case "http":
		return d.invokeHTTP(ctx, prompt)
	}
	return errResult(DriverMCP, 0, fmt.Sprintf("provider %q (mcp): unsupported transport %q (allowed: stdio, http)", d.provider.Name, d.provider.Transport))
}


// invokeHTTP implements MCP over HTTP per the 2025-11-25 Streamable
// HTTP transport — synchronous JSON response only (SSE streaming
// deferred to v0.7). Each JSON-RPC message is POSTed to the configured
// endpoint and the response read inline.
//
// Headers: HeadersLiteral entries are sent as-is; Headers entries
// resolve env-var indirection at invoke time (header-name → value of
// the env var named in Headers[header-name]).
func (d *mcpDriver) invokeHTTP(ctx context.Context, prompt string) (Result, error) {
	if d.provider.Endpoint == "" {
		return errResult(DriverMCP, 0, fmt.Sprintf("provider %q: http transport requires endpoint field", d.provider.Name))
	}
	timeout := mcpStdioTimeout
	if d.provider.TimeoutSec > 0 {
		timeout = time.Duration(d.provider.TimeoutSec) * time.Second
	}
	subCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	start := time.Now()

	httpRPC := &mcpHTTPClient{
		ctx:      subCtx,
		endpoint: d.provider.Endpoint,
		headers:  buildMCPHeaders(d.provider),
	}

	// 1. initialize
	const initID = 1
	initResp, err := httpRPC.call(initID, "initialize", map[string]any{
		"protocolVersion": "2025-11-25",
		"clientInfo":      map[string]string{"name": "jutsu", "version": "0.6.1"},
		"capabilities":    map[string]any{},
	})
	if err != nil {
		return errResult(DriverMCP, time.Since(start), fmt.Sprintf("initialize: %v", err))
	}
	if initResp.ID != initID {
		return errResult(DriverMCP, time.Since(start), fmt.Sprintf("initialize id mismatch: got %d want %d", initResp.ID, initID))
	}
	// 2. notifications/initialized — http MCP servers per spec accept
	// notifications as POSTs without expecting a response (server may
	// 200 OK with empty body or 202).
	_ = httpRPC.notify("notifications/initialized", nil)

	// 3. tools/list discovery
	tool := d.provider.ToolName
	if tool == "" {
		tool = mcpDefaultToolName
	}
	const listID = 100
	listResp, err := httpRPC.call(listID, "tools/list", map[string]any{})
	if err != nil {
		return errResult(DriverMCP, time.Since(start), fmt.Sprintf("tools/list: %v", err))
	}
	available, _ := extractToolNames(listResp)
	if len(available) > 0 && !slices.Contains(available, tool) {
		return errResult(DriverMCP, time.Since(start), fmt.Sprintf("tool %q not exposed by mcp server %q (available: %v). Set tool_name: in agents.yaml.", tool, d.provider.Name, available))
	}

	// 4. tools/call
	const callID = 2
	resp, err := httpRPC.call(callID, "tools/call", map[string]any{
		"name": tool,
		"arguments": AnalyzeArgs{
			Preset:        "swarm",
			SeverityVocab: defaultSeverityVocab(prompt),
			Input:         prompt,
		},
	})
	if err != nil {
		return errResult(DriverMCP, time.Since(start), fmt.Sprintf("tools/call: %v", err))
	}
	if resp.ID != callID {
		return errResult(DriverMCP, time.Since(start), fmt.Sprintf("tools/call id mismatch: got %d want %d", resp.ID, callID))
	}
	raw, err := extractMCPText(resp)
	if err != nil {
		return errResult(DriverMCP, time.Since(start), err.Error())
	}
	return Result{
		Raw:         raw,
		CostUSD:     0,
		Duration:    time.Since(start),
		Driver:      DriverMCP,
		CacheStatus: CacheUnsupported,
	}, nil
}

// buildMCPHeaders resolves env-var indirection in provider.Headers
// and merges with HeadersLiteral. HeadersLiteral wins on collision.
func buildMCPHeaders(p *Provider) map[string]string {
	out := map[string]string{}
	for hdr, envName := range p.Headers {
		if v := os.Getenv(envName); v != "" {
			out[hdr] = v
		}
	}
	for k, v := range p.HeadersLiteral {
		out[k] = v
	}
	return out
}

// mcpHTTPClient sends JSON-RPC messages over HTTP POSTs. Each call
// is independent (no persistent connection state); MCP servers
// implementing Streamable HTTP MUST tolerate this.
type mcpHTTPClient struct {
	ctx      context.Context
	endpoint string
	headers  map[string]string
}

func (c *mcpHTTPClient) call(id int, method string, params any) (*mcpRPCResponse, error) {
	body, err := json.Marshal(mcpRPCRequest{JSONRPC: "2.0", ID: &id, Method: method, Params: params})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(c.ctx, "POST", c.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("accept", "application/json")
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, SanitizeForLog(trimErr(string(respBody))))
	}
	var rpcResp mcpRPCResponse
	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		return nil, fmt.Errorf("parse response: %v", err)
	}
	return &rpcResp, nil
}

// notify sends a JSON-RPC notification (no id, no response expected).
// HTTP servers MAY respond with 200/202 + empty body. We discard the
// response.
func (c *mcpHTTPClient) notify(method string, params any) error {
	body := map[string]any{"jsonrpc": "2.0", "method": method}
	if params != nil {
		body["params"] = params
	}
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(c.ctx, "POST", c.endpoint, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("content-type", "application/json")
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	return nil
}

// invokeStdio spawns the MCP server, performs the JSON-RPC handshake
// (initialize + notifications/initialized), calls the analyze tool,
// and parses the response. The server process is killed at function
// exit regardless of success/failure.
func (d *mcpDriver) invokeStdio(ctx context.Context, prompt string) (Result, error) {
	if d.provider.Command == "" {
		return errResult(DriverMCP, 0, fmt.Sprintf("provider %q: stdio transport requires command field", d.provider.Name))
	}

	timeout := mcpStdioTimeout
	if d.provider.TimeoutSec > 0 {
		timeout = time.Duration(d.provider.TimeoutSec) * time.Second
	}
	subCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(subCtx, d.provider.Command, d.provider.Args...)
	cmd.WaitDelay = 5 * time.Second
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return errResult(DriverMCP, 0, fmt.Sprintf("stdin pipe: %v", err))
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return errResult(DriverMCP, 0, fmt.Sprintf("stdout pipe: %v", err))
	}
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf
	start := time.Now()
	if err := cmd.Start(); err != nil {
		return errResult(DriverMCP, 0, fmt.Sprintf("spawn %s: %v", d.provider.Command, err))
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	// Cap stdout reads at 8 MB to prevent OOM if a misbehaving server
	// streams without ever emitting a newline. Same cap as the http
	// driver's response-body reader.
	const maxStdoutBytes = 8 * 1024 * 1024
	rdr := bufio.NewReader(io.LimitReader(stdout, maxStdoutBytes))
	rpc := &mcpStdioClient{stdin: stdin, stdout: rdr, ctx: subCtx}

	// 1. initialize handshake.
	const initID = 1
	if err := rpc.send(initID, "initialize", map[string]any{
		"protocolVersion": "2025-11-25",
		"clientInfo":      map[string]string{"name": "jutsu", "version": "0.6.0"},
		"capabilities":    map[string]any{},
	}); err != nil {
		return errResult(DriverMCP, time.Since(start), fmt.Sprintf("send initialize: %v (stderr: %s)", err, SanitizeForLog(trimErr(stderrBuf.String()))))
	}
	initResp, err := rpc.recv()
	if err != nil {
		return errResult(DriverMCP, time.Since(start), fmt.Sprintf("recv initialize: %v (stderr: %s)", err, SanitizeForLog(trimErr(stderrBuf.String()))))
	}
	if initResp.ID != initID {
		return errResult(DriverMCP, time.Since(start), fmt.Sprintf("initialize response id mismatch: got %d, want %d (server out-of-order or interleaved notification)", initResp.ID, initID))
	}
	// 2. initialized notification (no id; no response expected).
	if err := rpc.notify("notifications/initialized", nil); err != nil {
		return errResult(DriverMCP, time.Since(start), fmt.Sprintf("notify initialized: %v", err))
	}

	// 3. tools/list discovery — validates the configured ToolName
	// exists on this server. Real-world servers (semgrep-mcp,
	// eslint-mcp) may expose `scan` or `audit` instead of the
	// default `analyze`; without discovery the tools/call fails
	// opaquely with method-not-found and the user has no clue what
	// tool name to pass via `tool_name:`.
	tool := d.provider.ToolName
	if tool == "" {
		tool = mcpDefaultToolName
	}
	const listID = 100
	if err := rpc.send(listID, "tools/list", map[string]any{}); err != nil {
		return errResult(DriverMCP, time.Since(start), fmt.Sprintf("send tools/list: %v", err))
	}
	listResp, err := rpc.recv()
	if err != nil {
		return errResult(DriverMCP, time.Since(start), fmt.Sprintf("recv tools/list: %v (stderr: %s)", err, SanitizeForLog(trimErr(stderrBuf.String()))))
	}
	if listResp.ID != listID {
		return errResult(DriverMCP, time.Since(start), fmt.Sprintf("tools/list response id mismatch: got %d, want %d", listResp.ID, listID))
	}
	available, err := extractToolNames(listResp)
	if err == nil && len(available) > 0 && !slices.Contains(available, tool) {
		return errResult(DriverMCP, time.Since(start), fmt.Sprintf("tool %q not exposed by mcp server %q (available: %v). Set tool_name: in agents.yaml to one of those.", tool, d.provider.Name, available))
	}
	// 4. tools/call for analyze.
	const callID = 2
	if err := rpc.send(callID, "tools/call", map[string]any{
		"name": tool,
		"arguments": AnalyzeArgs{
			Preset:        "swarm",                      // preset name fed via prompt today; refined in v0.6.x
			SeverityVocab: defaultSeverityVocab(prompt), // best-effort; see helper
			Input:         prompt,
		},
	}); err != nil {
		return errResult(DriverMCP, time.Since(start), fmt.Sprintf("send tools/call: %v (stderr: %s)", err, SanitizeForLog(trimErr(stderrBuf.String()))))
	}
	resp, err := rpc.recv()
	if err != nil {
		return errResult(DriverMCP, time.Since(start), fmt.Sprintf("recv tools/call: %v (stderr: %s)", err, SanitizeForLog(trimErr(stderrBuf.String()))))
	}
	if resp.ID != callID {
		return errResult(DriverMCP, time.Since(start), fmt.Sprintf("tools/call response id mismatch: got %d, want %d", resp.ID, callID))
	}

	raw, err := extractMCPText(resp)
	if err != nil {
		return errResult(DriverMCP, time.Since(start), err.Error())
	}

	return Result{
		Raw:         raw,
		CostUSD:     0,
		Duration:    time.Since(start),
		Driver:      DriverMCP,
		CacheStatus: CacheUnsupported,
	}, nil
}

// extractToolNames pulls the tool names from a tools/list response.
// MCP returns `result: {tools: [{name, description, inputSchema}]}`.
// Returns an empty slice when the server returns no tools array
// (caller skips the validation in that case rather than blocking).
func extractToolNames(resp *mcpRPCResponse) ([]string, error) {
	if resp.Error != nil {
		return nil, fmt.Errorf("tools/list error: %s", resp.Error.Message)
	}
	var result struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(result.Tools))
	for _, t := range result.Tools {
		out = append(out, t.Name)
	}
	return out, nil
}

// extractMCPText reads the response's content[0].text field, which is
// where MCP servers carry the tool's structured output as a JSON
// string.
func extractMCPText(resp *mcpRPCResponse) (string, error) {
	if resp.Error != nil {
		return "", fmt.Errorf("mcp server returned error: %s", resp.Error.Message)
	}
	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return "", fmt.Errorf("parse tools/call result: %v", err)
	}
	for _, c := range result.Content {
		if c.Type == "text" {
			return c.Text, nil
		}
	}
	return "", errors.New("mcp tools/call response missing text content block")
}

// defaultSeverityVocab is a best-effort extraction of the active
// preset's vocab from the prompt body. Per spec D7 the vocab should
// be passed as an explicit arg; v0.6 carries it implicitly inside the
// prompt and lets the analyzer infer. Stage 6.x will plumb the preset
// metadata directly.
func defaultSeverityVocab(prompt string) []string {
	// Pattern matching on common preset-prompt phrasings. Stage 6.x
	// will plumb the preset metadata explicitly so this heuristic
	// becomes unnecessary.
	switch {
	case strings.Contains(prompt, "critical | high | medium | low") ||
		strings.Contains(prompt, "CVSS"):
		return []string{"critical", "high", "medium", "low", "informational"}
	case strings.Contains(prompt, "recommended | alternative") ||
		strings.Contains(prompt, "recommended → alternative") ||
		strings.Contains(prompt, "recommended -> alternative"):
		return []string{"recommended", "alternative", "risky", "speculative"}
	default:
		return []string{"blocker", "issue", "minor", "info"}
	}
}

// --- JSON-RPC line client ---------------------------------------------

type mcpStdioClient struct {
	stdin  io.WriteCloser
	stdout *bufio.Reader
	ctx    context.Context // ctx-aware writes; bounds blocking on full pipe buffers if subprocess hangs
}

type mcpRPCRequest struct {
	JSONRPC string `json:"jsonrpc"`
	// ID uses *int so the omitempty tag genuinely distinguishes
	// "absent" (notification) from "ID=0" (request). With a plain
	// int, ID=0 would silently serialize as a notification, leaving
	// the client deadlocked on a recv() that the server will never
	// answer.
	ID     *int   `json:"id,omitempty"`
	Method string `json:"method"`
	Params any    `json:"params,omitempty"`
}

type mcpRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type mcpRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *mcpRPCError    `json:"error,omitempty"`
}

func (c *mcpStdioClient) send(id int, method string, params any) error {
	req := mcpRPCRequest{JSONRPC: "2.0", ID: &id, Method: method, Params: params}
	data, err := json.Marshal(req)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return c.writeCtx(data)
}

func (c *mcpStdioClient) notify(method string, params any) error {
	// Notification: no id field per JSON-RPC 2.0.
	body := map[string]any{"jsonrpc": "2.0", "method": method}
	if params != nil {
		body["params"] = params
	}
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return c.writeCtx(data)
}

// writeCtx wraps stdin.Write in a select against ctx.Done. Without
// this, a subprocess that's alive but not reading stdin (init bug,
// infinite loop) saturates the pipe buffer and Write blocks until
// the parent ctx cancels and exec.CommandContext kills the child.
// That kill releases the pipe via EPIPE — but the wait can be up to
// the full ctx timeout. The select-aware goroutine returns
// promptly with ctx.Err() on cancellation.
func (c *mcpStdioClient) writeCtx(data []byte) error {
	if c.ctx == nil {
		_, err := c.stdin.Write(data)
		return err
	}
	done := make(chan error, 1)
	go func() {
		_, err := c.stdin.Write(data)
		done <- err
	}()
	select {
	case <-c.ctx.Done():
		return c.ctx.Err()
	case err := <-done:
		return err
	}
}

func (c *mcpStdioClient) recv() (*mcpRPCResponse, error) {
	line, err := c.stdout.ReadBytes('\n')
	if err != nil {
		return nil, err
	}
	var resp mcpRPCResponse
	if err := json.Unmarshal(line, &resp); err != nil {
		return nil, fmt.Errorf("parse response %q: %v", trimErr(string(line)), err)
	}
	return &resp, nil
}

