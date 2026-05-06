package agents

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"os"
	"strconv"
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

// Retry policy: on 429 (rate limit) and 5xx (server error) only.
// Auth errors (4xx other than 429) and parse errors don't retry —
// they're not transient. Honors Retry-After header when present;
// otherwise exponential backoff with full jitter.
const (
	httpMaxRetries     = 2 // total attempts = retries + 1 = 3
	httpBaseBackoff    = 500 * time.Millisecond
	httpMaxBackoff     = 8 * time.Second
)

// retryableStatus reports whether an HTTP status warrants a retry.
func retryableStatus(code int) bool {
	return code == 429 || (code >= 500 && code < 600)
}

// nextBackoff returns the wait before the next attempt. Uses
// Retry-After (RFC 7231 delta-seconds form only — HTTP-date deferred)
// when the server provides it; falls back to exponential backoff with
// full jitter.
func nextBackoff(attempt int, retryAfter string) time.Duration {
	if retryAfter != "" {
		if secs, err := strconv.Atoi(strings.TrimSpace(retryAfter)); err == nil && secs > 0 {
			d := time.Duration(secs) * time.Second
			if d > httpMaxBackoff {
				return httpMaxBackoff
			}
			return d
		}
	}
	// Full-jitter exponential: rand[0, base * 2^attempt], capped.
	cap := time.Duration(1<<attempt) * httpBaseBackoff
	if cap > httpMaxBackoff {
		cap = httpMaxBackoff
	}
	// math/rand/v2 — goroutine-safe by design (Go 1.22+). N(int64) is
	// equivalent to v1's Int63n; eliminates the data-race risk under
	// concurrent retry loops.
	return time.Duration(rand.Int64N(int64(cap) + 1))
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
	respBody, status, err := d.doWithRetry(ctx, httpReq, body)
	if err != nil {
		return errResult(DriverHTTP, time.Since(start), fmt.Sprintf("anthropic request: %v", err))
	}
	if status < 200 || status >= 300 {
		return errResult(DriverHTTP, time.Since(start), fmt.Sprintf("anthropic %d: %s", status, SanitizeForLog(trimErr(string(respBody)))))
	}
	var ar anthropicResponse
	if err := json.Unmarshal(respBody, &ar); err != nil {
		return errResult(DriverHTTP, time.Since(start), fmt.Sprintf("parse anthropic response: %v", err))
	}

	raw := joinAnthropicContent(ar.Content)
	cost := computeAnthropicCost(d.provider.Cost, ar.Usage)
	cs := classifyAnthropicCache(ar.Usage)
	// When no system block was sent (sentinel-less prompt) Anthropic
	// has nothing to cache. Report that explicitly so users don't
	// chase ghost CacheMiss readings in the cost report.
	if system == "" {
		cs = CacheSkippedShort
	}

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
	Model    string          `json:"model"`
	Messages []openaiMessage `json:"messages"`
	// MaxCompletionTokens is the post-2024 OpenAI field; replaces the
	// deprecated max_tokens which is rejected by o-series reasoning
	// models (o1, o3, o4-mini). All current OpenAI-compat providers
	// (deepseek, glm, kimi) accept it as of 2026-05.
	MaxCompletionTokens int `json:"max_completion_tokens,omitempty"`
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
		Model:               d.provider.Model,
		Messages:            msgs,
		MaxCompletionTokens: defaultMaxTokens,
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
	respBody, status, err := d.doWithRetry(ctx, httpReq, body)
	if err != nil {
		return errResult(DriverHTTP, time.Since(start), fmt.Sprintf("openai request: %v", err))
	}
	if status < 200 || status >= 300 {
		return errResult(DriverHTTP, time.Since(start), fmt.Sprintf("openai %d: %s", status, SanitizeForLog(trimErr(string(respBody)))))
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

// doWithRetry sends the request and reads the body. On 429/5xx it
// retries up to httpMaxRetries times with exponential backoff +
// full jitter (or Retry-After when the server provides it). Returns
// the final body bytes + status code; (nil, 0, error) on transport
// failures or context cancellation.
//
// The original request body is reused across retries by re-cloning
// from the captured `body` byte slice — http.Request.Body is a
// io.ReadCloser that's consumed on first send.
func (d *httpDriver) doWithRetry(ctx context.Context, req *http.Request, body []byte) ([]byte, int, error) {
	var lastBody []byte
	var lastStatus int
	var lastRetryAfter string
	for attempt := 0; attempt <= httpMaxRetries; attempt++ {
		if attempt > 0 {
			delay := nextBackoff(attempt-1, lastRetryAfter)
			select {
			case <-ctx.Done():
				return nil, 0, ctx.Err()
			case <-time.After(delay):
			}
			// Recreate the body reader (consumed on prior attempt).
			req.Body = io.NopCloser(bytes.NewReader(body))
			req.GetBody = func() (io.ReadCloser, error) {
				return io.NopCloser(bytes.NewReader(body)), nil
			}
		}
		resp, err := d.httpClient().Do(req)
		if err != nil {
			// Transport-level error — not retryable (DNS / connection
			// refused / cert problem / context cancellation).
			return nil, 0, err
		}
		readBody, readErr := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024))
		// Capture Retry-After before closing — used by the next
		// attempt's nextBackoff(). RFC 7231 says servers MAY send
		// this on 429/503; honoring it avoids hammering when the
		// server has explicitly asked us to wait.
		lastRetryAfter = resp.Header.Get("Retry-After")
		_ = resp.Body.Close()
		if readErr != nil {
			return nil, resp.StatusCode, fmt.Errorf("read body: %v", readErr)
		}
		lastBody = readBody
		lastStatus = resp.StatusCode
		if !retryableStatus(resp.StatusCode) {
			return readBody, resp.StatusCode, nil
		}
		// Will retry (if attempts remain).
	}
	return lastBody, lastStatus, nil
}

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
