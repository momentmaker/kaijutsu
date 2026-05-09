package cli

import (
	"bytes"
	"encoding/json"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/momentmaker/kaijutsu/cli/internal/findings"
	"github.com/momentmaker/kaijutsu/cli/internal/swarm"
)

// TestFindingFile_NoNetworkImports is the privacy enforcement gate
// from the v0.7 spec: finding.go MUST NOT import any networking
// package. This test parses the file's actual import list and fails
// if any net/* (or bare net, or golang.org/x/net/*) import slips in.
// Catches both intentional telemetry adds and accidental drag-ins
// from a "convenient" helper package.
func TestFindingFile_NoNetworkImports(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "finding.go", nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parse finding.go: %v", err)
	}
	for _, imp := range file.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		if isNetworkPkg(path) {
			t.Errorf("finding.go imports %q — privacy boundary violated; v0.7 quality fingerprinting must be local-only",
				path)
		}
	}
}

// isNetworkPkg flags any package that opens a socket, makes RPC
// calls, or otherwise reaches the network. Conservative — better to
// nudge a contributor to justify a stdlib `net/textproto` (text-only,
// arguably safe) than to silently miss a `net/http` slip-in.
func isNetworkPkg(path string) bool {
	if path == "net" {
		return true
	}
	if strings.HasPrefix(path, "net/") {
		return true
	}
	if strings.HasPrefix(path, "golang.org/x/net/") {
		return true
	}
	return false
}

// TestFindingList_HappyPath records two runs and verifies the list
// scopes to the most recent run when no --run flag is passed.
func TestFindingList_HappyPath(t *testing.T) {
	store, _ := setupFindingTest(t)

	// Record two runs against the same codebase.
	mustRecord(t, store, "run-old", "fp1", "pr-review", oneFinding("claude", "old summary"))
	time.Sleep(10 * time.Millisecond) // ensure created_at ordering
	mustRecord(t, store, "run-new", "fp1", "pr-review", oneFinding("claude", "new summary"))
	store.Close()

	out := runCmd(t, "finding", "list", "--codebase", "fp1")
	if !strings.Contains(out, "new summary") {
		t.Errorf("list missing new run summary; got:\n%s", out)
	}
	if strings.Contains(out, "old summary") {
		t.Errorf("list should scope to most-recent run; got old summary in output:\n%s", out)
	}
	if !strings.Contains(out, "current codebase fp: fp1") {
		t.Errorf("list missing fp header; got:\n%s", out)
	}
}

// TestFindingList_PendingFiltersActioned ensures --pending excludes
// already-actioned rows.
func TestFindingList_PendingFiltersActioned(t *testing.T) {
	store, _ := setupFindingTest(t)
	mustRecord(t, store, "r1", "fp1", "pr-review", []swarm.AgentResult{
		{Agent: "claude", Findings: []swarm.Finding{
			{Severity: "issue", File: "a.go", LineRange: "1", Summary: "pending one"},
			{Severity: "issue", File: "b.go", LineRange: "2", Summary: "will accept"},
		}},
	})
	// Accept id=2 (the second insert).
	if err := findings.SetAction(store, 2, "accepted", ""); err != nil {
		t.Fatalf("SetAction: %v", err)
	}
	store.Close()

	out := runCmd(t, "finding", "list", "--codebase", "fp1", "--pending")
	if !strings.Contains(out, "pending one") {
		t.Errorf("--pending dropped a real pending row:\n%s", out)
	}
	if strings.Contains(out, "will accept") {
		t.Errorf("--pending leaked actioned row:\n%s", out)
	}
}

