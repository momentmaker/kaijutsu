package agents

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// httpDriver implements AgentDriver against an OpenAI-compat or
// Anthropic-compat HTTP endpoint. Constructed via BuildDriver from a
// resolved Provider config — all per-provider state (BaseURL, Model,
// APIKeyEnv, Protocol, CostRates) lives on the Provider; the driver
// is otherwise stateless and goroutine-safe.
type httpDriver struct {
	provider *Provider
	client   *http.Client
}

// Anthropic-compat protocol values.
const (
	protocolAnthropic = "anthropic-compat"
	protocolOpenAI    = "openai-compat"

	defaultMaxTokens = 4096
)

func (d *httpDriver) Name() string       { return d.provider.Name }
func (d *httpDriver) Driver() DriverKind { return DriverHTTP }

// Invoke sends prompt to the configured endpoint and returns Result.
// Honors ctx cancellation + opts.Timeout (smaller-of wins). API key
// is read from os.Getenv(provider.APIKeyEnv) at call time so users
// can rotate keys without restarting the process.
func (d *httpDriver) Invoke(ctx context.Context, prompt string, opts InvokeOpts) (Result, error) {
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	apiKey := os.Getenv(d.provider.APIKeyEnv)
	if d.provider.APIKeyEnv != "" && apiKey == "" {
		return errResult(DriverHTTP, 0, fmt.Sprintf("env %s not set", d.provider.APIKeyEnv))
	}

	switch d.provider.Protocol {
	case protocolAnthropic:
		return d.invokeAnthropic(ctx, prompt, apiKey)
	case protocolOpenAI:
		return d.invokeOpenAI(ctx, prompt, apiKey)
	}
	return errResult(DriverHTTP, 0, fmt.Sprintf("provider %q has unsupported protocol %q (allowed: %s, %s)", d.provider.Name, d.provider.Protocol, protocolOpenAI, protocolAnthropic))
}

// --- Anthropic-compat -------------------------------------------------

type anthropicSystemBlock struct {
	Type         string                 `json:"type"`
	Text         string                 `json:"text"`
	CacheControl *anthropicCacheControl `json:"cache_control,omitempty"`
}

type anthropicCacheControl struct {
	Type string `json:"type"` // "ephemeral"
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicRequest struct {
	Model     string                 `json:"model"`
	System    []anthropicSystemBlock `json:"system,omitempty"`
	Messages  []anthropicMessage     `json:"messages"`
	MaxTokens int                    `json:"max_tokens"`
}

type anthropicContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type anthropicUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
}

type anthropicResponse struct {
	Content []anthropicContent `json:"content"`
	Usage   anthropicUsage     `json:"usage"`
}

func (d *httpDriver) invokeAnthropic(ctx context.Context, prompt string, apiKey string) (Result, error) {
	system, user := splitSystemUser(prompt)
	req := anthropicRequest{
		Model:     d.provider.Model,
		Messages:  []anthropicMessage{{Role: "user", Content: user}},
		MaxTokens: defaultMaxTokens,
	}
	if system != "" {
		// Mark the system block as ephemeral-cacheable. Anthropic
		// caches the prefix after the first call (5-minute TTL); the
		// second call within that window reads from cache at ~10% of
		// normal input rate.
		req.System = []anthropicSystemBlock{{
			Type:         "text",
			Text:         system,
			CacheControl: &anthropicCacheControl{Type: "ephemeral"},
		}}
	}
	body, err := json.Marshal(req)
	if err != nil {
		return errResult(DriverHTTP, 0, fmt.Sprintf("marshal request: %v", err))
	}
	url := joinURL(d.provider.BaseURL, "/v1/messages")
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return errResult(DriverHTTP, 0, fmt.Sprintf("build request: %v", err))
	}
	httpReq.Header.Set("content-type", "application/json")
	if apiKey != "" {
		httpReq.Header.Set("x-api-key", apiKey)
	}
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	start := time.Now()
	resp, err := d.httpClient().Do(httpReq)
	if err != nil {
		return errResult(DriverHTTP, time.Since(start), fmt.Sprintf("anthropic request: %v", err))
	}
	defer resp.Body.Close()

	respBody, readErr := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024))
	if readErr != nil {
		return errResult(DriverHTTP, time.Since(start), fmt.Sprintf("read anthropic response: %v", readErr))
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return errResult(DriverHTTP, time.Since(start), fmt.Sprintf("anthropic %d: %s", resp.StatusCode, trimErr(string(respBody))))
	}
	var ar anthropicResponse
	if err := json.Unmarshal(respBody, &ar); err != nil {
		return errResult(DriverHTTP, time.Since(start), fmt.Sprintf("parse anthropic response: %v", err))
	}

	raw := joinAnthropicContent(ar.Content)
	cost := computeAnthropicCost(d.provider.Cost, ar.Usage)
	cs := classifyAnthropicCache(ar.Usage)

	return Result{
		Raw:         raw,
		CostUSD:     cost,
		Duration:    time.Since(start),
		Driver:      DriverHTTP,
		CacheStatus: cs,
	}, nil
}

