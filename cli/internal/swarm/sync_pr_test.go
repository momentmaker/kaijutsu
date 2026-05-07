package swarm

import (
	"reflect"
	"testing"
)

// TestParseReplyKeywords_HappyPath covers the simple accept + dismiss
// case across multiple action lines in a single reply.
func TestParseReplyKeywords_HappyPath(t *testing.T) {
	body := "Looks good overall.\n\naccept: 20260507T1200Z:0\ndismiss: 20260507T1200Z:1\n"
	got := ParseReplyKeywords(body)
	want := []SyncPRAction{
		{Verb: "accepted", RunID: "20260507T1200Z", Position: 0, Raw: "accept: 20260507T1200Z:0"},
		{Verb: "dismissed", RunID: "20260507T1200Z", Position: 1, Raw: "dismiss: 20260507T1200Z:1"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

// TestParseReplyKeywords_GrammarVariants pins the spec grammar:
// case-insensitive verb, leading whitespace, `>`-quoting,
// whitespace around colon. Each case is a 1-line reply.
func TestParseReplyKeywords_GrammarVariants(t *testing.T) {
	cases := []struct {
		name string
		body string
		ok   bool
	}{
		{"lowercase verb", "accept: X:0", true},
		{"capitalized verb", "Accept: X:0", true},
		{"uppercase verb", "ACCEPT: X:0", true},
		{"leading whitespace", "    accept: X:0", true},
		{"leading > quote", "> accept: X:0", true},
		{"multi > quote", ">> accept: X:0", true},
		{"whitespace around colon between verb and id", "accept   :   X:0", true},
		{"trailing whitespace", "accept: X:0   ", true},
		// Negative cases.
		{"missing colon between verb and id", "accept X:0", false},
		{"non-numeric position", "accept: X:foo", false},
		{"missing position", "accept: X:", false},
		{"verb-only line", "accept", false},
		{"unknown verb", "approve: X:0", false},
		{"comma-separated multi-id (not supported)", "accept: X:0, X:1", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseReplyKeywords(tc.body)
			if tc.ok && len(got) == 0 {
				t.Errorf("expected match; got none for body %q", tc.body)
			}
			if !tc.ok && len(got) > 0 {
				t.Errorf("expected no match; got %+v for body %q", got, tc.body)
			}
		})
	}
}

// TestParseReplyKeywords_EmptyAndNoMatches covers degenerate inputs.
func TestParseReplyKeywords_EmptyAndNoMatches(t *testing.T) {
	if got := ParseReplyKeywords(""); got != nil {
		t.Errorf("empty body → want nil, got %v", got)
	}
	if got := ParseReplyKeywords("just prose, no actions here"); got != nil {
		t.Errorf("no-match body → want nil, got %v", got)
	}
}

// TestIsScopedReply_PriorityChain covers the spec's priority rules
// for run-id resolution.
func TestIsScopedReply_PriorityChain(t *testing.T) {
	cases := []struct {
		name        string
		body        string
		anchorRunID string
		want        string
	}{
		{"priority 1: threaded reply", "anything", "RUN_THREAD", "RUN_THREAD"},
		{"priority 2: explicit run-id reference", "Hi, see run-id=RUN_REF for context.", "", "RUN_REF"},
		{"priority 2: quoted marker line", "> <!-- kaijutsu-pr-review:run-id=RUN_MARKER sha=abc -->", "", "RUN_MARKER"},
		{"priority 3: unscoped", "no scope here", "", ""},
		{"priority 1 wins over priority 2", "run-id=IGNORED here", "ANCHOR_WINS", "ANCHOR_WINS"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := IsScopedReply(tc.body, tc.anchorRunID)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