// TestFindingAccept_HappyPath accepts a finding and verifies state +
// reason are stored.
func TestFindingAccept_HappyPath(t *testing.T) {
	store, _ := setupFindingTest(t)
	mustRecord(t, store, "r1", "fp1", "pr-review", oneFinding("claude", "auth bypass"))
	store.Close()

	out := runCmd(t, "finding", "accept", "1", "--codebase", "fp1", "--reason", "real bug")
	if !strings.Contains(out, "accepted finding 1") {
		t.Errorf("missing confirm; got: %s", out)
	}

	// Reopen store to verify.
	st2 := mustOpen(t)
	defer st2.Close()
	row, err := findings.GetByID(st2, 1)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if row.UserAction == nil || *row.UserAction != "accepted" {
		t.Errorf("user_action=%v, want accepted", row.UserAction)
	}
	if row.ActionReason == nil || *row.ActionReason != "real bug" {
		t.Errorf("reason=%v, want 'real bug'", row.ActionReason)
	}
}

// TestFindingAccept_CrossCodebaseGuard verifies the spec's guard:
// accept refuses when finding's fp ≠ cwd's fp unless --cross-codebase.
func TestFindingAccept_CrossCodebaseGuard(t *testing.T) {
	store, _ := setupFindingTest(t)
	mustRecord(t, store, "r1", "fp-other", "pr-review", oneFinding("claude", "x"))
	store.Close()

	// Use --codebase to FORCE cwd resolution to a different fp.
	_, errOut, runErr := runCmdCapture(t, "finding", "accept", "1", "--codebase", "fp-mine")
	if runErr == nil {
		t.Fatalf("expected error from cross-codebase accept; output: %s", errOut)
	}
	if !strings.Contains(runErr.Error(), "--cross-codebase to override") {
		t.Errorf("error missing override hint: %v", runErr)
	}

	// With override, accept succeeds.
	out := runCmd(t, "finding", "accept", "1", "--codebase", "fp-mine", "--cross-codebase")
	if !strings.Contains(out, "accepted finding 1") {
		t.Errorf("override accept failed: %s", out)
	}
}

// TestFindingDismiss_RecordsState — symmetric to accept; verifies the
// shared cmd factory wires "dismissed" correctly.
func TestFindingDismiss_RecordsState(t *testing.T) {
	store, _ := setupFindingTest(t)
	mustRecord(t, store, "r1", "fp1", "pr-review", oneFinding("claude", "nit"))
	store.Close()

	runCmd(t, "finding", "dismiss", "1", "--codebase", "fp1")

	st2 := mustOpen(t)
	defer st2.Close()
	row, _ := findings.GetByID(st2, 1)
	if row.UserAction == nil || *row.UserAction != "dismissed" {
		t.Errorf("user_action=%v, want dismissed", row.UserAction)
	}
}

// TestFindingPrecision_BootstrapBucket verifies the < 10 actioned tuple
// renders with bootstrap weight; ≥ 10 renders with mature precision.
// (Renamed from TestFindingStats_BootstrapBucket in v0.14.0 when the
// `stats` cobra subcommand was renamed to `precision`.)
func TestFindingPrecision_BootstrapBucket(t *testing.T) {
	store, _ := setupFindingTest(t)
	// 3 findings, 2 accepted, 1 dismissed → actioned=3 → bootstrap
	results := []swarm.AgentResult{
		{Agent: "default-claude", Findings: []swarm.Finding{
			{Severity: "issue", File: "a.go", LineRange: "1", Summary: "f1"},
			{Severity: "issue", File: "a.go", LineRange: "2", Summary: "f2"},
			{Severity: "issue", File: "a.go", LineRange: "3", Summary: "f3"},
		}},
	}
	mustRecord(t, store, "r1", "fp1", "pr-review", results)
	for _, id := range []int64{1, 2} {
		_ = findings.SetAction(store, id, "accepted", "")
	}
	_ = findings.SetAction(store, 3, "dismissed", "")
	store.Close()

	out := runCmd(t, "finding", "precision", "--codebase", "fp1")
	if !strings.Contains(out, "bootstrap") {
		t.Errorf("expected bootstrap weight label; got:\n%s", out)
	}
	if !strings.Contains(out, "default-claude") {
		t.Errorf("missing persona row:\n%s", out)
	}
}

