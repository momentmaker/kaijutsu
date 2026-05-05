package swarm

import "testing"

func TestMergePasses_PreferP2WhenAvailable(t *testing.T) {
	p1 := []AgentResult{
		{Agent: "claude", Findings: []Finding{{Severity: SeverityIssue, File: "a", LineRange: "1", Summary: "old"}}},
		{Agent: "codex", Findings: []Finding{{Severity: SeverityMinor, File: "b", LineRange: "2", Summary: "old"}}},
	}
	p2 := []AgentResult{
		{Agent: "claude", Findings: []Finding{{Severity: SeverityBlocker, File: "a", LineRange: "1", Summary: "revised"}}},
		{Agent: "codex", Err: "timeout"},
	}
	merged := mergePasses(p1, p2)
	if len(merged) != 2 {
		t.Fatalf("want 2 results, got %d", len(merged))
	}
	if merged[0].Findings[0].Summary != "revised" {
		t.Errorf("expected p2 to overwrite p1 for claude, got %q", merged[0].Findings[0].Summary)
	}
	if merged[1].Findings[0].Summary != "old" {
		t.Errorf("expected p1 to remain for codex (p2 errored), got %q", merged[1].Findings[0].Summary)
	}
}

func TestMergePasses_EmptyP2KeepsP1(t *testing.T) {
	p1 := []AgentResult{
		{Agent: "claude", Findings: []Finding{{Summary: "x"}}},
	}
	merged := mergePasses(p1, nil)
	if len(merged) != 1 || merged[0].Findings[0].Summary != "x" {
		t.Fatalf("merged corrupted on empty p2: %+v", merged)
	}
}

func TestPeerFindings_ExcludesSelfAndErrored(t *testing.T) {
	all := []AgentResult{
		{Agent: "claude", Findings: []Finding{{Summary: "c"}}},
		{Agent: "codex", Err: "auth-failed"},
		{Agent: "gemini", Findings: []Finding{{Summary: "g"}}},
	}
	peers := peerFindings(all, "claude")
	if len(peers) != 1 {
		t.Fatalf("want 1 peer (gemini only), got %d", len(peers))
	}
	if peers[0]["agent"] != "gemini" {
		t.Errorf("expected gemini, got %v", peers[0]["agent"])
	}
}
