package swarm

import (
	"strings"
	"testing"
)

func TestClusterFindings_ConsensusAndOrder(t *testing.T) {
	results := []AgentResult{
		{Agent: "claude", Findings: []Finding{
			{Severity: SeverityBlocker, File: "a.go", LineRange: "10", Summary: "race"},
			{Severity: SeverityMinor, File: "b.go", LineRange: "20", Summary: "nit"},
		}},
		{Agent: "codex", Findings: []Finding{
			{Severity: SeverityIssue, File: "a.go", LineRange: "10", Summary: "race (different wording)"},
		}},
		{Agent: "gemini", Findings: []Finding{
			{Severity: SeverityIssue, File: "c.go", LineRange: "5", Summary: "lone wolf"},
		}},
	}
	clusters := clusterFindings(results)
	if len(clusters) != 3 {
		t.Fatalf("want 3 clusters, got %d", len(clusters))
	}
	// First cluster is the consensus blocker (highest severity wins
	// the merge).
	if clusters[0].Key != "a.go:10" {
		t.Errorf("want a.go:10 first, got %s", clusters[0].Key)
	}
	if clusters[0].Severity != SeverityBlocker {
		t.Errorf("want blocker severity (max across reporters), got %s", clusters[0].Severity)
	}
	if clusters[0].ConsensusOf != 2 {
		t.Errorf("want 2 reporters, got %d", clusters[0].ConsensusOf)
	}
	if clusters[0].OutOfTotal != 3 {
		t.Errorf("want 3 total, got %d", clusters[0].OutOfTotal)
	}
	// Lone-wolf finding should appear; its consensus is 1.
	loneFound := false
	for _, c := range clusters {
		if c.Key == "c.go:5" && c.ConsensusOf == 1 {
			loneFound = true
		}
	}
	if !loneFound {
		t.Error("lone-wolf c.go:5 cluster missing or wrong consensus")
	}
}

func TestRenderDisagreementTable_OrdersAgentsCanonically(t *testing.T) {
	results := []AgentResult{
		{Agent: "gemini", Findings: []Finding{{Severity: SeverityIssue, File: "x", LineRange: "1", Summary: "s"}}},
		{Agent: "claude", Findings: []Finding{{Severity: SeverityIssue, File: "x", LineRange: "1", Summary: "s"}}},
	}
	clusters := clusterFindings(results)
	table := renderDisagreementTable(results, clusters)
	// claude column header must appear before gemini (canonical order).
	cIdx := strings.Index(table, "claude")
	gIdx := strings.Index(table, "gemini")
	if cIdx < 0 || gIdx < 0 || cIdx > gIdx {
		t.Fatalf("expected claude before gemini, got table:\n%s", table)
	}
}

func TestRenderDisagreementTable_EmptyOnNoClusters(t *testing.T) {
	if got := renderDisagreementTable(nil, nil); got != "" {
		t.Fatalf("want empty, got %q", got)
	}
}

func TestEscapePipes(t *testing.T) {
	if escapePipes("a|b|c") != "a\\|b\\|c" {
		t.Fail()
	}
}

func TestAssembleMarkdown_HeaderUsesPresetName(t *testing.T) {
	cases := []struct {
		preset *Preset
		want   string
	}{
		{&Preset{Name: "pr-review"}, "## kaijutsu pr-review"},
		{&Preset{Name: "doc-review"}, "## kaijutsu doc-review"},
		{&Preset{Name: "brainstorm"}, "## kaijutsu brainstorm"},
		{&Preset{Name: "refactor-plan"}, "## kaijutsu refactor-plan"},
		{&Preset{Name: "security-audit"}, "## kaijutsu security-audit"},
	}
	for _, tc := range cases {
		t.Run(tc.preset.Name, func(t *testing.T) {
			md := assembleMarkdown(tc.preset, nil, "", "draft", nil)
			if !strings.HasPrefix(md, tc.want+"\n\n") {
				t.Errorf("preset %q: expected prefix %q, got first 50 chars %q", tc.preset.Name, tc.want, md[:50])
			}
		})
	}
}

func TestFallbackMarkdown_HeaderUsesPresetName(t *testing.T) {
	preset := &Preset{Name: "doc-review"}
	md := fallbackMarkdown(preset, nil, "", "")
	want := "## kaijutsu doc-review (synthesis failed; raw findings below)"
	if !strings.HasPrefix(md, want) {
		t.Errorf("expected prefix %q, got %q", want, md[:min(len(md), len(want))])
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
