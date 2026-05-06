package agents

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// --- Anthropic-compat tests ------------------------------------------

func TestHTTPDriver_AnthropicHappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			http.Error(w, "wrong path "+r.URL.Path, http.StatusNotFound)
			return
		}
		if r.Header.Get("x-api-key") != "test-key-XYZ" {
			http.Error(w, "missing key", http.StatusUnauthorized)
			return
		}
		// Echo a canned response.
		_, _ = w.Write([]byte(`{
			"content": [{"type":"text","text":"hello"}],
			"usage": {"input_tokens": 100, "output_tokens": 20, "cache_creation_input_tokens": 0, "cache_read_input_tokens": 0}
		}`))
	}))
	defer srv.Close()
	t.Setenv("FAKE_ANTHROPIC_KEY", "test-key-XYZ")

	d := newTestHTTPDriver(srv.URL, protocolAnthropic, "claude-test-1", "FAKE_ANTHROPIC_KEY")
	res, err := d.Invoke(context.Background(), "user prompt", InvokeOpts{})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if res.Raw != "hello" {
		t.Errorf("Raw = %q, want %q", res.Raw, "hello")
	}
	if res.CacheStatus != CacheMiss {
		t.Errorf("CacheStatus = %q, want %q (no cache_read_input_tokens)", res.CacheStatus, CacheMiss)
	}
	if res.Driver != DriverHTTP {
		t.Errorf("Driver = %q, want %q", res.Driver, DriverHTTP)
	}
	if res.Duration <= 0 {
		t.Errorf("Duration = %v, want > 0", res.Duration)
	}
}

// TestHTTPDriver_AnthropicCacheHit_Cost50PctReduction is the spec AC
// gate: when cache_read_input_tokens dominates, computed cost drops
// to ≥50% below the uncached cost.
func TestHTTPDriver_AnthropicCacheHit_Cost50PctReduction(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			// First call: writes to cache.
			_, _ = w.Write([]byte(`{
				"content": [{"type":"text","text":"first"}],
				"usage": {"input_tokens": 0, "output_tokens": 100, "cache_creation_input_tokens": 1000, "cache_read_input_tokens": 0}
			}`))
		} else {
			// Second call: reads from cache.
			_, _ = w.Write([]byte(`{
				"content": [{"type":"text","text":"second"}],
				"usage": {"input_tokens": 0, "output_tokens": 100, "cache_creation_input_tokens": 0, "cache_read_input_tokens": 1000}
			}`))
		}
	}))
	defer srv.Close()
	t.Setenv("FAKE_ANTHROPIC_KEY", "k")

	d := newTestHTTPDriver(srv.URL, protocolAnthropic, "claude-test-1", "FAKE_ANTHROPIC_KEY")
	d.provider.Cost = &CostRates{
		InputPerMtok:       3.00, // claude 3.5 sonnet
		OutputPerMtok:      15.00,
		CachedInputPerMtok: 0.30, // 10% of input
	}
	r1, err := d.Invoke(context.Background(), "system\n\n<<<USER>>>\n\nbody", InvokeOpts{})
	if err != nil {
		t.Fatal(err)
	}
	r2, err := d.Invoke(context.Background(), "system\n\n<<<USER>>>\n\nbody", InvokeOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if r1.CacheStatus != CacheMiss {
		t.Errorf("call 1 CacheStatus = %q, want miss", r1.CacheStatus)
	}
	if r2.CacheStatus != CacheHit {
		t.Errorf("call 2 CacheStatus = %q, want hit", r2.CacheStatus)
	}
	if r2.CostUSD >= 0.5*r1.CostUSD {
		t.Errorf("cache hit cost reduction insufficient: r1=%.6f r2=%.6f (want r2 < 0.5*r1)", r1.CostUSD, r2.CostUSD)
	}
}

func TestHTTPDriver_AnthropicSendsCacheControlOnSystem(t *testing.T) {
	var captured anthropicRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &captured)
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"ok"}],"usage":{}}`))
	}))
	defer srv.Close()
	t.Setenv("K", "v")

	d := newTestHTTPDriver(srv.URL, protocolAnthropic, "m", "K")
	_, err := d.Invoke(context.Background(), "i am the system\n\n<<<USER>>>\n\nuser body", InvokeOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if len(captured.System) != 1 {
		t.Fatalf("expected 1 system block, got %d", len(captured.System))
	}
	if captured.System[0].Text != "i am the system" {
		t.Errorf("system text = %q", captured.System[0].Text)
	}
	if captured.System[0].CacheControl == nil || captured.System[0].CacheControl.Type != "ephemeral" {
		t.Errorf("system block missing cache_control: %+v", captured.System[0].CacheControl)
	}
	if len(captured.Messages) != 1 || captured.Messages[0].Content != "user body" {
		t.Errorf("messages = %+v", captured.Messages)
	}
}

func TestHTTPDriver_AnthropicHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"rate limited"}`, http.StatusTooManyRequests)
	}))
	defer srv.Close()
	t.Setenv("K", "v")

	d := newTestHTTPDriver(srv.URL, protocolAnthropic, "m", "K")
	res, err := d.Invoke(context.Background(), "x", InvokeOpts{})
	if err == nil {
		t.Fatal("expected error on 429; got nil")
	}
	if !strings.Contains(err.Error(), "anthropic 429") {
		t.Errorf("error = %v, want substring 'anthropic 429'", err)
	}
	if res.Err != err.Error() {
		t.Errorf("Result.Err != error.Error()")
	}
}