// TestFindingClear_DryRunByDefault verifies the spec's dry-run
// default: without --yes, count + exit 0 without mutation.
func TestFindingClear_DryRunByDefault(t *testing.T) {
	store, _ := setupFindingTest(t)
	mustRecord(t, store, "r1", "fp1", "pr-review", oneFinding("claude", "x"))
	store.Close()

	out := runCmd(t, "finding", "clear", "--codebase", "fp1")
	if !strings.Contains(out, "[dry-run]") || !strings.Contains(out, "1 row") {
		t.Errorf("dry-run output missing expected text:\n%s", out)
	}

	// Verify row still present.
	st2 := mustOpen(t)
	defer st2.Close()
	rows, _ := findings.ListFindings(st2, findings.ListOpts{CodebaseFP: "fp1"})
	if len(rows) != 1 {
		t.Errorf("dry-run mutated DB: rows=%d, want 1", len(rows))
	}
}

// TestFindingClear_YesActuallyDeletes covers the destructive path.
func TestFindingClear_YesActuallyDeletes(t *testing.T) {
	store, _ := setupFindingTest(t)
	mustRecord(t, store, "r1", "fp1", "pr-review", oneFinding("claude", "x"))
	mustRecord(t, store, "r2", "fp2", "pr-review", oneFinding("gemini", "y"))
	store.Close()

	out := runCmd(t, "finding", "clear", "--codebase", "fp1", "--yes")
	if !strings.Contains(out, "deleted 1 row") {
		t.Errorf("delete confirm missing:\n%s", out)
	}

	st2 := mustOpen(t)
	defer st2.Close()
	rows, _ := findings.ListFindings(st2, findings.ListOpts{AllCodebases: true})
	if len(rows) != 1 || rows[0].CodebaseFP != "fp2" {
		t.Errorf("only fp2 should remain; rows=%+v", rows)
	}
}