func joinAnthropicContent(blocks []anthropicContent) string {
	var b strings.Builder
	for _, blk := range blocks {
		if blk.Type == "text" {
			b.WriteString(blk.Text)
		}
	}
	return b.String()
}

func computeAnthropicCost(rates *CostRates, u anthropicUsage) float64 {
	if rates == nil {
		return 0
	}
	const perMtok = 1_000_000.0
	cost := float64(u.OutputTokens) * rates.OutputPerMtok / perMtok
	cost += float64(u.InputTokens) * rates.InputPerMtok / perMtok
	cachedRate := rates.CachedInputPerMtok
	if cachedRate == 0 {
		cachedRate = rates.InputPerMtok * 0.10 // Anthropic published default
	}
	cost += float64(u.CacheReadInputTokens) * cachedRate / perMtok
	// cache_creation_input_tokens charged at 125% of input rate per
	// Anthropic — but we conservatively treat as input rate to avoid
	// over-charging when rates table doesn't carry the +25% premium.
	cost += float64(u.CacheCreationInputTokens) * rates.InputPerMtok / perMtok
	return cost
}

func classifyAnthropicCache(u anthropicUsage) CacheStatus {
	switch {
	case u.CacheReadInputTokens > 0 && u.InputTokens > 0:
		return CachePartial
	case u.CacheReadInputTokens > 0:
		return CacheHit
	case u.CacheCreationInputTokens > 0:
		return CacheMiss
	default:
		return CacheMiss
	}
}

// --- OpenAI-compat ----------------------------------------------------

type openaiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openaiRequest struct {
	Model     string          `json:"model"`
	Messages  []openaiMessage `json:"messages"`
	MaxTokens int             `json:"max_tokens,omitempty"`
}

type openaiPromptDetails struct {
	CachedTokens int `json:"cached_tokens"`
}

type openaiUsage struct {
	PromptTokens        int                  `json:"prompt_tokens"`
	CompletionTokens    int                  `json:"completion_tokens"`
	TotalTokens         int                  `json:"total_tokens"`
	PromptTokensDetails *openaiPromptDetails `json:"prompt_tokens_details,omitempty"`
}

type openaiChoice struct {
	Message openaiMessage `json:"message"`
}

type openaiResponse struct {
	Choices []openaiChoice `json:"choices"`
	Usage   openaiUsage    `json:"usage"`
}

