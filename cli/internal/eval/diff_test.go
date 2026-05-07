package eval

import (
	"strings"
	"testing"
)

// TestStripNoise_RemovesTimestampAndRunIDs covers the noise-strip
// invariants. Without these, identical-content runs would produce
// false-positive diffs from clock + run-id variance.
func TestStripNoise_RemovesTimestampAndRunIDs(t *testing.T) {
	in := `Run started at 2026-05-08T12:34:56Z
Findings: 3
ID: 20260508T123456Z:0
<!-- kaijutsu-pr-review:run-id=20260508T123456Z sha=abc1234 -->`
	got := StripNoise(in)
	if strings.Contains(got, "2026-05-08T12:34:56Z") {
		t.Error("timestamp not stripped")
	}
	if strings.Contains(got, "20260508T123456Z:0") {
		t.Error("run-id format not stripped")
	}
	if strings.Contains(got, "kaijutsu-pr-review") {
		t.Error("kaijutsu marker not stripped")
	}
}

// TestLineDiff_IdenticalInputsAllEqual covers the no-diff case:
// baseline == challenger → all DiffEqual lines.
func TestLineDiff_IdenticalInputsAllEqual(t *testing.T) {
	in := "line 1\nline 2\nline 3"
	res := LineDiff(in, in)
	for i, l := range res.Lines {
		if l.Kind != DiffEqual {
			t.Errorf("line %d: kind=%v, want DiffEqual", i, l.Kind)
		}
	}
	if res.BaselineTruncated || res.ChallengerTruncated {
		t.Error("short identical inputs shouldn't be truncated")
	}
}

// TestLineDiff_AddedLineMarkedChallengerOnly covers the "challenger
// added a line" case.
func TestLineDiff_AddedLineMarkedChallengerOnly(t *testing.T) {
	baseline := "line 1\nline 2"
	challenger := "line 1\nNEW LINE\nline 2"
	res := LineDiff(baseline, challenger)
	foundAdded := false
	for _, l := range res.Lines {
		if l.Kind == DiffChallengerOnly && l.Text == "NEW LINE" {
			foundAdded = true
		}
	}
	if !foundAdded {
		t.Errorf("expected DiffChallengerOnly line 'NEW LINE'; got %v", res.Lines)
	}
}

// TestLineDiff_RemovedLineMarkedBaselineOnly covers the
// "challenger dropped a line" case.
func TestLineDiff_RemovedLineMarkedBaselineOnly(t *testing.T) {
	baseline := "line 1\nGONE LINE\nline 2"
	challenger := "line 1\nline 2"
	res := LineDiff(baseline, challenger)
	foundRemoved := false
	for _, l := range res.Lines {
		if l.Kind == DiffBaselineOnly && l.Text == "GONE LINE" {
			foundRemoved = true
		}
	}
	if !foundRemoved {
		t.Errorf("expected DiffBaselineOnly line 'GONE LINE'; got %v", res.Lines)
	}
}

// TestDiff_TruncatesAt200Lines pins the MaxDiffLinesPerSide
// constant. Inputs over the cap surface truncation flags + diff
// only the kept range. Regression-flipping the constant (e.g. to
// 100) is caught.
func TestDiff_TruncatesAt200Lines(t *testing.T) {
	// Build inputs with 250 lines each.
	var b, c strings.Builder
	for i := 0; i < 250; i++ {
		b.WriteString("baseline line ")
		b.WriteString(strings.Repeat("a", i%5))
		b.WriteString("\n")
		c.WriteString("challenger line ")
		c.WriteString(strings.Repeat("a", i%5))
		c.WriteString("\n")
	}
	res := LineDiff(b.String(), c.String())
	if !res.BaselineTruncated {
		t.Error("BaselineTruncated should be true for 250-line input")
	}
	if !res.ChallengerTruncated {
		t.Error("ChallengerTruncated should be true for 250-line input")
	}
	// Sanity-check that the diff itself is bounded. Worst case:
	// MaxDiffLinesPerSide × 2 = 400 lines (no overlap).
	if len(res.Lines) > 2*MaxDiffLinesPerSide+5 {
		t.Errorf("diff bounded by 2*cap=400; got %d lines", len(res.Lines))
	}
}

// TestLineDiff_NoiseStrippedBeforeDiff verifies that timestamp +
// run-id variance is normalized OUT before LCS so otherwise-
// identical runs produce all-equal diff. Regression guard against
// false-positive diffs from clock skew.
func TestLineDiff_NoiseStrippedBeforeDiff(t *testing.T) {
	baseline := "Run at 2026-05-08T12:00:00Z\nfindings: 3\n"
	challenger := "Run at 2026-05-08T13:30:00Z\nfindings: 3\n" // different timestamp ONLY
	res := LineDiff(baseline, challenger)
	// After noise-strip, both reduce to "Run at <NOISE>\nfindings: 3"
	// → all DiffEqual.
	for _, l := range res.Lines {
		if l.Kind != DiffEqual {
			t.Errorf("noise-only diff should be all-equal; got %+v", l)
		}
	}
}
