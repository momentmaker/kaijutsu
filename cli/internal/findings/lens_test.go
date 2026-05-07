package findings

import "testing"

// TestLensFromSummary_StableShape pins the regex grammar: every
// canonical lens prefix returns the lens name; everything outside
// the 8-lens whitelist returns "". Guards against accidental drift
// in dreamSummaryPattern (the regex shape is the v0.9 schema-
// migration source-of-truth — recorder, validator, and synthesizer
// all share it).
func TestLensFromSummary_StableShape(t *testing.T) {
	cases := []struct {
		summary string
		want    string
	}{
		// 8 canonical lens prefixes (v0.9 frozen set).
		{"[lens:honest] take", "honest"},
		{"[lens:fit] take", "fit"},
		{"[lens:gaps] take", "gaps"},
		{"[lens:wild] take", "wild"},
		{"[lens:adversary] take", "adversary"},
		{"[lens:inverse] take", "inverse"},
		{"[lens:status-quo] take", "status-quo"},
		{"[lens:time] take", "time"},

		// Whitespace tolerance: 0 and 2+ spaces after bracket OK.
		{"[lens:honest]thought", "honest"},
		{"[lens:honest]   thought", "honest"},

		// Negative cases — must return "".
		{"", ""},
		{"no prefix", ""},
		{"[lens:unknown] take", ""},     // not in whitelist
		{"[lens:HONEST] take", ""},      // case-sensitive (whitelist is lower)
		{"[lens:gaps]", ""},             // no trailing content
		{"prefix [lens:gaps] mid", ""},  // not at start
		{"[lens:] take", ""},            // empty lens name
	}
	for _, tc := range cases {
		got := LensFromSummary(tc.summary)
		if got != tc.want {
			t.Errorf("LensFromSummary(%q) = %q, want %q", tc.summary, got, tc.want)
		}
	}
}
