package agents

import (
	"fmt"
	"strings"
	"time"

	"github.com/pkoukk/tiktoken-go"
)

// EstimateAccuracyPct is the disclaimer band reported in --estimate's
// footer. tiktoken-go for openai-compat providers tightens accuracy
// to ±5%; char-count fallback for unknown providers + Anthropic is
// ±20% (Anthropic's count_tokens API requires a key + network call,
// not worth the round-trip in a dry-run command).
const (
	EstimateAccuracyTokenizerPct = 5
	EstimateAccuracyFallbackPct  = 20
	// EstimateAccuracyPct is the LARGER of the two — surfaces the
	// worst case in the footer disclaimer when a swarm spans both
	// kinds of providers.
	EstimateAccuracyPct = EstimateAccuracyFallbackPct
)

// CharsPerToken is the rough heuristic used by the char-count
// fallback tokenizer: 1 token ≈ 4 chars for English code/markdown.
// Real tokenizers (BPE) vary by content; this is the spec-documented
// fallback that gives ±20% accuracy on diff/code prompts.
const CharsPerToken = 4

// CountTokensCharFallback estimates token count from byte length using
// the 4-chars-per-token heuristic. Returns 0 for empty input. Used by
// estimate when no provider-native tokenizer is available (Anthropic
// providers, unknown protocols).
func CountTokensCharFallback(s string) int {
	if s == "" {
		return 0
	}
	// Round up so a 1-char prompt counts as 1 token, not 0.
	return (len(s) + CharsPerToken - 1) / CharsPerToken
}

// CountTokensForProvider picks the most accurate tokenizer for a given
// provider and falls back to char-count when no native tokenizer is
// available.
//
// Returns (tokenCount, accuracyPct). accuracyPct is 5 when a native
// tokenizer ran and 20 for the char-count fallback — caller surfaces
// this in the --estimate footer.
func CountTokensForProvider(p *Provider, s string) (int, int) {
	if s == "" {
		return 0, EstimateAccuracyTokenizerPct
	}
	if p != nil && p.Protocol == "openai-compat" {
		if n, ok := countTokensTiktoken(p.Model, s); ok {
			return n, EstimateAccuracyTokenizerPct
		}
	}
	return CountTokensCharFallback(s), EstimateAccuracyFallbackPct
}

// OutputTokensEstimate is a fixed conservative estimate of how many
// tokens a typical swarm response generates. Real responses vary
// wildly (a finding-list might be 50 tokens; a debate critique might
// be 2000). 800 is the median observed during v0.5 swarm runs.
const OutputTokensEstimate = 800

// CostProjection bundles the per-call projection rendered by
// `jutsu swarm <preset> --estimate`.
type CostProjection struct {
	PersonaName    string
	ProviderName   string
	DriverKind     DriverKind
	InputTokens    int
	OutputEstimate int
	CostUSD        float64
	RateCardDate   string  // empty when no rate card
	Stale          bool    // true when rate_card_date is older than 90 days
	StaleDays      int     // age of rate card; 0 if not stale or absent
	HasRateCard    bool
	// AccuracyPct is the tokenizer's accuracy band for this projection
	// (5 for native tiktoken, 20 for char-count fallback). Used by
	// the footer to show the worst case across all rows.
	AccuracyPct int
}

// countTokensTiktoken runs tiktoken-go against the provider's model.
// Returns (tokens, true) on success, (0, false) when the model isn't
// recognized by tiktoken (unknown openai-compat models like
// deepseek-chat fall back to cl100k_base which is close enough for
// dry-run purposes).
func countTokensTiktoken(model, s string) (int, bool) {
	enc, err := tiktoken.EncodingForModel(model)
	if err != nil {
		// Unknown model: fall back to cl100k_base (GPT-4 / GPT-3.5
		// tokenizer; covers most openai-compat dialects within ±10%).
		enc, err = tiktoken.GetEncoding("cl100k_base")
		if err != nil {
			return 0, false
		}
	}
	return len(enc.Encode(s, nil, nil)), true
}

// ProjectCost computes the projected cost of one Invoke call given a
// resolved Provider + persona's prompt. Returns CostProjection with
// HasRateCard=false when the provider lacks pricing information.
//
// Char-count tokenizer is used unconditionally for v0.6 Stage 4;
// Stage 5+ may swap in provider-native tokenizers behind the same
// signature.
//
// Cost shown is a CONSERVATIVE upper bound — the projection assumes
// every input token is uncached and billed at full InputPerMtok rate.
// Real runs that hit provider prompt cache (Anthropic ephemeral, OpenAI
// 1024-token threshold) bill cached input at 10-50% of normal rate.
// `--estimate` shows the worst case so users budget against the ceiling.
func ProjectCost(personaName string, provider *Provider, prompt string) CostProjection {
	in, accuracy := CountTokensForProvider(provider, prompt)
	cp := CostProjection{
		PersonaName:    personaName,
		ProviderName:   provider.Name,
		DriverKind:     provider.Driver,
		InputTokens:    in,
		OutputEstimate: OutputTokensEstimate,
		AccuracyPct:    accuracy,
	}
	if provider.Cost == nil {
		return cp
	}
	cp.HasRateCard = true
	cp.RateCardDate = provider.Cost.RateCardDate
	const perMtok = 1_000_000.0
	cp.CostUSD = float64(in)*provider.Cost.InputPerMtok/perMtok +
		float64(OutputTokensEstimate)*provider.Cost.OutputPerMtok/perMtok

	if days, ok := rateCardAgeDays(provider.Cost.RateCardDate); ok && days > 90 {
		cp.Stale = true
		cp.StaleDays = days
	}
	return cp
}

// rateCardAgeDays parses YYYY-MM-DD and returns the age in days. When
// the date is unparseable or in the future, ok=false.
func rateCardAgeDays(rateCardDate string) (int, bool) {
	if strings.TrimSpace(rateCardDate) == "" {
		return 0, false
	}
	t, err := time.Parse("2006-01-02", rateCardDate)
	if err != nil {
		return 0, false
	}
	age := int(time.Since(t).Hours() / 24)
	if age < 0 {
		return 0, false
	}
	return age, true
}

// FormatProjection renders a CostProjection as a single table row for
// `jutsu swarm <preset> --estimate` output. Used by the cli package to
// build the table; here so the formatting stays in lockstep with the
// CostProjection struct's field meaning.
func FormatProjection(cp CostProjection) string {
	costStr := "(no rate card)"
	if cp.HasRateCard {
		costStr = fmt.Sprintf("$%.4f", cp.CostUSD)
	}
	stale := ""
	if cp.Stale {
		stale = fmt.Sprintf(" [stale rate card: %dd old]", cp.StaleDays)
	}
	return fmt.Sprintf("%-30s %-12s %6d %6d  %s%s",
		cp.PersonaName, cp.DriverKind, cp.InputTokens, cp.OutputEstimate, costStr, stale)
}
