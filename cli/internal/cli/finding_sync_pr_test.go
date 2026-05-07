package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/momentmaker/kaijutsu/cli/internal/findings"
	"github.com/momentmaker/kaijutsu/cli/internal/swarm"
)

// TestApplySyncPRActions_DryRunDoesNotWrite seeds a finding, runs
// applySyncPRActions in dry-run mode, asserts the row's action stays
// unchanged.
func TestApplySyncPRActions_DryRunDoesNotWrite(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "test.db")
	store := openSyncPRTestStore(t, dbPath)
	defer store.Close()

	runID := "run-1"
	seedSyncPRFinding(t, store, runID, 0)

	actions := []swarm.SyncPRAction{
		{Verb: "accepted", RunID: runID, Position: 0, Raw: "accept: run-1:0"},
	}
	out := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	if err := applySyncPRActions(out, stderr, store, actions, false); err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	if !strings.Contains(out.String(), "[dry-run]") {
		t.Errorf("expected dry-run banner; got:\n%s", out.String())
	}
	// Row must still have NULL user_action.
	var ua any
	if err := store.DB().QueryRow(`SELECT user_action FROM findings WHERE run_id=? AND position=?`, runID, 0).Scan(&ua); err != nil {
		t.Fatalf("query: %v", err)
	}
	if ua != nil {
		t.Errorf("dry-run wrote action; got %v, want NULL", ua)
	}
}

// TestApplySyncPRActions_ApplyWrites verifies the write path: --apply
// records the action via SetAction.
func TestApplySyncPRActions_ApplyWrites(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "test.db")
	store := openSyncPRTestStore(t, dbPath)
	defer store.Close()

	runID := "run-2"
	seedSyncPRFinding(t, store, runID, 0)

	actions := []swarm.SyncPRAction{
		{Verb: "dismissed", RunID: runID, Position: 0, Raw: "dismiss: run-2:0"},
	}
	out := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	if err := applySyncPRActions(out, stderr, store, actions, true); err != nil {
		t.Fatalf("apply: %v", err)
	}
	var ua string
	if err := store.DB().QueryRow(`SELECT user_action FROM findings WHERE run_id=? AND position=?`, runID, 0).Scan(&ua); err != nil {
		t.Fatalf("query: %v", err)
	}
	if ua != "dismissed" {
		t.Errorf("got user_action=%q, want dismissed", ua)
	}
}

// TestApplySyncPRActions_IdempotentNoop: re-applying the same action
// is a noop (logs "noop" but doesn't error or double-write).
func TestApplySyncPRActions_IdempotentNoop(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "test.db")
	store := openSyncPRTestStore(t, dbPath)
	defer store.Close()

	runID := "run-3"
	seedSyncPRFinding(t, store, runID, 0)

	// First apply.
	actions := []swarm.SyncPRAction{
		{Verb: "accepted", RunID: runID, Position: 0, Raw: "accept: run-3:0"},
	}
	out := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	_ = applySyncPRActions(out, stderr, store, actions, true)

	// Second apply (same action) → noop logged.
	out.Reset()
	if err := applySyncPRActions(out, stderr, store, actions, true); err != nil {
		t.Fatalf("re-apply: %v", err)
	}
	if !strings.Contains(out.String(), "noop") {
		t.Errorf("expected 'noop' in output; got:\n%s", out.String())
	}
}

// TestApplySyncPRActions_ConflictLogsChanged verifies the
// accept→dismiss conflict path: prior action overridden, '[changed]'
// log line appears.
func TestApplySyncPRActions_ConflictLogsChanged(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "test.db")
	store := openSyncPRTestStore(t, dbPath)
	defer store.Close()

	runID := "run-4"
	seedSyncPRFinding(t, store, runID, 0)

	// First apply: accepted.
	out := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	_ = applySyncPRActions(out, stderr, store, []swarm.SyncPRAction{
		{Verb: "accepted", RunID: runID, Position: 0, Raw: "accept"},
	}, true)
	// Second apply: dismissed (conflict).
	out.Reset()
	if err := applySyncPRActions(out, stderr, store, []swarm.SyncPRAction{
		{Verb: "dismissed", RunID: runID, Position: 0, Raw: "dismiss"},
	}, true); err != nil {
		t.Fatalf("conflict apply: %v", err)
	}
	if !strings.Contains(out.String(), "[changed]") {
		t.Errorf("expected '[changed]' log; got:\n%s", out.String())
	}
}

// TestApplySyncPRActions_SkipsUnknownRunPosition: action references a
// (run_id, position) pair that doesn't exist → log skip + count, no
// error.
func TestApplySyncPRActions_SkipsUnknownRunPosition(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "test.db")
	store := openSyncPRTestStore(t, dbPath)
	defer store.Close()

	out := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	if err := applySyncPRActions(out, stderr, store, []swarm.SyncPRAction{
		{Verb: "accepted", RunID: "no-such-run", Position: 0, Raw: "accept"},
	}, true); err != nil {
		t.Fatalf("err on unknown row: %v", err)
	}
	if !strings.Contains(stderr.String(), "no row for") {
		t.Errorf("expected 'no row for' in stderr; got:\n%s", stderr.String())
	}
}

// --- helpers ---

func openSyncPRTestStore(t *testing.T, path string) *findings.Store {
	t.Helper()
	store, err := findings.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return store
}

func seedSyncPRFinding(t *testing.T, store *findings.Store, runID string, position int) {
	t.Helper()
	_, err := store.DB().Exec(
		`INSERT INTO findings(run_id, codebase_fp, preset, provider, persona,
		                      severity, file, line_range, summary, position)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		runID, "fp1", "pr-review", "claude", "default-claude",
		"info", "x.go", "1", "test finding", position,
	)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
}
