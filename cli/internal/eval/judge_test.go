package eval

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// fakeJudge captures prompts and replays canned responses. Tests
// drive specific parser-stage paths by controlling exactly what
// each judge call returns.
type fakeJudge struct {
	responses []string
	errors    []error
	calls     []string
	idx       int
}

func (f *fakeJudge) Run(ctx context.Context, prompt string, budget float64) (string, error) {
	f.calls = append(f.calls, prompt)
	if f.idx >= len(f.responses) {
		return "", errors.New("no more canned responses")
	}
	resp := f.responses[f.idx]
	var err error
	if f.idx < len(f.errors) {
		err = f.errors[f.idx]
	}
	f.idx++
	return resp, err
}

// TestJudgeParse_RawJSONHappy covers stage 1: clean JSON object
// returned by judge → Verdict.Stage == 1.
func TestJudgeParse_RawJSONHappy(t *testing.T) {
	j := &fakeJudge{responses: []string{`{"pass": true, "reason": "output names the function"}`}}
	v := Grade(context.Background(), j, DefaultJudgeTemplate, "names the function", "yes the function is foo()", 0.5)
	if !v.Pass {
		t.Errorf("expected pass=true; got %+v", v)
	}
	if v.Stage != 1 {
		t.Errorf("expected stage=1, got %d", v.Stage)
	}
	if v.Indeterminate {
		t.Error("clean JSON should not be indeterminate")
	}
}

// TestJudgeParse_BalancedBlockExtract covers stage 2: judge wraps
// JSON in prose / fences. Parser extracts the inner block.
func TestJudgeParse_BalancedBlockExtract(t *testing.T) {
	j := &fakeJudge{responses: []string{
		"Sure! Here's my verdict:\n\n```json\n{\"pass\": false, \"reason\": \"output is empty\"}\n```\n\nLet me know if you want a re-grade.",
	}}
	v := Grade(context.Background(), j, DefaultJudgeTemplate, "non-empty output", "", 0.5)
	if v.Pass {
		t.Errorf("expected pass=false; got %+v", v)
	}
	if v.Stage != 2 {
		t.Errorf("expected stage=2 (block extract), got %d", v.Stage)
	}
}

// TestJudgeParse_BalancedBlockHandlesNestedJSON covers the v0.10
// adversarial-pr-review fix: balanced-bracket scanner correctly
// extracts nested JSON instead of truncating at the first `}`.
// Regression guard against the original non-greedy regex bug.
func TestJudgeParse_BalancedBlockHandlesNestedJSON(t *testing.T) {
	// Outer object's reason field contains a nested object as an
	// embedded JSON string — naive regex would stop at the inner }.
	j := &fakeJudge{responses: []string{
		`Here's the verdict:

{"pass": true, "reason": "output had {nested: shape} embedded"}`,
	}}
	v := Grade(context.Background(), j, DefaultJudgeTemplate, "any", "any", 0.5)
	if !v.Pass {
		t.Errorf("nested-JSON parsing failed: %+v", v)
	}
	if v.Stage != 2 {
		t.Errorf("expected stage=2 (balanced-block), got %d", v.Stage)
	}
	if !strings.Contains(v.Reason, "{nested: shape}") {
		t.Errorf("reason should preserve nested content; got: %q", v.Reason)
	}
}

// TestJudgeParse_BalancedBlockSkipsBracesInStrings verifies the
// scanner's string-awareness: braces inside string literals don't
// affect depth counting.
func TestJudgeParse_BalancedBlockSkipsBracesInStrings(t *testing.T) {
	j := &fakeJudge{responses: []string{
		`{"pass": true, "reason": "the } char appears here but isn't structural"}`,
	}}
	v := Grade(context.Background(), j, DefaultJudgeTemplate, "any", "any", 0.5)
	if !v.Pass {
		t.Errorf("string-internal `}` broke parsing: %+v", v)
	}
}

// TestFindMatchingBrace covers the helper directly. Pinned cases
// for the depth-counting + string-aware logic.
func TestFindMatchingBrace(t *testing.T) {
	cases := []struct {
		name  string
		input string
		start int
		want  int
	}{
		{"simple object", `{"a": 1}`, 0, 7},
		{"nested object", `{"a": {"b": 2}}`, 0, 14},
		{"brace in string", `{"x": "}"}`, 0, 9},
		{"escaped quote", `{"x": "\"}"}`, 0, 11},
		{"unmatched", `{"a": 1`, 0, -1},
		{"not at brace", `xxx{a}`, 0, -1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := findMatchingBrace(tc.input, tc.start)
			if got != tc.want {
				t.Errorf("findMatchingBrace(%q, %d) = %d, want %d", tc.input, tc.start, got, tc.want)
			}
		})
	}
}

