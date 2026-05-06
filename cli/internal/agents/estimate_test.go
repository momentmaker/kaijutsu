package agents

import (
	"strings"
	"testing"
)

func TestCountTokensCharFallback(t *testing.T) {
	cases := []struct {
		s    string
		want int
	}{
		{"", 0},
		{"a", 1},                       // 1 char → 1 token
		{"abcd", 1},                    // 4 chars → 1 token
		{"abcde", 2},                   // 5 chars → 2 tokens
		{strings.Repeat("x", 1000), 250}, // 1000 chars → 250 tokens
	}
	for _, tc := range cases {
		if got := CountTokensCharFallback(tc.s); got != tc.want {
			t.Errorf("CountTokensCharFallback(%q) = %d, want %d", tc.s, got, tc.want)
		}
	}
}

func TestProjectCost_NoRateCardReturnsZero(t *testing.T) {
	p := &Provider{Name: "x", Driver: DriverHTTP}
	cp := ProjectCost("default-x", p, "hello world")
	if cp.HasRateCard {
		t.Error("HasRateCard should be false when provider.Cost is nil")
	}
	if cp.CostUSD != 0 {
		t.Errorf("CostUSD = %v, want 0 when no rate card", cp.CostUSD)
	}
	if cp.InputTokens == 0 {
		t.Error("InputTokens should still be computed even without rate card")
	}
}

func TestProjectCost_AppliesRates(t *testing.T) {
	p := &Provider{
		Name:   "deepseek",
		Driver: DriverHTTP,
		Cost: &CostRates{
			InputPerMtok:  0.27,
			OutputPerMtok: 1.10,
		},
	}
	// 4-char prompt = 1 token in. Output estimate fixed at OutputTokensEstimate.
	cp := ProjectCost("default-deepseek", p, "abcd")
	wantIn := 1.0 * 0.27 / 1_000_000.0
	wantOut := float64(OutputTokensEstimate) * 1.10 / 1_000_000.0
	want := wantIn + wantOut
	if !floatNear(cp.CostUSD, want, 0.000001) {
		t.Errorf("CostUSD = %v, want %v", cp.CostUSD, want)
	}
}

func TestProjectCost_StaleRateCardFlagged(t *testing.T) {
	p := &Provider{
		Name:   "old",
		Driver: DriverHTTP,
		Cost: &CostRates{
			InputPerMtok:  1.0,
			OutputPerMtok: 1.0,
			RateCardDate:  "2020-01-01", // certainly more than 90 days ago
		},
	}
	cp := ProjectCost("p", p, "x")
	if !cp.Stale {
		t.Error("Stale should be true for 2020 rate card")
	}
	if cp.StaleDays < 90 {
		t.Errorf("StaleDays = %d, want >= 90", cp.StaleDays)
	}
}

func TestProjectCost_FreshRateCardNotFlagged(t *testing.T) {
	// Use a clearly recent date by parsing "now-30d". time.Now in tests
	// is fine; we can hardcode to 2026-04-15 which the test environment
	// (currentDate=2026-05-05) treats as ~20 days old.
	p := &Provider{
		Name:   "fresh",
		Driver: DriverHTTP,
		Cost:   &CostRates{InputPerMtok: 1.0, OutputPerMtok: 1.0, RateCardDate: "2026-04-15"},
	}
	cp := ProjectCost("p", p, "x")
	if cp.Stale {
		t.Error("Stale should be false for recent rate card")
	}
}

func TestRateCardAgeDays_UnparseableReturnsFalse(t *testing.T) {
	for _, in := range []string{"", "  ", "tomorrow", "2026-13-01"} {
		if _, ok := rateCardAgeDays(in); ok {
			t.Errorf("rateCardAgeDays(%q) = ok=true, want false", in)
		}
	}
}

func TestFormatProjection_HasRateCardCostShown(t *testing.T) {
	cp := CostProjection{
		PersonaName:    "p",
		ProviderName:   "p",
		DriverKind:     DriverHTTP,
		InputTokens:    100,
		OutputEstimate: 800,
		CostUSD:        0.0123,
		HasRateCard:    true,
	}
	got := FormatProjection(cp)
	if !strings.Contains(got, "$0.0123") {
		t.Errorf("FormatProjection missing cost: %q", got)
	}
}

func TestFormatProjection_NoRateCardLabel(t *testing.T) {
	cp := CostProjection{PersonaName: "p", DriverKind: DriverCLI, InputTokens: 100, OutputEstimate: 800}
	got := FormatProjection(cp)
	if !strings.Contains(got, "(no rate card)") {
		t.Errorf("FormatProjection missing no-rate-card label: %q", got)
	}
}

func TestCountTokensForProvider_OpenAICompatUsesTiktoken(t *testing.T) {
	p := &Provider{Protocol: "openai-compat", Model: "gpt-4o"}
	// "Hello world" is 2 tokens with cl100k_base / o200k_base.
	tokens, accuracy := CountTokensForProvider(p, "Hello world")
	if tokens != 2 {
		t.Errorf("openai-compat tokens = %d, want 2 (tiktoken accuracy)", tokens)
	}
	if accuracy != EstimateAccuracyTokenizerPct {
		t.Errorf("accuracy = %d, want %d", accuracy, EstimateAccuracyTokenizerPct)
	}
}

func TestCountTokensForProvider_AnthropicFallsBackToCharCount(t *testing.T) {
	p := &Provider{Protocol: "anthropic-compat", Model: "claude-opus-4-7"}
	tokens, accuracy := CountTokensForProvider(p, "Hello world")
	// Char fallback: 11 chars / 4 = 3 (rounded up).
	if tokens != 3 {
		t.Errorf("anthropic char-fallback tokens = %d, want 3", tokens)
	}
	if accuracy != EstimateAccuracyFallbackPct {
		t.Errorf("accuracy = %d, want %d (char-count fallback)", accuracy, EstimateAccuracyFallbackPct)
	}
}

func TestCountTokensForProvider_UnknownOpenAIModelFallsBackToCl100k(t *testing.T) {
	// deepseek-chat isn't in tiktoken's model registry, but
	// countTokensTiktoken falls back to cl100k_base which still
	// gives a real token count (better than char-count).
	p := &Provider{Protocol: "openai-compat", Model: "deepseek-chat"}
	tokens, accuracy := CountTokensForProvider(p, "Hello world")
	if tokens == 0 {
		t.Fatal("expected non-zero token count from cl100k_base fallback")
	}
	if accuracy != EstimateAccuracyTokenizerPct {
		t.Errorf("accuracy = %d, want %d (cl100k_base counts as native)", accuracy, EstimateAccuracyTokenizerPct)
	}
}

func TestCountTokensForProvider_EmptyInput(t *testing.T) {
	p := &Provider{Protocol: "openai-compat", Model: "gpt-4o"}
	tokens, _ := CountTokensForProvider(p, "")
	if tokens != 0 {
		t.Errorf("empty input tokens = %d, want 0", tokens)
	}
}

func floatNear(a, b, eps float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < eps
}
