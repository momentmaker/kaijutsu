package findings

import (
	"path/filepath"
	"testing"

	"github.com/momentmaker/kaijutsu/cli/internal/swarm"
)

// TestRecordRun_HappyPath inserts findings from two agents in one
// call, verifies row count + provider mapping + nullable handling.
func TestRecordRun_HappyPath(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()

	results := []swarm.AgentResult{
		{
			Agent: "paranoid-security-claude",
			Findings: []swarm.Finding{
				{Severity: "blocker", File: "auth.go", LineRange: "10-20", Summary: "auth bypass", Reasoning: "missing check", Confidence: 0.9},
				{Severity: "minor", File: "auth.go", LineRange: "30", Summary: "naming nit"},
			},
		},
		{
			Agent: "default-gemini",
			Findings: []swarm.Finding{
				{Severity: "issue", File: "main.go", LineRange: "5", Summary: "race"},
			},
		},
	}
	meta := RunMeta{
		RunID:      "run-abc",
		CodebaseFP: "fp123456789abcdef",
		Preset:     "pr-review",
		ProviderForPersona: map[string]string{
			"paranoid-security-claude": "claude",
			"default-gemini":           "gemini",
		},
	}

	n, err := RecordRun(store, meta, results)
	if err != nil {
		t.Fatalf("RecordRun: %v", err)
	}
	if n != 3 {
		t.Fatalf("written=%d, want 3", n)
	}

	got, err := CountForRun(store, "run-abc")
	if err != nil {
		t.Fatalf("CountForRun: %v", err)
	}
	if got != 3 {
		t.Fatalf("count=%d, want 3", got)
	}

	// Provider mapping populated correctly.
	var provider string
	if err := store.DB().QueryRow(
		`SELECT provider FROM findings WHERE persona='paranoid-security-claude' LIMIT 1`,
	).Scan(&provider); err != nil {
		t.Fatalf("provider lookup: %v", err)
	}
	if provider != "claude" {
		t.Fatalf("provider=%q, want claude", provider)
	}

	// Nullable fields: reasoning omitted on row 2 → NULL.
	var reasoning, confidence any
	if err := store.DB().QueryRow(
		`SELECT reasoning, confidence FROM findings WHERE summary='naming nit'`,
	).Scan(&reasoning, &confidence); err != nil {
		t.Fatalf("nullable lookup: %v", err)
	}
	if reasoning != nil {
		t.Fatalf("reasoning=%v, want nil", reasoning)
	}
	if confidence != nil {
		t.Fatalf("confidence=%v, want nil", confidence)
	}

	// user_action defaults to NULL on insert (pending state).
	var userAction any
	if err := store.DB().QueryRow(
		`SELECT user_action FROM findings WHERE summary='auth bypass'`,
	).Scan(&userAction); err != nil {
		t.Fatalf("user_action lookup: %v", err)
	}
	if userAction != nil {
		t.Fatalf("user_action=%v, want nil (pending)", userAction)
	}
}

// TestRecordRun_LegacyModeProviderFallback covers the legacy v0.5
// path: ProviderForPersona is nil, Agent name == provider name.
func TestRecordRun_LegacyModeProviderFallback(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()

	results := []swarm.AgentResult{
		{Agent: "claude", Findings: []swarm.Finding{
			{Severity: "info", File: "x.go", LineRange: "1", Summary: "ok"},
		}},
	}
	meta := RunMeta{RunID: "r1", CodebaseFP: "fp1", Preset: "pr-review"}

	if _, err := RecordRun(store, meta, results); err != nil {
		t.Fatalf("RecordRun: %v", err)
	}

	var provider string
	if err := store.DB().QueryRow(`SELECT provider FROM findings`).Scan(&provider); err != nil {
		t.Fatalf("provider lookup: %v", err)
	}
	if provider != "claude" {
		t.Fatalf("provider=%q, want claude (fallback to agent name)", provider)
	}
}

// TestRecordRun_SkipsErrors verifies errored AgentResults don't
// produce rows. An errored agent has nothing useful to record — the
// error is surfaced elsewhere (stderr summary), not the findings DB.
func TestRecordRun_SkipsErrors(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()

	results := []swarm.AgentResult{
		{Agent: "claude", Err: "rate limit"},
		{Agent: "gemini", Findings: []swarm.Finding{
			{Severity: "info", File: "x.go", LineRange: "1", Summary: "ok"},
		}},
	}
	meta := RunMeta{RunID: "r1", CodebaseFP: "fp1", Preset: "pr-review"}

	n, err := RecordRun(store, meta, results)
	if err != nil {
		t.Fatalf("RecordRun: %v", err)
	}
	if n != 1 {
		t.Fatalf("written=%d, want 1 (errored agent skipped)", n)
	}
}

// TestRecordRun_EmptyResults is a no-op that still commits cleanly —
// callers don't need to pre-check whether any agent produced findings.
func TestRecordRun_EmptyResults(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()

	n, err := RecordRun(store, RunMeta{
		RunID: "r1", CodebaseFP: "fp1", Preset: "pr-review",
	}, nil)
	if err != nil {
		t.Fatalf("RecordRun: %v", err)
	}
	if n != 0 {
		t.Fatalf("written=%d, want 0", n)
	}
}

// TestRecordRun_RequiresMeta validates the precondition checks. A
// missing RunID etc. would silently produce un-scopable rows; better
// to fail loudly so the swarm wrapper logs a clear stderr warning.
func TestRecordRun_RequiresMeta(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()

	cases := []struct {
		name string
		meta RunMeta
	}{
		{"missing RunID", RunMeta{CodebaseFP: "fp1", Preset: "p"}},
		{"missing CodebaseFP", RunMeta{RunID: "r1", Preset: "p"}},
		{"missing Preset", RunMeta{RunID: "r1", CodebaseFP: "fp1"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := RecordRun(store, tc.meta, nil); err == nil {
				t.Fatalf("RecordRun(%+v): want error, got nil", tc.meta)
			}
		})
	}
}

// TestRecordRun_NilStore — caller passing a nil store gets a clean
// error, not a panic. Stage 2's caller wraps RecordRun in
// log-and-continue, so a nil here would otherwise crash the swarm.
func TestRecordRun_NilStore(t *testing.T) {
	if _, err := RecordRun(nil, RunMeta{RunID: "r", CodebaseFP: "f", Preset: "p"}, nil); err == nil {
		t.Fatal("RecordRun(nil): want error, got nil")
	}
}

// openTestStore creates a fresh DB under t.TempDir and returns the
// open Store. Caller defers Close.
func openTestStore(t *testing.T) *Store {
	t.Helper()
	tmp := t.TempDir()
	store, err := Open(filepath.Join(tmp, "findings.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return store
}
