package agents

import (
	"fmt"
	"strings"
	"time"
)

// EstimateAccuracyPct is the disclaimer band reported in --estimate's
// footer. Char-count fallback (current) is ±20% per spec D9; provider-
// native tokenizers (deferred to v0.6.x) will tighten to ±5%.
const EstimateAccuracyPct = 20

// CharsPerToken is the rough heuristic used by the char-count
// fallback tokenizer: 1 token ≈ 4 chars for English code/markdown.
// Real tokenizers (BPE) vary by content; this is the spec-documented
// fallback that gives ±20% accuracy on diff/code prompts.
const CharsPerToken = 4

// CountTokensCharFallback estimates token count from byte length using
// the 4-chars-per-token heuristic. Returns 0 for empty input. Used by
// estimate when no provider-native tokenizer is available.
func CountTokensCharFallback(s string) int {
	if s == "" {
		return 0
	}
	// Round up so a 1-char prompt counts as 1 token, not 0.
	return (len(s) + CharsPerToken - 1) / CharsPerToken
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
}

// ProjectCost computes the projected cost of one Invoke call given a
// resolved Provider + persona's prompt. Returns CostProjection with
// HasRateCard=false when the provider lacks pricing information.
//
// Char-count tokenizer is used unconditionally for v0.6 Stage 4;
// Stage 5+ may swap in provider-native tokenizers behind the same
// signature.
func ProjectCost(personaName string, provider *Provider, prompt string) CostProjection {
	in := CountTokensCharFallback(prompt)
	cp := CostProjection{
		PersonaName:    personaName,
		ProviderName:   provider.Name,
		DriverKind:     provider.Driver,
		InputTokens:    in,
		OutputEstimate: OutputTokensEstimate,
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
