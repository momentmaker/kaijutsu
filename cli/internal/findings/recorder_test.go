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

	n, _, err := RecordRun(store, meta, results)
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

	if _, _, err := RecordRun(store, meta, results); err != nil {
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

	n, _, err := RecordRun(store, meta, results)
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

	n, _, err := RecordRun(store, RunMeta{
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
			if _, _, err := RecordRun(store, tc.meta, nil); err == nil {
				t.Fatalf("RecordRun(%+v): want error, got nil", tc.meta)
			}
		})
	}
}

// TestRecordRun_NilStore — caller passing a nil store gets a clean
// error, not a panic. Stage 2's caller wraps RecordRun in
// log-and-continue, so a nil here would otherwise crash the swarm.
func TestRecordRun_NilStore(t *testing.T) {
	if _, _, err := RecordRun(nil, RunMeta{RunID: "r", CodebaseFP: "f", Preset: "p"}, nil); err == nil {
		t.Fatal("RecordRun(nil): want error, got nil")
	}
}

// TestValidDreamFinding covers the v0.8.3 lens-prefix validator.
// Dream-preset rows MUST conform to the lens-in-summary encoding;
// malformed rows get skipped at recorder time.
func TestValidDreamFinding(t *testing.T) {
	cases := []struct {
		name      string
		summary   string
		reasoning string
		want      bool
	}{
		{"happy path", "[lens:gaps] We aren't asking about X", "load_bearing: true. The user assumed Y.", true},
		{"happy path with status-quo", "[lens:status-quo] Inaction has cost Z", "load_bearing: false", true},
		{"empty reasoning ok", "[lens:wild] Bold idea here", "", true},
		{"missing prefix", "We aren't asking about X", "load_bearing: true", false},
		{"unknown lens", "[lens:premortem] something happened", "load_bearing: true", false},
		{"bad reasoning prefix", "[lens:honest] strong observation", "this is the reasoning without prefix", false},
		{"uppercase reasoning bool", "[lens:honest] x", "load_bearing: TRUE  more text", true},
		{"caps in lens name rejected", "[lens:Honest] x", "load_bearing: true", false},
		{"prefix-only summary", "[lens:gaps]", "load_bearing: true", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ValidDreamFinding(tc.summary, tc.reasoning)
			if got != tc.want {
				t.Errorf("ValidDreamFinding(%q, %q) = %v, want %v", tc.summary, tc.reasoning, got, tc.want)
			}
		})
	}
}

// TestRecordRun_DreamSkipsMalformed verifies dream-preset rows that
// fail validation get skipped (with skipped count returned), while
// valid rows still get inserted. Non-dream presets are unaffected.
func TestRecordRun_DreamSkipsMalformed(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()

	results := []swarm.AgentResult{
		{Agent: "claude", Findings: []swarm.Finding{
			{Severity: "blocker", File: "", LineRange: "", Summary: "[lens:gaps] valid finding", Reasoning: "load_bearing: true reason"},
			{Severity: "issue", File: "", LineRange: "", Summary: "no lens prefix here", Reasoning: "load_bearing: false"},
			{Severity: "issue", File: "", LineRange: "", Summary: "[lens:premortem] unknown lens", Reasoning: "load_bearing: true"},
			{Severity: "info", File: "", LineRange: "", Summary: "[lens:honest] another valid", Reasoning: ""},
		}},
	}
	written, skipped, err := RecordRun(store, RunMeta{
		RunID: "r1", CodebaseFP: "fp1", Preset: "dream",
	}, results)
	if err != nil {
		t.Fatalf("RecordRun: %v", err)
	}
	if written != 2 {
		t.Errorf("written = %d, want 2 (two valid dream findings)", written)
	}
	if skipped != 2 {
		t.Errorf("skipped = %d, want 2 (no-prefix + unknown-lens)", skipped)
	}
}

// TestRecordRun_NonDreamUnaffected verifies the validator only fires
// for the dream preset. pr-review / doc-review etc. record all rows
// regardless of summary shape.
func TestRecordRun_NonDreamUnaffected(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()

	results := []swarm.AgentResult{
		{Agent: "claude", Findings: []swarm.Finding{
			{Severity: "issue", File: "x.go", LineRange: "1", Summary: "no lens prefix needed", Reasoning: "any reasoning"},
		}},
	}
	written, skipped, err := RecordRun(store, RunMeta{
		RunID: "r1", CodebaseFP: "fp1", Preset: "pr-review",
	}, results)
	if err != nil {
		t.Fatalf("RecordRun: %v", err)
	}
	if written != 1 {
		t.Errorf("written = %d, want 1", written)
	}
	if skipped != 0 {
		t.Errorf("non-dream preset should never skip; got skipped = %d", skipped)
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