// TestFindingExport_WritesSchemaV1 verifies the JSON shape includes
// schema_version: 1 (forward-compat field for future imports).
func TestFindingExport_WritesSchemaV1(t *testing.T) {
	store, tmp := setupFindingTest(t)
	mustRecord(t, store, "r1", "fp1", "pr-review", oneFinding("claude", "x"))
	store.Close()

	dest := filepath.Join(tmp, "out.json")
	runCmd(t, "finding", "export", dest, "--codebase", "fp1")

	data, err := readFile(dest)
	if err != nil {
		t.Fatalf("read export: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload["schema_version"] != float64(1) {
		t.Errorf("schema_version=%v, want 1", payload["schema_version"])
	}
	rows, _ := payload["findings"].([]any)
	if len(rows) != 1 {
		t.Errorf("findings len=%d, want 1", len(rows))
	}
}

// TestFindingAccept_MissingIdFriendly verifies a missing id surfaces
// as actionable text, not a raw "sql: no rows in result set" leak.
func TestFindingAccept_MissingIdFriendly(t *testing.T) {
	store, _ := setupFindingTest(t)
	store.Close()

	_, _, err := runCmdCapture(t, "finding", "accept", "999")
	if err == nil {
		t.Fatal("expected error for missing id")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention 'not found': %v", err)
	}
	if strings.Contains(err.Error(), "sql:") {
		t.Errorf("raw sql error leaked: %v", err)
	}
}

// TestFinding_DBFlagOverridesEnv covers v0.8.3's --db persistent
// flag. Resolution order: --db > $KAIJUTSU_FINDINGS_DB > default.
// Test verifies the flag wins over the env var when both are set.
func TestFinding_DBFlagOverridesEnv(t *testing.T) {
	tmp := t.TempDir()
	envPath := filepath.Join(tmp, "env-db.sqlite")
	flagPath := filepath.Join(tmp, "flag-db.sqlite")

	// Create the flag-target DB so openFindingsStore's existence
	// check passes; env-target deliberately missing to detect leak.
	t.Setenv("KAIJUTSU_FINDINGS_DB", envPath)
	store, err := findings.Open(flagPath)
	if err != nil {
		t.Fatalf("seed flag DB: %v", err)
	}
	store.Close()

	out, _, err := runCmdCapture(t, "finding", "--db", flagPath, "list")
	if err != nil {
		t.Fatalf("--db override should resolve to flag-target DB; got error: %v\nout: %s", err, out)
	}
	if strings.Contains(out, envPath) {
		t.Errorf("--db flag was ignored (output references env path %q): %s", envPath, out)
	}
}

// TestFinding_NoStorePathErrorsCleanly verifies the spec contract
// that the DB is created on first swarm run, not on a `jutsu finding`
// invocation. Without the guard we'd silently drop a 0-row file at
// the user's HOME on a curiosity probe.
func TestFinding_NoStorePathErrorsCleanly(t *testing.T) {
	tmp := t.TempDir()
	missing := filepath.Join(tmp, "does-not-exist", "findings.db")
	t.Setenv("KAIJUTSU_FINDINGS_DB", missing)

	_, _, err := runCmdCapture(t, "finding", "list")
	if err == nil {
		t.Fatal("expected error when DB absent; got nil")
	}
	if !strings.Contains(err.Error(), "no findings store") {
		t.Errorf("error message missing actionable hint: %v", err)
	}
	// Side-effect check: the missing path must NOT have been created.
	if _, statErr := os.Stat(missing); !os.IsNotExist(statErr) {
		t.Errorf("openFindingsStore created the DB despite the guard: %v", statErr)
	}
}

// TestParseDurationSpec_DaysAndStdLib covers the days extension over
// time.ParseDuration.
func TestParseDurationSpec_DaysAndStdLib(t *testing.T) {
	cases := []struct {
		in   string
		want time.Duration
		bad  bool
	}{
		{"30d", 30 * 24 * time.Hour, false},
		{"1d", 24 * time.Hour, false},
		{"24h", 24 * time.Hour, false},
		{"15m", 15 * time.Minute, false},
		{"-1d", 0, true},
		{"-30m", 0, true},
		{"-1h", 0, true},
		{"abcd", 0, true},
		{"", 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got, err := parseDurationSpec(tc.in)
			if tc.bad {
				if err == nil {
					t.Fatalf("want error for %q, got %v", tc.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// --- helpers ---

// setupFindingTest creates an isolated temp HOME + temp DB via
// KAIJUTSU_FINDINGS_DB env override and returns an open store + tmp
// dir. Caller manages Close.
func setupFindingTest(t *testing.T) (*findings.Store, string) {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("KAIJUTSU_FINDINGS_DB", filepath.Join(tmp, "findings.db"))
	store, err := findings.Open(filepath.Join(tmp, "findings.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return store, tmp
}

func mustOpen(t *testing.T) *findings.Store {
	t.Helper()
	path := os.Getenv("KAIJUTSU_FINDINGS_DB")
	if path == "" {
		t.Fatal("KAIJUTSU_FINDINGS_DB not set; setupFindingTest must precede mustOpen")
	}
	s, err := findings.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return s
}

func mustRecord(t *testing.T, store *findings.Store, runID, fp, preset string, results []swarm.AgentResult) {
	t.Helper()
	if _, _, err := findings.RecordRun(store, findings.RunMeta{
		RunID: runID, CodebaseFP: fp, Preset: preset,
	}, results); err != nil {
		t.Fatalf("RecordRun: %v", err)
	}
}

func oneFinding(agent, summary string) []swarm.AgentResult {
	return []swarm.AgentResult{{
		Agent: agent,
		Findings: []swarm.Finding{
			{Severity: "issue", File: "x.go", LineRange: "1", Summary: summary},
		},
	}}
}

// runCmd executes the full root cobra tree against args. Returns
// stdout. Fatal on error so calling tests stay tight.
func runCmd(t *testing.T, args ...string) string {
	t.Helper()
	stdout, _, err := runCmdCapture(t, args...)
	if err != nil {
		t.Fatalf("cmd %v: %v", args, err)
	}
	return stdout
}

func runCmdCapture(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	root := NewRootCmd()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs(args)
	err := root.Execute()
	return stdout.String(), stderr.String(), err
}

func readFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}
