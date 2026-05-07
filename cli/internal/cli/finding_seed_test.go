package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/momentmaker/kaijutsu/cli/internal/findings"
)

// TestFindingSeed_DevOnlyFlagGated covers the --dev flag gate. Without
// the flag (and without the env-var), the subcommand returns the
// "unknown command" error matching cobra's standard wording.
func TestFindingSeed_DevOnlyFlagGated(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("KAIJUTSU_FINDINGS_DB", filepath.Join(tmp, "test.db"))
	t.Setenv("KAIJUTSU_FINDING_SEED", "")

	out, _, err := runCmdCapture(t, "finding", "seed",
		"--provider", "claude", "--persona", "p", "--preset", "dream",
		"--codebase", "fp1", "--accepts", "1")
	if err == nil {
		t.Fatalf("expected gated rejection, got success: %s", out)
	}
	if !strings.Contains(err.Error(), "unknown command") {
		t.Errorf("error %q should contain 'unknown command' (cobra-style)", err.Error())
	}
}

// TestFindingSeed_DevOnlyEnvVarGated covers the env-var gate. With
// KAIJUTSU_FINDING_SEED=1, the subcommand runs without the --dev flag.
func TestFindingSeed_DevOnlyEnvVarGated(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("KAIJUTSU_FINDINGS_DB", filepath.Join(tmp, "test.db"))
	t.Setenv("KAIJUTSU_FINDING_SEED", "1")

	out, _, err := runCmdCapture(t, "finding", "seed",
		"--provider", "claude", "--persona", "p", "--preset", "dream",
		"--codebase", "fp1", "--accepts", "3", "--dismisses", "2")
	if err != nil {
		t.Fatalf("seed with env-var gate: %v", err)
	}
	if !strings.Contains(out, "seeded 5 row(s)") {
		t.Errorf("output should report 5 rows; got %q", out)
	}
}

// TestFindingSeed_DevFlagPathRuns covers the --dev flag path.
func TestFindingSeed_DevFlagPathRuns(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("KAIJUTSU_FINDINGS_DB", filepath.Join(tmp, "test.db"))
	t.Setenv("KAIJUTSU_FINDING_SEED", "")

	out, _, err := runCmdCapture(t, "finding", "--dev", "seed",
		"--provider", "claude", "--persona", "p", "--preset", "dream",
		"--codebase", "fp1", "--accepts", "2")
	if err != nil {
		t.Fatalf("seed with --dev flag: %v", err)
	}
	if !strings.Contains(out, "seeded 2 row(s)") {
		t.Errorf("output should report 2 rows; got %q", out)
	}
}

// TestFindingSeed_RejectsWhenNeitherSet covers the production-CLI
// rejection path: no flag, no env-var, exits with the gated error.
func TestFindingSeed_RejectsWhenNeitherSet(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("KAIJUTSU_FINDINGS_DB", filepath.Join(tmp, "test.db"))
	t.Setenv("KAIJUTSU_FINDING_SEED", "")

	_, _, err := runCmdCapture(t, "finding", "seed",
		"--provider", "claude", "--persona", "p", "--preset", "dream",
		"--codebase", "fp1", "--accepts", "1")
	if err == nil {
		t.Fatal("expected gated rejection, got success")
	}
}

// TestFindingSeed_InsertsActionedRows verifies the seed actually
// writes rows that the weighter will see (real DB writes, not no-ops).
// Round-trips: seed a tuple, then query directly to confirm action
// counts AND that the lens column is populated when --lens is passed.
func TestFindingSeed_InsertsActionedRows(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "test.db")
	t.Setenv("KAIJUTSU_FINDINGS_DB", dbPath)
	t.Setenv("KAIJUTSU_FINDING_SEED", "1")

	if _, _, err := runCmdCapture(t, "finding", "seed",
		"--provider", "claude", "--persona", "honest-persona",
		"--preset", "dream", "--codebase", "fp-test",
		"--lens", "honest",
		"--accepts", "9", "--dismisses", "1"); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// Action-count check via `finding list` output.
	out, _, err := runCmdCapture(t, "finding", "list", "--all-codebases")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	accepts := strings.Count(out, "accepted")
	dismisses := strings.Count(out, "dismissed")
	if accepts != 9 {
		t.Errorf("accepted count = %d, want 9", accepts)
	}
	if dismisses != 1 {
		t.Errorf("dismissed count = %d, want 1", dismisses)
	}

	// Direct DB check for the lens column — `finding list` doesn't
	// surface lens, so verify via the underlying store.
	store, err := findings.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	var lensCount int
	if err := store.DB().QueryRow(
		`SELECT COUNT(*) FROM findings WHERE lens = 'honest'`,
	).Scan(&lensCount); err != nil {
		t.Fatalf("lens count: %v", err)
	}
	if lensCount != 10 {
		t.Errorf("rows with lens='honest' = %d, want 10", lensCount)
	}
}
