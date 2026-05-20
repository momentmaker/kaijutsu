package findings

import (
	"testing"

	"github.com/momentmaker/kaijutsu/cli/internal/swarm"
)

// TestRecordRun_WritesLensAndPosition covers the v0.9 recorder
// changes: lens column populated from [lens:<name>] summary prefix
// for dream rows; position is monotonic 0..N-1 across the run.
func TestRecordRun_WritesLensAndPosition(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()

	results := []swarm.AgentResult{
		{
			Agent: "default-claude",
			Findings: []swarm.Finding{
				{Severity: "info", File: "x.go", LineRange: "1", Summary: "[lens:honest] take 1", Reasoning: "load_bearing: false"},
				{Severity: "info", File: "x.go", LineRange: "2", Summary: "[lens:gaps] take 2", Reasoning: "load_bearing: true"},
			},
		},
		{
			Agent: "default-antigravity",
			Findings: []swarm.Finding{
				{Severity: "info", File: "y.go", LineRange: "3", Summary: "[lens:wild] take 3", Reasoning: "load_bearing: false"},
			},
		},
	}
	meta := RunMeta{
		RunID:      "r-dream",
		CodebaseFP: "fp-dream",
		Preset:     "dream",
		ProviderForPersona: map[string]string{
			"default-claude": "claude",
			"default-antigravity": "antigravity",
		},
	}

	n, skipped, err := RecordRun(store, meta, results)
	if err != nil {
		t.Fatalf("RecordRun: %v", err)
	}
	if n != 3 {
		t.Fatalf("written=%d, want 3", n)
	}
	if skipped != 0 {
		t.Fatalf("skipped=%d, want 0 (all rows valid)", skipped)
	}

	// Verify lens populated per row.
	expected := map[string]string{
		"[lens:honest] take 1": "honest",
		"[lens:gaps] take 2":   "gaps",
		"[lens:wild] take 3":   "wild",
	}
	for summary, wantLens := range expected {
		var gotLens string
		if err := store.DB().QueryRow(
			`SELECT lens FROM findings WHERE summary = ?`, summary,
		).Scan(&gotLens); err != nil {
			t.Fatalf("lens lookup %q: %v", summary, err)
		}
		if gotLens != wantLens {
			t.Errorf("summary %q: lens=%q, want %q", summary, gotLens, wantLens)
		}
	}

	// Verify position monotonic 0..N-1 across the whole run, regardless
	// of agent. position = 0 for first inserted row, 2 for last.
	var positions []int
	rows, err := store.DB().Query(
		`SELECT position FROM findings WHERE run_id = ? ORDER BY position`, "r-dream",
	)
	if err != nil {
		t.Fatalf("position query: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var p int
		if err := rows.Scan(&p); err != nil {
			t.Fatalf("scan position: %v", err)
		}
		positions = append(positions, p)
	}
	if len(positions) != 3 {
		t.Fatalf("got %d positions, want 3", len(positions))
	}
	for i, p := range positions {
		if p != i {
			t.Errorf("positions[%d] = %d, want %d (monotonic)", i, p, i)
		}
	}
}

// TestRecordRun_NonDreamHasNullLens verifies non-dream presets leave
// the lens column NULL — lens is dream-only by spec.
func TestRecordRun_NonDreamHasNullLens(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()

	results := []swarm.AgentResult{
		{Agent: "claude", Findings: []swarm.Finding{
			{Severity: "issue", File: "x.go", LineRange: "1", Summary: "regular pr-review finding"},
		}},
	}
	meta := RunMeta{RunID: "r1", CodebaseFP: "fp1", Preset: "pr-review"}

	if _, _, err := RecordRun(store, meta, results); err != nil {
		t.Fatalf("RecordRun: %v", err)
	}

	var lens any
	if err := store.DB().QueryRow(`SELECT lens FROM findings`).Scan(&lens); err != nil {
		t.Fatalf("lens lookup: %v", err)
	}
	if lens != nil {
		t.Errorf("non-dream row: lens=%v, want NULL", lens)
	}
}

