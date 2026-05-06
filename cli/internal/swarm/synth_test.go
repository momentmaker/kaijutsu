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
	clusters := clusterFindings(results, nil)
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
	clusters := clusterFindings(results, nil)
	table := renderDisagreementTable(results, clusters, nil, false)
	// claude column header must appear before gemini (canonical order).
	cIdx := strings.Index(table, "claude")
	gIdx := strings.Index(table, "gemini")
	if cIdx < 0 || gIdx < 0 || cIdx > gIdx {
		t.Fatalf("expected claude before gemini, got table:\n%s", table)
	}
}

func TestClusterFindings_DropsMCPInfoSeverity(t *testing.T) {
	results := []AgentResult{
		{Agent: "claude", Driver: "cli", Findings: []Finding{
			{Severity: SeverityInfo, File: "a.go", LineRange: "10", Summary: "llm-info kept"},
		}},
		{Agent: "semgrep-mcp", Driver: "mcp", Findings: []Finding{
			{Severity: SeverityInfo, File: "b.go", LineRange: "20", Summary: "mcp-info dropped"},
			{Severity: SeverityIssue, File: "c.go", LineRange: "30", Summary: "mcp-issue kept"},
		}},
	}
	clusters := clusterFindings(results, nil)
	for _, g := range clusters {
		if g.Key == "b.go:20" {
			t.Errorf("MCP info-tier finding should have been dropped, but cluster present: %+v", g)
		}
	}
	// LLM info kept + MCP issue kept = 2 clusters.
	if len(clusters) != 2 {
		t.Errorf("expected 2 clusters (llm-info kept + mcp-issue kept), got %d", len(clusters))
	}
}

func TestRenderDisagreementTable_TagsMCPColumns(t *testing.T) {
	results := []AgentResult{
		{Agent: "claude", Driver: "cli", Findings: []Finding{
			{Severity: SeverityIssue, File: "a", LineRange: "1", Summary: "s"},
		}},
		{Agent: "semgrep-mcp", Driver: "mcp", Findings: []Finding{
			{Severity: SeverityIssue, File: "a", LineRange: "1", Summary: "s"},
		}},
	}
	clusters := clusterFindings(results, nil)
	table := renderDisagreementTable(results, clusters, nil, false)
	if !strings.Contains(table, "semgrep-mcp [deterministic]") {
		t.Errorf("expected '[deterministic]' tag on MCP column header; got:\n%s", table)
	}
	if !strings.Contains(table, "✓⚙") {
		t.Errorf("expected gear-marked check ✓⚙ for MCP cell; got:\n%s", table)
	}
	// LLM column should NOT carry the gear marker or tag.
	if strings.Contains(table, "claude [deterministic]") {
		t.Errorf("cli column should not carry [deterministic] tag; got:\n%s", table)
	}
}

func TestRenderDisagreementTable_EmptyOnNoClusters(t *testing.T) {
	if got := renderDisagreementTable(nil, nil, nil, false); got != "" {
		t.Fatalf("want empty, got %q", got)
	}
}

// TestClusterFindings_WeightedSortPromotesHighWeight covers the
// v0.7 secondary-sort change: a single-reporter, lower-severity
// finding from a high-weight agent should outrank a multi-reporter
// cluster of low-weight agents at same severity. Cold-start path
// (all weights = 1.0) collapses back to v0.6 ordering — see
// TestClusterFindings_ConsensusAndOrder above for that contract.
func TestClusterFindings_WeightedSortPromotesHighWeight(t *testing.T) {
	results := []AgentResult{
		// gemini high-weight: lone finding at issue severity.
		{Agent: "gemini", Findings: []Finding{
			{Severity: SeverityIssue, File: "z.go", LineRange: "1", Summary: "lone trustworthy"},
		}},
		// claude+codex low-weight chorus at same severity, different file.
		{Agent: "claude", Findings: []Finding{
			{Severity: SeverityIssue, File: "a.go", LineRange: "1", Summary: "chorus"},
		}},
		{Agent: "codex", Findings: []Finding{
			{Severity: SeverityIssue, File: "a.go", LineRange: "1", Summary: "chorus"},
		}},
	}
	weights := map[string]float64{
		"gemini": 1.0, // perfect record
		"claude": 0.10,
		"codex":  0.10,
	}
	clusters := clusterFindings(results, weights)
	if len(clusters) != 2 {
		t.Fatalf("want 2 clusters, got %d", len(clusters))
	}
	// gemini's weighted_consensus = 1.0; claude+codex chorus = 0.20.
	// So gemini's cluster should sort FIRST despite ConsensusOf=1
	// vs the chorus's ConsensusOf=2.
	if clusters[0].Key != "z.go:1" {
		t.Errorf("expected gemini's z.go:1 first under weighted sort, got %s",
			clusters[0].Key)
	}
}