func (d *httpDriver) invokeOpenAI(ctx context.Context, prompt string, apiKey string) (Result, error) {
	system, user := splitSystemUser(prompt)
	msgs := []openaiMessage{}
	if system != "" {
		msgs = append(msgs, openaiMessage{Role: "system", Content: system})
	}
	msgs = append(msgs, openaiMessage{Role: "user", Content: user})

	req := openaiRequest{
		Model:     d.provider.Model,
		Messages:  msgs,
		MaxTokens: defaultMaxTokens,
	}
	body, err := json.Marshal(req)
	if err != nil {
		return errResult(DriverHTTP, 0, fmt.Sprintf("marshal request: %v", err))
	}
	url := joinURL(d.provider.BaseURL, "/chat/completions")
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return errResult(DriverHTTP, 0, fmt.Sprintf("build request: %v", err))
	}
	httpReq.Header.Set("content-type", "application/json")
	if apiKey != "" {
		httpReq.Header.Set("authorization", "Bearer "+apiKey)
	}

	start := time.Now()
	resp, err := d.httpClient().Do(httpReq)
	if err != nil {
		return errResult(DriverHTTP, time.Since(start), fmt.Sprintf("openai request: %v", err))
	}
	defer resp.Body.Close()

	respBody, readErr := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024))
	if readErr != nil {
		return errResult(DriverHTTP, time.Since(start), fmt.Sprintf("read openai response: %v", readErr))
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return errResult(DriverHTTP, time.Since(start), fmt.Sprintf("openai %d: %s", resp.StatusCode, trimErr(string(respBody))))
	}
	var or openaiResponse
	if err := json.Unmarshal(respBody, &or); err != nil {
		return errResult(DriverHTTP, time.Since(start), fmt.Sprintf("parse openai response: %v", err))
	}
	if len(or.Choices) == 0 {
		return errResult(DriverHTTP, time.Since(start), "openai response missing choices")
	}

	cost := computeOpenAICost(d.provider.Cost, or.Usage)
	cs := classifyOpenAICache(or.Usage)

	return Result{
		Raw:         or.Choices[0].Message.Content,
		CostUSD:     cost,
		Duration:    time.Since(start),
		Driver:      DriverHTTP,
		CacheStatus: cs,
	}, nil
}

func computeOpenAICost(rates *CostRates, u openaiUsage) float64 {
	if rates == nil {
		return 0
	}
	const perMtok = 1_000_000.0
	cached := 0
	if u.PromptTokensDetails != nil {
		cached = u.PromptTokensDetails.CachedTokens
	}
	uncached := u.PromptTokens - cached
	if uncached < 0 {
		uncached = 0
	}
	cachedRate := rates.CachedInputPerMtok
	if cachedRate == 0 {
		cachedRate = rates.InputPerMtok * 0.50 // OpenAI published default
	}
	cost := float64(uncached) * rates.InputPerMtok / perMtok
	cost += float64(cached) * cachedRate / perMtok
	cost += float64(u.CompletionTokens) * rates.OutputPerMtok / perMtok
	return cost
}

func classifyOpenAICache(u openaiUsage) CacheStatus {
	cached := 0
	if u.PromptTokensDetails != nil {
		cached = u.PromptTokensDetails.CachedTokens
	}
	switch {
	case cached > 0 && cached < u.PromptTokens:
		return CachePartial
	case cached > 0:
		return CacheHit
	default:
		return CacheMiss
	}
}

// --- Helpers ----------------------------------------------------------

func (d *httpDriver) httpClient() *http.Client {
	if d.client != nil {
		return d.client
	}
	return http.DefaultClient
}

// splitSystemUser splits a prompt around an optional sentinel. The
// swarm pipeline assembles prompts in the convention:
//   <system_prompt>\n\n<<<USER>>>\n\n<user_prompt>
// Drivers honor the sentinel and route the system half into the
// protocol's first-class system field.
//
// Backward-compat: when no sentinel is present, the entire prompt
// becomes the user message and system is empty (matches v0.5 cli
// driver behavior where system + user are concatenated).
const userSentinel = "\n\n<<<USER>>>\n\n"

func splitSystemUser(p string) (system, user string) {
	idx := strings.Index(p, userSentinel)
	if idx < 0 {
		return "", p
	}
	return p[:idx], p[idx+len(userSentinel):]
}

// joinURL appends path to base, handling a trailing slash on either
// side without doubling.
func joinURL(base, path string) string {
	base = strings.TrimRight(base, "/")
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return base + path
}

// errResult builds a Result with Err populated for the failure path,
// along with the matching error so callers can branch on either source.
// `dur` is the wall-clock the in-flight call took before failing
// (zero when the failure happened before any I/O).
func errResult(kind DriverKind, dur time.Duration, msg string) (Result, error) {
	return Result{
			Driver:      kind,
			CacheStatus: CacheUnsupported,
			Duration:    dur,
			Err:         msg,
		},
		errors.New(msg)
}
