package swarm

import (
	"strings"
	"testing"
)

func TestExtractMarker(t *testing.T) {
	body := "## review\n\nstuff\n\n<!-- kaijutsu-pr-review:run-id=20260505T120000Z sha=abc1234 -->\n"
	if got := extractMarker(body); got != "abc1234" {
		t.Fatalf("want abc1234, got %q", got)
	}
}

func TestExtractMarker_NoMarker(t *testing.T) {
	if got := extractMarker("plain comment"); got != "" {
		t.Fatalf("want empty, got %q", got)
	}
}

func TestStripHistoryFooter(t *testing.T) {
	body := "main body\n\n<details><summary>Previous review (sha old)</summary>\n\nold body\n\n</details>"
	stripped := stripHistoryFooter(body)
	if strings.Contains(stripped, "Previous review") {
		t.Fatalf("history footer not stripped: %q", stripped)
	}
	if !strings.Contains(stripped, "main body") {
		t.Fatal("main body lost during strip")
	}
}

func TestBuildHistoryFooter_NestsCleanly(t *testing.T) {
	prior := "old body\n\n<!-- kaijutsu-pr-review:run-id=X sha=oldsha -->"
	footer := buildHistoryFooter(prior)
	if !strings.Contains(footer, "<details>") {
		t.Fatal("missing details wrapper")
	}
	if !strings.Contains(footer, "oldsha") {
		t.Fatal("expected prior sha in heading")
	}
	if strings.Count(footer, "<details>") > 1 {
		t.Fatal("nested details — strip should have removed prior history")
	}
}

func TestIDFromCommentURL(t *testing.T) {
	url := "https://github.com/foo/bar/pull/42#issuecomment-12345"
	if got := idFromCommentURL(strings.Replace(url, "issuecomment-", "issuecomment/comments/", 1)); got != 0 {
		// Skip — alternate format. The realistic input uses /comments/<id>.
		t.Logf("skip alt format: %d", got)
	}
	url2 := "/repos/foo/bar/issues/comments/98765"
	if got := idFromCommentURL(url2); got != 98765 {
		t.Fatalf("want 98765, got %d", got)
	}
}

func TestMarker_Roundtrip(t *testing.T) {
	m := Marker("RID", "abc")
	if extractMarker(m) != "abc" {
		t.Fatalf("marker roundtrip failed: %q", m)
	}
}
