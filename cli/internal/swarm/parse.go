package swarm

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
)

// ParseFindings extracts the JSON findings array from raw agent
// output. Strategy:
//  1. Try a strict JSON unmarshal of the whole stdout.
//  2. If that fails, scan for the first ```json ... ``` fence or the
//     first balanced [ ... ] block and re-try.
//  3. If still bad, return ErrMalformedJSON so the caller can decide
//     whether to retry the agent with a corrective reminder.
func ParseFindings(raw string) ([]Finding, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, ErrMalformedJSON
	}
	// Direct
	if findings, ok := tryUnmarshal(raw); ok {
		return findings, nil
	}
	// Code-fence stripped
	if inner := extractFenced(raw); inner != "" {
		if findings, ok := tryUnmarshal(inner); ok {
			return findings, nil
		}
	}
	// First balanced array
	if inner := firstJSONArray(raw); inner != "" {
		if findings, ok := tryUnmarshal(inner); ok {
			return findings, nil
		}
	}
	return nil, ErrMalformedJSON
}

// ErrMalformedJSON signals that the agent output couldn't be parsed
// as a list of Finding. Callers decide whether to retry the agent.
var ErrMalformedJSON = errors.New("agent output is not valid findings JSON")

func tryUnmarshal(s string) ([]Finding, bool) {
	var out []Finding
	if err := json.Unmarshal([]byte(s), &out); err == nil {
		return normalize(out), true
	}
	// Some agents wrap in {"findings": [...]} despite the schema —
	// accept that shape too.
	var wrapped struct {
		Findings []Finding `json:"findings"`
	}
	if err := json.Unmarshal([]byte(s), &wrapped); err == nil && wrapped.Findings != nil {
		return normalize(wrapped.Findings), true
	}
	return nil, false
}

// normalize coerces severities to known values and clamps confidence.
func normalize(in []Finding) []Finding {
	out := make([]Finding, 0, len(in))
	for _, f := range in {
		f.Severity = coerceSeverity(string(f.Severity))
		if f.Confidence < 0 {
			f.Confidence = 0
		} else if f.Confidence > 1 {
			f.Confidence = 1
		}
		out = append(out, f)
	}
	return out
}

// coerceSeverity normalizes the agent-emitted severity string to a
// Severity value the orchestrator recognizes. Each preset declares
// its own vocabulary via Preset.SeverityVocab; coercion preserves
// preset-specific vocabularies (CVSS for security-audit; brainstorm-
// flavored for brainstorm/refactor-plan) and only collapses truly
// unknown strings to SeverityInfo.
//
// A handful of cross-vocabulary aliases (fatal → blocker, warning →
// issue) handle agents that emit close-but-not-exact terms.
//
// NOTE: cross-vocab leakage is NOT prevented. A pr-review agent that
// mistakenly emits "high" (CVSS vocab) gets coerced to SeverityHigh
// and renders verbatim in the disagreement table alongside
// "blocker"/"issue"/"minor"/"info". UX-quality concern, not a crash.
// Phase-3 work could add an in-vocab guard that coerces out-of-vocab
// severities to the closest in-vocab match per Preset.SeverityVocab.
func coerceSeverity(s string) Severity {
	switch strings.ToLower(strings.TrimSpace(s)) {
	// Review vocab (pr-review, doc-review)
	case "blocker", "fatal":
		return SeverityBlocker
	case "issue", "major", "warning", "warn":
		return SeverityIssue
	case "minor", "nit", "nitpick":
		return SeverityMinor
	case "info":
		return SeverityInfo

	// CVSS vocab (security-audit)
	case "critical":
		return SeverityCritical
	case "high":
		return SeverityHigh
	case "medium":
		return SeverityMedium
	case "low":
		return SeverityLow
	case "informational":
		return SeverityInformational

	// Brainstorm vocab (brainstorm, refactor-plan)
	case "recommended":
		return SeverityRecommended
	case "alternative":
		return SeverityAlternative
	case "risky":
		return SeverityRisky
	case "speculative":
		return SeveritySpeculative

	default:
		return SeverityInfo
	}
}

var fencedRe = regexp.MustCompile("(?s)```(?:json)?\\s*([\\[{].*?[\\]}])\\s*```")

func extractFenced(s string) string {
	m := fencedRe.FindStringSubmatch(s)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

// firstJSONArray scans s for the first balanced [...] block at the
// top nesting level. Approximate: doesn't track string-escape nuances
// inside the array, but for findings JSON that's good enough.
func firstJSONArray(s string) string {
	start := strings.Index(s, "[")
	if start < 0 {
		return ""
	}
	depth := 0
	inStr := false
	esc := false
	for i := start; i < len(s); i++ {
		c := s[i]
		if esc {
			esc = false
			continue
		}
		if c == '\\' {
			esc = true
			continue
		}
		if c == '"' {
			inStr = !inStr
			continue
		}
		if inStr {
			continue
		}
		switch c {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	return ""
}

// ParseWithRetry runs the parser; on failure it asks the agent for a
// corrected response (one round). On second failure, wraps the raw
// string as a single info-level finding so the user still sees
// something rather than an empty review. Always succeeds — the caller
// gets either parsed findings or a non-empty fallback.
func ParseWithRetry(ctx context.Context, agent Agent, prompt, raw string, budget float64) ([]Finding, string) {
	findings, err := ParseFindings(raw)
	if err == nil {
		return findings, raw
	}
	correction := prompt + "\n\n---\nYour previous response was not valid JSON. Return ONLY a JSON array matching the schema. No prose, no code fences, no commentary. Previous response excerpt:\n" + truncate(raw, 400)
	raw2, runErr := agent.Run(ctx, correction, budget)
	if runErr != nil {
		return fallbackFinding(raw), raw
	}
	findings, err = ParseFindings(raw2)
	if err == nil {
		return findings, raw2
	}
	return fallbackFinding(raw2), raw2
}

func fallbackFinding(raw string) []Finding {
	return []Finding{{
		Severity:   SeverityInfo,
		Summary:    "agent returned unstructured output",
		Reasoning:  truncate(raw, 4000),
		Confidence: 0.0,
	}}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