// --- OpenAI-compat tests ---------------------------------------------

func TestHTTPDriver_OpenAIHappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.Error(w, "wrong path "+r.URL.Path, http.StatusNotFound)
			return
		}
		if !strings.HasPrefix(r.Header.Get("authorization"), "Bearer ") {
			http.Error(w, "missing bearer", http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{
			"choices": [{"message": {"role":"assistant","content":"hello-oa"}}],
			"usage": {"prompt_tokens": 100, "completion_tokens": 20, "total_tokens": 120}
		}`))
	}))
	defer srv.Close()
	t.Setenv("FAKE_OPENAI_KEY", "sk-x")

	d := newTestHTTPDriver(srv.URL, protocolOpenAI, "deepseek-coder", "FAKE_OPENAI_KEY")
	res, err := d.Invoke(context.Background(), "user prompt", InvokeOpts{})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if res.Raw != "hello-oa" {
		t.Errorf("Raw = %q", res.Raw)
	}
	if res.CacheStatus != CacheMiss {
		t.Errorf("CacheStatus = %q, want miss", res.CacheStatus)
	}
}

func TestHTTPDriver_OpenAICacheHit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
			"choices": [{"message": {"content":"x"}}],
			"usage": {"prompt_tokens": 1500, "completion_tokens": 10, "total_tokens": 1510, "prompt_tokens_details": {"cached_tokens": 1500}}
		}`))
	}))
	defer srv.Close()
	t.Setenv("K", "k")
	d := newTestHTTPDriver(srv.URL, protocolOpenAI, "m", "K")
	res, err := d.Invoke(context.Background(), "x", InvokeOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if res.CacheStatus != CacheHit {
		t.Errorf("CacheStatus = %q, want hit", res.CacheStatus)
	}
}

