package eval

import (
	"regexp"
	"strings"
)

// MaxDiffLinesPerSide is the truncation cap for the report's diff
// view. v0.10 ships 200 lines per side; longer outputs surface a
// "[truncated]" marker so the user knows to look at the raw output
// file instead.
const MaxDiffLinesPerSide = 200

// nonDeterministicNoiseRegexes strip content known to vary between
// otherwise-identical runs. The diff view is meant to surface what
// SKILL/PERSONA/PRESET loading changes, not what wall-clock noise
// changes. Patterns:
//   - ISO-8601 timestamps with timezone (run timestamps)
//   - run-ids of shape <[A-Za-z0-9]+>:<digits> (kaijutsu marker IDs)
//   - kaijutsu HTML comment markers (full marker line)
var nonDeterministicNoiseRegexes = []*regexp.Regexp{
	regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[-+]\d{2}:?\d{2})`),
	regexp.MustCompile(`[A-Za-z0-9]+:\d+`),
	regexp.MustCompile(`<!-- kaijutsu-[a-z-]+:[^>]+-->`),
}

// StripNoise removes non-deterministic content from a diff input
// per the regex set. Returns the cleaned string with noise replaced
// by `<NOISE>` placeholders so the LCS diff highlights real
// differences only.
func StripNoise(s string) string {
	for _, re := range nonDeterministicNoiseRegexes {
		s = re.ReplaceAllString(s, "<NOISE>")
	}
	return s
}

// DiffResult carries the line-level comparison output for the
// report's diff section. Truncated bookkeeping lets the renderer
// surface "[truncated, see raw output for full text]".
type DiffResult struct {
	Lines       []DiffLine
	BaselineTruncated  bool
	ChallengerTruncated bool
}

// DiffLine is one row in the line-by-line diff.
type DiffLine struct {
	Kind  DiffKind
	Text  string
}

type DiffKind int

const (
	DiffEqual DiffKind = iota
	DiffBaselineOnly
	DiffChallengerOnly
)

// LineDiff produces a line-level diff between baseline and
// challenger outputs. Algorithm: split on `\n`, strip noise, run
// LCS over the line streams, emit (equal | baseline-only |
// challenger-only) lines in order. Truncation: each side capped at
// MaxDiffLinesPerSide BEFORE diffing so the algo's worst-case stays
// O(N²) bounded at ~40k cells (200×200).
//
// Trade: we lose the ability to show diffs whose load-bearing
// content lives past line 200. v0.10 marks that case via the
// Truncated flags so users can open the raw output. v0.10.x
// candidate: smarter chunking that finds the meaningful delta
// region.
func LineDiff(baseline, challenger string) DiffResult {
	bLines := strings.Split(StripNoise(baseline), "\n")
	cLines := strings.Split(StripNoise(challenger), "\n")
	res := DiffResult{}
	if len(bLines) > MaxDiffLinesPerSide {
		bLines = bLines[:MaxDiffLinesPerSide]
		res.BaselineTruncated = true
	}
	if len(cLines) > MaxDiffLinesPerSide {
		cLines = cLines[:MaxDiffLinesPerSide]
		res.ChallengerTruncated = true
	}
	res.Lines = lcsDiff(bLines, cLines)
	return res
}

// lcsDiff is the standard dynamic-programming LCS algorithm followed
// by a backtrack to emit the diff lines. Bounded at the truncation
// cap × cap = 40k cells; trivially fast at v0.10 expected scales.
func lcsDiff(a, b []string) []DiffLine {
	m, n := len(a), len(b)
	if m == 0 && n == 0 {
		return nil
	}
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if a[i-1] == b[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else {
				if dp[i-1][j] >= dp[i][j-1] {
					dp[i][j] = dp[i-1][j]
				} else {
					dp[i][j] = dp[i][j-1]
				}
			}
		}
	}
	// Backtrack from (m, n) to (0, 0); emit lines as we go;
	// reverse at the end so output is in source order.
	var out []DiffLine
	i, j := m, n
	for i > 0 || j > 0 {
		switch {
		case i > 0 && j > 0 && a[i-1] == b[j-1]:
			out = append(out, DiffLine{Kind: DiffEqual, Text: a[i-1]})
			i--
			j--
		case j > 0 && (i == 0 || dp[i][j-1] >= dp[i-1][j]):
			out = append(out, DiffLine{Kind: DiffChallengerOnly, Text: b[j-1]})
			j--
		default:
			out = append(out, DiffLine{Kind: DiffBaselineOnly, Text: a[i-1]})
			i--
		}
	}
	// Reverse in place.
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}
