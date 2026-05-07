package cli

import (
	"bytes"
	"path/filepath"
	"testing"
	"time"

	"github.com/momentmaker/kaijutsu/cli/internal/findings"
	"github.com/momentmaker/kaijutsu/cli/internal/swarm"
)

// TestResolveDreamLensWeights_NonDreamReturnsNil pins the preset
// gate. Non-dream presets get nil — the lens column is dream-only.
func TestResolveDreamLensWeights_NonDreamReturnsNil(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("KAIJUTSU_FINDINGS_DB", filepath.Join(tmp, "test.db"))

	results := []swarm.AgentResult{
		{Agent: "claude", Findings: []swarm.Finding{
			{Summary: "[lens:honest] take", File: "x.go", LineRange: "1"},
		}},
	}
	got := resolveDreamLensWeights(&bytes.Buffer{}, "", "pr-review", results, nil)
	if got != nil {
		t.Errorf("non-dream preset → want nil, got %v", got)
	}
}

// TestResolveDreamLensWeights_KillswitchHonored covers the env-var
// gate. Even on dream preset with valid findings, killswitch=off
// suppresses the weights map.
func TestResolveDreamLensWeights_KillswitchHonored(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("KAIJUTSU_FINDINGS_DB", filepath.Join(tmp, "test.db"))
	t.Setenv("KAIJUTSU_DREAM_ADAPTIVE_LENS", "off")

	results := []swarm.AgentResult{
		{Agent: "default-claude", Findings: []swarm.Finding{
			{Summary: "[lens:honest] take", File: "x.go", LineRange: "1"},
		}},
	}
	got := resolveDreamLensWeights(&bytes.Buffer{}, "", "dream", results, nil)
	if got != nil {
		t.Errorf("killswitch=off → want nil, got %v", got)
	}
}

// TestResolveDreamLensWeights_AggregatesMatureWindow seeds the
// weighter with mature samples for two (persona, lens) pairs, runs
// the resolver, asserts averaged per-lens weights surface.
func TestResolveDreamLensWeights_AggregatesMatureWindow(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "test.db")
	t.Setenv("KAIJUTSU_FINDINGS_DB", dbPath)
	t.Setenv("KAIJUTSU_DREAM_ADAPTIVE_LENS", "")

	store, err := findings.Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	// resolveDreamLensWeights uses findings.Fingerprint on cwd.
	// Compute the same fp here so the seeded rows match.
	fp := findings.Fingerprint(tmp)

	// Seed 10 accepted on (claude, default-claude, dream, fp, honest)
	// → mature, precision 1.0 → clamped to PrecisionCeiling.
	tx, _ := store.DB().Begin()
	stmt, _ := tx.Prepare(`INSERT INTO findings(
		run_id, codebase_fp, preset, provider, persona,
		severity, file, line_range, summary, lens,
		user_action, action_at
	) VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`)
	now := time.Now()
	for i := 0; i < 10; i++ {
		stmt.Exec("seed-r", fp, "dream", "claude", "default-claude",
			"info", "x.go", "1", "[lens:honest] s", "honest", "accepted", now)
	}
	stmt.Close()
	tx.Commit()
	store.Close()

	// Now run resolver with a result that produced honest-lens
	// findings. Use projectRoot=tmp so findings.Fingerprint(cwd)
	// matches the seeded fp.
	results := []swarm.AgentResult{
		{Agent: "default-claude", Findings: []swarm.Finding{
			{Summary: "[lens:honest] new take", File: "x.go", LineRange: "1"},
		}},
	}
	got := resolveDreamLensWeights(&bytes.Buffer{}, tmp, "dream", results, nil)
	if got == nil {
		t.Fatal("expected non-nil lens weights map")
	}
	w, ok := got["honest"]
	if !ok {
		t.Errorf("expected honest lens weight; got %v", got)
	}
	// All 10 seeds were accepted → precision 1.0 → ceiling.
	if w != findings.PrecisionCeiling {
		t.Errorf("honest weight = %v, want %v (PrecisionCeiling)", w, findings.PrecisionCeiling)
	}
}

// TestResolveDreamLensWeights_EmptyDBReturnsNil verifies the
// cold-start path: no DB file exists → nil → buildSynthPrompt skips
// the lens-weights block (v0.8.3-byte-identical contract).
func TestResolveDreamLensWeights_EmptyDBReturnsNil(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("KAIJUTSU_FINDINGS_DB", filepath.Join(tmp, "no-such-db.db"))
	t.Setenv("KAIJUTSU_DREAM_ADAPTIVE_LENS", "")

	results := []swarm.AgentResult{
		{Agent: "default-claude", Findings: []swarm.Finding{
			{Summary: "[lens:honest] take", File: "x.go", LineRange: "1"},
		}},
	}
	got := resolveDreamLensWeights(&bytes.Buffer{}, "", "dream", results, nil)
	if got != nil {
		t.Errorf("missing DB → want nil, got %v", got)
	}
}