// TestClusterFindings_ColdStartMatchesV06 verifies the byte-identical
// contract: when weights are nil OR all 1.0, ordering exactly matches
// the v0.6 severity → ConsensusOf → key sort. This is the spec's
// backward-compat guarantee.
func TestClusterFindings_ColdStartMatchesV06(t *testing.T) {
	results := []AgentResult{
		{Agent: "claude", Findings: []Finding{
			{Severity: SeverityIssue, File: "z.go", LineRange: "1", Summary: "lone"},
		}},
		{Agent: "codex", Findings: []Finding{
			{Severity: SeverityIssue, File: "a.go", LineRange: "1", Summary: "chorus"},
		}},
		{Agent: "gemini", Findings: []Finding{
			{Severity: SeverityIssue, File: "a.go", LineRange: "1", Summary: "chorus"},
		}},
	}
	// nil weights and all-cold weights MUST produce same ordering.
	nilSorted := clusterFindings(results, nil)
	coldSorted := clusterFindings(results, map[string]float64{"claude": 1.0, "codex": 1.0, "gemini": 1.0})
	if nilSorted[0].Key != coldSorted[0].Key {
		t.Errorf("cold-start divergence: nil=%s, all-cold=%s",
			nilSorted[0].Key, coldSorted[0].Key)
	}
	// And both must put a.go:1 first (consensus=2 wins under v0.6 rules).
	if nilSorted[0].Key != "a.go:1" {
		t.Errorf("expected v0.6 ordering (consensus first), got %s", nilSorted[0].Key)
	}
}

// TestRenderDisagreementTable_ShowWeightsAnnotates verifies the
// --show-weights flag adds "(0.85)" style annotations to column
// headers; off by default.
func TestRenderDisagreementTable_ShowWeightsAnnotates(t *testing.T) {
	results := []AgentResult{
		{Agent: "claude", Findings: []Finding{{Severity: SeverityIssue, File: "x", LineRange: "1", Summary: "s"}}},
	}
	clusters := clusterFindings(results, nil)
	weights := map[string]float64{"claude": 0.85}

	// off (default)
	off := renderDisagreementTable(results, clusters, weights, false)
	if strings.Contains(off, "0.85") {
		t.Errorf("show-weights off should NOT annotate; got: %s", off)
	}
	// on
	on := renderDisagreementTable(results, clusters, weights, true)
	if !strings.Contains(on, "claude (0.85)") {
		t.Errorf("show-weights on should annotate column header; got: %s", on)
	}
}

// TestBuildSynthPrompt_ColdStartByteIdentical confirms the prompt
// has zero weights-section bytes when every weight is cold (1.0).
func TestBuildSynthPrompt_ColdStartByteIdentical(t *testing.T) {
	template := "review the following:\n%s"
	body := []byte(`{"agents":[]}`)

	bare := buildSynthPrompt(template, body, nil)
	allCold := buildSynthPrompt(template, body, map[string]float64{"claude": 1.0})

	if bare != allCold {
		t.Errorf("nil vs all-cold weights produced different prompts:\nnil:\n%s\nall-cold:\n%s",
			bare, allCold)
	}
	if strings.Contains(bare, "weights ") {
		t.Errorf("cold-start prompt leaked weights section:\n%s", bare)
	}
}

// TestBuildSynthPrompt_NonColdEmitsWeightsSection covers the warm
// path: any non-cold weight triggers the weights: header.
func TestBuildSynthPrompt_NonColdEmitsWeightsSection(t *testing.T) {
	template := "core:\n%s"
	body := []byte(`x`)
	prompt := buildSynthPrompt(template, body, map[string]float64{"claude": 0.85})
	if !strings.Contains(prompt, "weights ") {
		t.Errorf("warm path should emit weights section; got:\n%s", prompt)
	}
	if !strings.Contains(prompt, "claude: 0.85") {
		t.Errorf("warm path should include per-agent line; got:\n%s", prompt)
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