func TestHTTPDriver_OpenAIPartialCache(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
			"choices": [{"message": {"content":"x"}}],
			"usage": {"prompt_tokens": 1500, "completion_tokens": 10, "prompt_tokens_details": {"cached_tokens": 1000}}
		}`))
	}))
	defer srv.Close()
	t.Setenv("K", "k")
	d := newTestHTTPDriver(srv.URL, protocolOpenAI, "m", "K")
	res, err := d.Invoke(context.Background(), "x", InvokeOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if res.CacheStatus != CachePartial {
		t.Errorf("CacheStatus = %q, want partial", res.CacheStatus)
	}
}

// --- Auth + lifecycle tests ------------------------------------------

func TestHTTPDriver_MissingAPIKeySurfacesError(t *testing.T) {
	d := newTestHTTPDriver("http://example.invalid", protocolAnthropic, "m", "DEFINITELY_UNSET_ENV_VAR_FOR_TEST")
	res, err := d.Invoke(context.Background(), "x", InvokeOpts{})
	if err == nil {
		t.Fatal("expected error on missing API key; got nil")
	}
	if res.Err == "" {
		t.Error("Result.Err empty on missing-key path")
	}
	if !strings.Contains(err.Error(), "DEFINITELY_UNSET_ENV_VAR_FOR_TEST") {
		t.Errorf("error doesn't mention the env var: %v", err)
	}
}

func TestHTTPDriver_ContextCancellationPropagates(t *testing.T) {
	// Server hangs forever; canceled ctx must abort the request.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
			return
		case <-time.After(2 * time.Second):
			// Bound the server-side hang so test teardown is fast;
			// 100ms client timeout still triggers well before this.
			return
		}
	}))
	defer srv.Close()
	t.Setenv("K", "v")

	d := newTestHTTPDriver(srv.URL, protocolAnthropic, "m", "K")
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := d.Invoke(ctx, "x", InvokeOpts{})
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected context-cancellation error; got nil")
	}
	if elapsed > 5*time.Second {
		t.Errorf("ctx cancellation didn't propagate: elapsed %v", elapsed)
	}
}

func TestHTTPDriver_OptsTimeoutPropagates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
			return
		case <-time.After(2 * time.Second):
			// Bound the server-side hang so test teardown is fast;
			// 100ms client timeout still triggers well before this.
			return
		}
	}))
	defer srv.Close()
	t.Setenv("K", "v")

	d := newTestHTTPDriver(srv.URL, protocolAnthropic, "m", "K")
	start := time.Now()
	_, err := d.Invoke(context.Background(), "x", InvokeOpts{Timeout: 100 * time.Millisecond})
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected timeout error; got nil")
	}
	if elapsed > 5*time.Second {
		t.Errorf("opts.Timeout ignored: elapsed %v", elapsed)
	}
}

// --- BuildDriver tests -----------------------------------------------

func TestBuildDriver_HTTPRequiresProtocolBaseURLModel(t *testing.T) {
	cases := []struct {
		name string
		p    *Provider
		want string
	}{
		{"missing all", &Provider{Name: "x", Driver: DriverHTTP}, "base_url is required"},
		{"missing model", &Provider{Name: "x", Driver: DriverHTTP, BaseURL: "https://x"}, "model is required"},
		{"missing protocol", &Provider{Name: "x", Driver: DriverHTTP, BaseURL: "https://x", Model: "m"}, "protocol is required"},
		{"unknown protocol", &Provider{Name: "x", Driver: DriverHTTP, BaseURL: "https://x", Model: "m", Protocol: "weird"}, "unsupported protocol"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := BuildDriver(tc.p)
			if err == nil {
				t.Fatal("expected error; got nil")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v; want substring %q", err, tc.want)
			}
		})
	}
}

func TestBuildDriver_HTTPSucceedsWithFullConfig(t *testing.T) {
	p := &Provider{
		Name:     "deepseek",
		Driver:   DriverHTTP,
		BaseURL:  "https://api.deepseek.com/v1",
		Model:    "deepseek-coder",
		Protocol: protocolOpenAI,
	}
	d, err := BuildDriver(p)
	if err != nil {
		t.Fatal(err)
	}
	if d.Driver() != DriverHTTP {
		t.Errorf("Driver() = %q, want %q", d.Driver(), DriverHTTP)
	}
	if d.Name() != "deepseek" {
		t.Errorf("Name() = %q", d.Name())
	}
}

func TestBuildDriver_RejectsUnimplementedKinds(t *testing.T) {
	cases := []struct {
		kind DriverKind
		want string
	}{
		{DriverCLICompat, "cli-compat"},
		{DriverMCP, "mcp"},
	}
	for _, tc := range cases {
		t.Run(string(tc.kind), func(t *testing.T) {
			_, err := BuildDriver(&Provider{Name: "x", Driver: tc.kind})
			if err == nil {
				t.Fatal("expected error; got nil")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v; want substring %q", err, tc.want)
			}
		})
	}
}

func TestBuildDriver_CLISuccessPath(t *testing.T) {
	cases := []struct {
		name string
		want DriverKind
	}{
		{"claude", DriverCLI},
		{"codex", DriverCLI},
		{"gemini", DriverCLI},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d, err := BuildDriver(&Provider{Name: tc.name, Driver: DriverCLI})
			if err != nil {
				t.Fatal(err)
			}
			if d.Driver() != tc.want {
				t.Errorf("Driver() = %q, want %q", d.Driver(), tc.want)
			}
			if d.Name() != tc.name {
				t.Errorf("Name() = %q, want %q", d.Name(), tc.name)
			}
		})
	}
}

func TestBuildDriver_CLIRejectsUnknownName(t *testing.T) {
	_, err := BuildDriver(&Provider{Name: "opencode", Driver: DriverCLI})
	if err == nil {
		t.Fatal("expected error for unknown cli provider name; got nil")
	}
	if !strings.Contains(err.Error(), "claude|codex|gemini") {
		t.Errorf("error doesn't hint allowed cli names: %v", err)
	}
}

func TestBuildDriver_NilProvider(t *testing.T) {
	_, err := BuildDriver(nil)
	if err == nil {
		t.Fatal("expected error on nil provider")
	}
	if !strings.Contains(err.Error(), "nil provider") {
		t.Errorf("error = %v; want substring 'nil provider'", err)
	}
}

// --- Test helper -----------------------------------------------------

// newTestHTTPDriver constructs a Provider + httpDriver pointing at a
// test server URL. Used throughout the table tests.
func newTestHTTPDriver(baseURL, protocol, model, apiKeyEnv string) *httpDriver {
	return &httpDriver{
		provider: &Provider{
			Name:      "test-provider",
			Driver:    DriverHTTP,
			Protocol:  protocol,
			BaseURL:   baseURL,
			Model:     model,
			APIKeyEnv: apiKeyEnv,
		},
	}
}
