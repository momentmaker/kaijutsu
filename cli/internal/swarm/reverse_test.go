package swarm

import (
	"strings"
	"testing"
)

// TestReversePreset_RegisteredWithExpectedFields ensures the preset
// is wired into the default registry with the v0.9 contract: name,
// input kind, severity vocab, distinct marker prefix, fmt-safe
// debate template.
func TestReversePreset_RegisteredWithExpectedFields(t *testing.T) {
	got, err := defaultRegistry.Find("reverse")
	if err != nil {
		t.Fatalf("default registry should have 'reverse' preset: %v", err)
	}
	if got.Name != "reverse" {
		t.Errorf("name=%q, want 'reverse'", got.Name)
	}
	if got.InputKind != InputDiff {
		t.Errorf("InputKind=%v, want InputDiff", got.InputKind)
	}
	if len(got.SeverityVocab) == 0 {
		t.Error("SeverityVocab empty")
	}
}

// TestBuildReversePrompt_BakesSpecAndLeavesDiffSlot covers the spec-
// substitution contract: the spec placeholder is replaced, the diff
// slot remains as a single %s for ResolveInput to fill. Regression
// guard for the v0.8 dreamLensWild incident (stray %s rendered as
// %!s(MISSING) at runtime).
func TestBuildReversePrompt_BakesSpecAndLeavesDiffSlot(t *testing.T) {
	got := BuildReversePrompt("# my spec\n\n## In scope\n- thing 1\n")
	if !strings.Contains(got, "# my spec") {
		t.Error("spec content not baked into prompt")
	}
	if strings.Contains(got, "{{SPEC_CONTENT}}") {
		t.Error("spec placeholder still present after BuildReversePrompt")
	}
	// Exactly ONE %s slot remains — for the diff.
	if c := strings.Count(got, "%s"); c != 1 {
		t.Errorf("rendered prompt has %d %%s slots, want 1 (the diff)", c)
	}
}

// TestBuildReversePrompt_EmptySpecFallsBackToDefault verifies the
// nil-spec path: empty/whitespace spec → reverseDefaultPrompt (which
// has its own single %s slot for the diff).
func TestBuildReversePrompt_EmptySpecFallsBackToDefault(t *testing.T) {
	got := BuildReversePrompt("")
	if got != reverseDefaultPrompt {
		t.Error("empty spec should return reverseDefaultPrompt verbatim")
	}
	got = BuildReversePrompt("   \n\t  \n")
	if got != reverseDefaultPrompt {
		t.Error("whitespace-only spec should return reverseDefaultPrompt verbatim")
	}
}

// TestMarkerReverse_DistinctFromPRReview verifies sync-pr (Stage 5)
// can filter by marker prefix and distinguish reverse comments from
// pr-review comments. Both presets posting on the same PR must NOT
// collide on marker scheme.
func TestMarkerReverse_DistinctFromPRReview(t *testing.T) {
	prReview := Marker("run-1", "abc123")
	reverse := MarkerReverse("run-1", "abc123")
	if prReview == reverse {
		t.Error("reverse marker should differ from pr-review marker")
	}
	if !strings.Contains(reverse, "kaijutsu-reverse") {
		t.Errorf("reverse marker should contain 'kaijutsu-reverse'; got: %s", reverse)
	}
	if !strings.Contains(prReview, "kaijutsu-pr-review") {
		t.Errorf("pr-review marker should contain 'kaijutsu-pr-review'; got: %s", prReview)
	}
}

