package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// TestSuggest_PRReviewTopsRanking verifies the ranking algorithm
// catches the obvious case: "review my PR" → pr-review at the top.
// Pre-fix: the 2-char "PR" token was dropped by a min-3 filter, so
// doc-review (which has "review" in its description more often) won
// the wrong query. Test guards against the regression.
func TestSuggest_PRReviewTopsRanking(t *testing.T) {
	root := NewRootCmd()
	var stdout bytes.Buffer
	root.SetOut(&stdout)
	root.SetArgs([]string{"suggest", "review my PR", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("suggest execute: %v", err)
	}

	var payload struct {
		Suggestions []suggestion `json:"suggestions"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("parse JSON: %v\noutput:\n%s", err, stdout.String())
	}
	if len(payload.Suggestions) == 0 {
		t.Fatal("no suggestions returned")
	}
	if payload.Suggestions[0].Name != "pr-review" {
		t.Errorf("top suggestion = %q, want pr-review", payload.Suggestions[0].Name)
	}
}

// TestSuggest_DreamMatchesKeywords verifies dream surfaces for
// queries that actually contain dream-anchored keywords (lens,
// interrogation, exploration). v0.8.2 ships keyword ranking only
// — semantic queries like "should I build X" are a v0.9 LLM/
// embedding-ranker concern. This test guards the keyword path,
// not the semantic wishlist.
func TestSuggest_DreamMatchesKeywords(t *testing.T) {
	root := NewRootCmd()
	var stdout bytes.Buffer
	root.SetOut(&stdout)
	root.SetArgs([]string{"suggest", "interrogation lens exploration", "--json", "--top", "3"})
	_ = root.Execute()

	if !strings.Contains(stdout.String(), `"name": "dream"`) {
		t.Errorf("dream missing from top-3 for interrogation/lens query; output:\n%s", stdout.String())
	}
}

// TestSuggest_JSONHasSchemaVersion guards the v0.8.2 output contract:
// schema_version + task + count + suggestions array shape. Agents
// parsing the output depend on this being stable.
func TestSuggest_JSONHasSchemaVersion(t *testing.T) {
	root := NewRootCmd()
	var stdout bytes.Buffer
	root.SetOut(&stdout)
	root.SetArgs([]string{"suggest", "test query", "--json"})
	_ = root.Execute()

	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("parse JSON: %v", err)
	}
	if v, ok := payload["schema_version"].(float64); !ok || v != 1 {
		t.Errorf("schema_version = %v, want 1", payload["schema_version"])
	}
	if _, ok := payload["task"]; !ok {
		t.Error("missing 'task' field")
	}
	if _, ok := payload["suggestions"]; !ok {
		t.Error("missing 'suggestions' field")
	}
}

// TestSuggest_TopFlagLimitsResults verifies --top N caps the result
// count. Default is 5; explicit --top 2 should return ≤ 2.
func TestSuggest_TopFlagLimitsResults(t *testing.T) {
	root := NewRootCmd()
	var stdout bytes.Buffer
	root.SetOut(&stdout)
	root.SetArgs([]string{"suggest", "review", "--json", "--top", "2"})
	_ = root.Execute()

	var payload struct {
		Suggestions []suggestion `json:"suggestions"`
	}
	_ = json.Unmarshal(stdout.Bytes(), &payload)
	if len(payload.Suggestions) > 2 {
		t.Errorf("--top 2 returned %d suggestions, want ≤ 2", len(payload.Suggestions))
	}
}

// TestTokenize_StopwordsAndShortTokens covers the token extraction
// logic. Short noise like "a"/"i" dropped; meaningful 2-char tokens
// like "pr"/"go" kept; stopwords stripped.
func TestTokenize_StopwordsAndShortTokens(t *testing.T) {
	cases := map[string][]string{
		"review my PR":                   {"review", "pr"},
		"how can I write Go tests":       {"write", "go", "tests"}, // can/how dropped (stopwords); I dropped (1-char); Go/tests are domain tokens, KEEP
		"what should we do for the auth": {"auth"},          // most are stopwords
		"":                               {},
		"a I":                            {}, // all single-char
	}
	for in, want := range cases {
		got := tokenize(in)
		if len(got) != len(want) {
			t.Errorf("tokenize(%q) = %v, want %v", in, got, want)
			continue
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("tokenize(%q)[%d] = %q, want %q", in, i, got[i], want[i])
			}
		}
	}
}