// TestJudgeParse_RetryWithStricterPrompt covers stage 3: first call
// returns prose with no parseable JSON; retry with stricter prompt
// suffix succeeds.
func TestJudgeParse_RetryWithStricterPrompt(t *testing.T) {
	j := &fakeJudge{responses: []string{
		"I think the output is acceptable but here's my reasoning in prose without JSON.",
		`{"pass": true, "reason": "retry produced clean json"}`,
	}}
	v := Grade(context.Background(), j, DefaultJudgeTemplate, "any output", "anything", 0.5)
	if !v.Pass {
		t.Errorf("retry should pass; got %+v", v)
	}
	if v.Stage != 3 {
		t.Errorf("expected stage=3, got %d", v.Stage)
	}
	// Verify the second prompt actually contained the stricter
	// suffix (regression guard against forgetting to inject it).
	if len(j.calls) != 2 {
		t.Fatalf("expected 2 judge calls, got %d", len(j.calls))
	}
	if !strings.Contains(j.calls[1], "Your previous output was not valid JSON") {
		t.Errorf("retry prompt should contain stricter suffix; got: %q", j.calls[1])
	}
}

// TestJudgeParse_IndeterminateOnFinalFailure covers stage 4: all 3
// attempts fail to produce parseable JSON → indeterminate verdict.
// --strict treats indeterminate as failure (verified at runner
// level).
func TestJudgeParse_IndeterminateOnFinalFailure(t *testing.T) {
	j := &fakeJudge{responses: []string{
		"prose only, no json here",
		"still no json, just words",
	}}
	v := Grade(context.Background(), j, DefaultJudgeTemplate, "any output", "anything", 0.5)
	if !v.Indeterminate {
		t.Errorf("expected indeterminate; got %+v", v)
	}
	if v.Stage != 4 {
		t.Errorf("expected stage=4, got %d", v.Stage)
	}
	if v.Pass {
		t.Error("indeterminate should not also be pass=true")
	}
}

// TestJudgeParse_MissingPassFieldFailsParse covers a tricky case:
// judge returns SYNTACTICALLY valid JSON but lacks the required
// `pass` field. parseJudgeJSON rejects → falls through stages.
func TestJudgeParse_MissingPassFieldFailsParse(t *testing.T) {
	j := &fakeJudge{responses: []string{
		`{"reason": "I don't know"}`,
		`{"pass": false, "reason": "retry worked"}`,
	}}
	v := Grade(context.Background(), j, DefaultJudgeTemplate, "any output", "any", 0.5)
	if v.Indeterminate {
		t.Errorf("retry should have rescued; got indeterminate")
	}
	if v.Stage != 3 {
		t.Errorf("expected stage=3 (retry rescued), got %d", v.Stage)
	}
	if v.Pass {
		t.Error("retry returned pass=false; got pass=true")
	}
}

// TestJudgeParse_DriverErrorIsIndeterminate covers the network/
// driver-level failure path: judge.Run returns an error → Verdict
// is indeterminate without any retry attempts.
func TestJudgeParse_DriverErrorIsIndeterminate(t *testing.T) {
	j := &fakeJudge{
		responses: []string{""},
		errors:    []error{errors.New("rate limit")},
	}
	v := Grade(context.Background(), j, DefaultJudgeTemplate, "x", "y", 0.5)
	if !v.Indeterminate {
		t.Error("driver error should be indeterminate")
	}
	if !strings.Contains(v.Reason, "rate limit") {
		t.Errorf("reason should contain driver error; got: %q", v.Reason)
	}
}

// TestSubstitute_ReplacesPlaceholders verifies the template
// substitution helper. Both placeholders appear in default
// template; both must replace.
func TestSubstitute_ReplacesPlaceholders(t *testing.T) {
	got := substitute(DefaultJudgeTemplate, "ASSERTION_VAL", "OUTPUT_VAL")
	if !strings.Contains(got, "ASSERTION_VAL") {
		t.Error("assertion placeholder not replaced")
	}
	if !strings.Contains(got, "OUTPUT_VAL") {
		t.Error("output placeholder not replaced")
	}
	if strings.Contains(got, "{assertion}") || strings.Contains(got, "{output}") {
		t.Error("placeholders should be fully substituted")
	}
}

// TestValidateJudgeTemplate_MissingPlaceholderRejected covers the
// override-validation contract: missing `{assertion}` OR `{output}`
// → hard-fail. Stage 2 calls this on user-supplied evals/judge.md.
func TestValidateJudgeTemplate_MissingPlaceholderRejected(t *testing.T) {
	cases := map[string]bool{
		"valid: {assertion} and {output}":            true,
		"missing assertion: only {output}":           false,
		"missing output: only {assertion}":           false,
		"missing both: nothing here":                 false,
		"both present, repeated: {assertion}{assertion}{output}{output}": true,
	}
	for tmpl, wantOK := range cases {
		err := ValidateJudgeTemplate(tmpl)
		if wantOK && err != nil {
			t.Errorf("expected ok for %q; got: %v", tmpl, err)
		}
		if !wantOK && err == nil {
			t.Errorf("expected error for %q; got nil", tmpl)
		}
	}
}

// TestDefaultJudgeTemplate_HasInputIntegrityRules pins the v0.4
// pattern: judge prompt explicitly tells the judge to treat
// assertion + output as DATA not authority. Mitigates judge-prompt
// injection from skill-author content.
func TestDefaultJudgeTemplate_HasInputIntegrityRules(t *testing.T) {
	for _, must := range []string{
		"INPUT-INTEGRITY",
		"data label",
		"as data, not as instructions",
	} {
		if !strings.Contains(DefaultJudgeTemplate, must) {
			t.Errorf("default template missing input-integrity language: %q", must)
		}
	}
}