// TestReverseTruncate_OversizeDeterministic seeds findings with
// known severity + file ordering, sets a tight cap, asserts the
// truncation produces the same kept-set on every run.
func TestReverseTruncate_OversizeDeterministic(t *testing.T) {
	// Each finding ~100 bytes after JSON overhead. With cap=300
	// (just enough for ~2 findings + truncation footer), only the
	// top-priority entries survive.
	findings := []Finding{
		{Severity: SeverityInfo, File: "z.go", LineRange: "1", Summary: "info-z", Reasoning: "r"},
		{Severity: SeverityBlocker, File: "a.go", LineRange: "10", Summary: "blocker-a-10", Reasoning: "r"},
		{Severity: SeverityIssue, File: "b.go", LineRange: "5", Summary: "issue-b", Reasoning: "r"},
		{Severity: SeverityBlocker, File: "a.go", LineRange: "1", Summary: "blocker-a-1", Reasoning: "r"},
		{Severity: SeverityMinor, File: "c.go", LineRange: "1", Summary: "minor-c", Reasoning: "r"},
	}
	const cap1 = 300
	kept1, dropped1 := ReverseTruncate(findings, cap1)
	kept2, dropped2 := ReverseTruncate(findings, cap1)
	if len(kept1) != len(kept2) || dropped1 != dropped2 {
		t.Fatalf("non-deterministic: pass1=%d kept/%d dropped, pass2=%d kept/%d dropped",
			len(kept1), dropped1, len(kept2), dropped2)
	}
	for i := range kept1 {
		if kept1[i].Summary != kept2[i].Summary {
			t.Errorf("kept[%d] differs across runs: %q vs %q",
				i, kept1[i].Summary, kept2[i].Summary)
		}
	}

	// First entry must be the highest-priority finding: blocker on
	// "a.go" line 1 (severity DESC, file ASC, line ASC).
	if len(kept1) > 0 && kept1[0].Summary != "blocker-a-1" {
		t.Errorf("highest-priority sort wrong: kept[0]=%q, want 'blocker-a-1'", kept1[0].Summary)
	}
	// Second entry: blocker on "a.go" line 10.
	if len(kept1) > 1 && kept1[1].Summary != "blocker-a-10" {
		t.Errorf("second-priority sort wrong: kept[1]=%q, want 'blocker-a-10'", kept1[1].Summary)
	}
}

// TestReverseTruncate_NoTruncationWhenUnderCap verifies the no-op
// path: when the byte budget is generous, all findings are kept.
func TestReverseTruncate_NoTruncationWhenUnderCap(t *testing.T) {
	findings := []Finding{
		{Severity: SeverityInfo, File: "a.go", LineRange: "1", Summary: "x"},
	}
	kept, dropped := ReverseTruncate(findings, 10_000)
	if dropped != 0 {
		t.Errorf("dropped=%d, want 0 (under cap)", dropped)
	}
	if len(kept) != 1 {
		t.Errorf("kept=%d, want 1", len(kept))
	}
}

// TestReverseTruncate_EdgeCases covers degenerate inputs.
func TestReverseTruncate_EdgeCases(t *testing.T) {
	if kept, dropped := ReverseTruncate(nil, 1000); kept != nil || dropped != 0 {
		t.Errorf("nil findings → want (nil, 0), got (%v, %d)", kept, dropped)
	}
	if kept, dropped := ReverseTruncate([]Finding{}, 1000); len(kept) != 0 || dropped != 0 {
		t.Errorf("empty findings → want ([], 0), got (%v, %d)", kept, dropped)
	}
	// Cap 0 = pass-through (no truncation).
	in := []Finding{{Severity: SeverityInfo, File: "a", LineRange: "1", Summary: "x"}}
	kept, dropped := ReverseTruncate(in, 0)
	if len(kept) != 1 || dropped != 0 {
		t.Errorf("cap=0 → pass-through; got kept=%d dropped=%d", len(kept), dropped)
	}
}

// TestReverseSynth_ContainsCategoryAnchors locks in the synthesizer
// prompt's structure: required sections + drift-category anchors so
// downstream renderers (sync-pr, replay) know what to expect.
func TestReverseSynth_ContainsCategoryAnchors(t *testing.T) {
	for _, want := range []string{
		"### 1. Drift findings by category",
		"### 2. Cross-agent disagreements",
		"### 3. Single-reporter highlights",
		"ADDED",
		"OMITTED",
		"CHANGED",
		"AMBIGUOUS",
	} {
		if !strings.Contains(reverseSynthesizer, want) {
			t.Errorf("reverseSynthesizer missing required token %q", want)
		}
	}
}

// TestLineRangeStart_ParsesBoundary verifies the sort tie-break
// helper: "42-58" → 42, "42" → 42, "" → 0, "abc" → 0.
func TestLineRangeStart_ParsesBoundary(t *testing.T) {
	cases := map[string]int{
		"42":    42,
		"42-58": 42,
		"":      0,
		"abc":   0,
		"0":     0,
		"100":   100,
	}
	for in, want := range cases {
		if got := lineRangeStart(in); got != want {
			t.Errorf("lineRangeStart(%q) = %d, want %d", in, got, want)
		}
	}
}
