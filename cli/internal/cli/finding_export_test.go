package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// finding_export_test.go pins the v0.15 export upgrades:
// --format json|jsonl, --since duration, optional path → stdout for jsonl.

func TestFindingExport_DefaultJSONUnchangedBackCompat(t *testing.T) {
	store, _ := setupFindingTest(t)
	mustRecord(t, store, "r1", "fp1", "pr-review", oneFinding("default-claude", "x"))
	store.Close()

	dir := t.TempDir()
	out := filepath.Join(dir, "all.json")
	stdout, _, err := runCmdCapture(t, "finding", "export", "--codebase", "fp1", out)
	if err != nil {
		t.Fatalf("cmd: %v\n%s", err, stdout)
	}
	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		SchemaVersion int                      `json:"schema_version"`
		Findings      []map[string]interface{} `json:"findings"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("parse: %v\n%s", err, body)
	}
	if payload.SchemaVersion != 1 {
		t.Errorf("schema_version = %d; want 1", payload.SchemaVersion)
	}
	if len(payload.Findings) != 1 {
		t.Errorf("findings = %d; want 1", len(payload.Findings))
	}
}

func TestFindingExport_JSONLToStdoutPathless(t *testing.T) {
	store, _ := setupFindingTest(t)
	mustRecord(t, store, "r1", "fp1", "pr-review", oneFinding("default-claude", "x"))
	mustRecord(t, store, "r2", "fp1", "pr-review", oneFinding("default-gemini", "y"))
	store.Close()

	out, _, err := runCmdCapture(t, "finding", "export", "--codebase", "fp1", "--format", "jsonl")
	if err != nil {
		t.Fatalf("cmd: %v\n%s", err, out)
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines; got %d:\n%s", len(lines), out)
	}
	for i, line := range lines {
		var row map[string]interface{}
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			t.Errorf("line %d not JSON: %v", i, err)
		}
	}
}

func TestFindingExport_JSONLDashPath(t *testing.T) {
	store, _ := setupFindingTest(t)
	mustRecord(t, store, "r1", "fp1", "pr-review", oneFinding("default-claude", "x"))
	store.Close()

	out, _, err := runCmdCapture(t, "finding", "export", "--codebase", "fp1", "--format", "jsonl", "-")
	if err != nil {
		t.Fatalf("cmd: %v", err)
	}
	if !strings.Contains(out, `"summary":"x"`) {
		t.Errorf("expected JSONL with row content; got %q", out)
	}
}

func TestFindingExport_JSONLToFile(t *testing.T) {
	store, _ := setupFindingTest(t)
	mustRecord(t, store, "r1", "fp1", "pr-review", oneFinding("default-claude", "x"))
	store.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "out.jsonl")
	stdout, _, err := runCmdCapture(t, "finding", "export", "--codebase", "fp1", "--format", "jsonl", path)
	if err != nil {
		t.Fatalf("cmd: %v", err)
	}
	if !strings.Contains(stdout, "exported 1 row(s)") {
		t.Errorf("expected confirmation line on stdout; got %q", stdout)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), `"summary":"x"`) {
		t.Errorf("expected row in JSONL file; got %q", body)
	}
}

func TestFindingExport_SinceWindowFilters(t *testing.T) {
	store, _ := setupFindingTest(t)
	mustRecord(t, store, "old", "fp1", "pr-review", oneFinding("default-claude", "old-row"))
	if _, err := store.DB().Exec(`UPDATE findings SET created_at = ? WHERE run_id = 'old'`, "2020-01-01 00:00:00"); err != nil {
		t.Fatal(err)
	}
	mustRecord(t, store, "fresh", "fp1", "pr-review", oneFinding("default-claude", "fresh-row"))
	store.Close()

	out, _, err := runCmdCapture(t, "finding", "export", "--codebase", "fp1", "--format", "jsonl", "--since", "7d")
	if err != nil {
		t.Fatalf("cmd: %v", err)
	}
	if strings.Contains(out, "old-row") {
		t.Errorf("old row should be filtered out by --since 7d; got:\n%s", out)
	}
	if !strings.Contains(out, "fresh-row") {
		t.Errorf("fresh row should be present; got:\n%s", out)
	}
}

func TestFindingExport_BadFormatRejectedExit2(t *testing.T) {
	cmd := newFindingExportCmd()
	cmd.SetArgs([]string{"--format", "bogus", "/tmp/x"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for --format bogus")
	}
	if got := ExitCode(err); got != ExitUsage {
		t.Errorf("ExitCode = %d; want %d (usage)", got, ExitUsage)
	}
}

func TestFindingExport_BadSinceRejectedExit2(t *testing.T) {
	cmd := newFindingExportCmd()
	cmd.SetArgs([]string{"--since", "0d", "--format", "jsonl"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for --since 0d")
	}
	if got := ExitCode(err); got != ExitUsage {
		t.Errorf("ExitCode = %d; want %d (usage)", got, ExitUsage)
	}
}

func TestFindingExport_JSONRequiresPath(t *testing.T) {
	cmd := newFindingExportCmd()
	cmd.SetArgs([]string{"--format", "json"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error: --format json without path")
	}
	if got := ExitCode(err); got != ExitUsage {
		t.Errorf("ExitCode = %d; want %d (usage)", got, ExitUsage)
	}
}
