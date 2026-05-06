package agents

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
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
		return errResult(DriverMCP, 0, fmt.Sprintf("provider %q (mcp): http transport not implemented in v0.6 (stdio only)", d.provider.Name))
	}
	return errResult(DriverMCP, 0, fmt.Sprintf("provider %q (mcp): unsupported transport %q (allowed: stdio)", d.provider.Name, d.provider.Transport))
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

	// 3. tools/call for analyze.
	tool := d.provider.ToolName
	if tool == "" {
		tool = mcpDefaultToolName
	}
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

