package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/momentmaker/kaijutsu/cli/internal/findings"
	"github.com/momentmaker/kaijutsu/cli/internal/swarm"
)

// finding_stats_test.go covers the v0.14.0 `jutsu finding stats`
// cobra subcommand: TTY/JSON output, --since/--by/--source flag
// validation, source enrichment via swarm.DefaultRegistry, empty-DB
// behavior.

func TestFindingStats_HelpListsFlags(t *testing.T) {
	cmd := newFindingStatsCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"--by", "--since", "--source", "--codebase", "--all-codebases", "--json"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("help missing %q", want)
		}
	}
}

func TestFindingStats_PipeRendersJSON(t *testing.T) {
	store, _ := setupFindingTest(t)
	mustRecord(t, store, "r1", "fp1", "pr-review", oneFinding("default-claude", "x"))
	mustRecord(t, store, "r2", "fp1", "pr-review", oneFinding("default-claude", "y"))
	store.Close()

	out := runCmd(t, "finding", "stats", "--codebase", "fp1", "--json")
	var rows []map[string]any
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("output is not valid JSON: %v\nbody: %s", err, out)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d; want 1\nbody: %s", len(rows), out)
	}
	if rows[0]["group"] != "pr-review" {
		t.Errorf("group = %v; want pr-review", rows[0]["group"])
	}
	if rows[0]["count"].(float64) != 2 {
		t.Errorf("count = %v; want 2", rows[0]["count"])
	}
	if rows[0]["source"] != "built-in" {
		t.Errorf("source = %v; want built-in (pr-review is registered)", rows[0]["source"])
	}
}

func TestFindingStats_EmptyDB_JSONNoErrorExit(t *testing.T) {
	// Through runCmd, stdout is a buffer not a TTY so JSON path runs;
	// empty DB must emit `[]\n` and exit 0 (no error).
	store, _ := setupFindingTest(t)
	store.Close()

	out := runCmd(t, "finding", "stats", "--codebase", "fp1")
	if strings.TrimSpace(out) != "[]" {
		t.Errorf("expected JSON empty array on empty DB; got: %q", out)
	}
}

func TestRenderStatsTable_EmptyEmitsFriendlyMessage(t *testing.T) {
	// TTY path is hit by direct renderer call; runCmd always falls
	// through to JSON via the bytes.Buffer not-a-TTY check.
	var buf bytes.Buffer
	renderStatsTable(&buf, nil, findings.StatsByPreset, "90d", findings.StatsOpts{})
	if !strings.Contains(buf.String(), "(no recorded runs in window)") {
		t.Errorf("expected friendly empty message; got: %s", buf.String())
	}
}

func TestFindingStats_GroupByPersona(t *testing.T) {
	store, _ := setupFindingTest(t)
	mustRecord(t, store, "r1", "fp1", "pr-review", oneFinding("default-claude", "x"))
	mustRecord(t, store, "r2", "fp1", "pr-review", oneFinding("default-antigravity", "y"))
	store.Close()

	out := runCmd(t, "finding", "stats", "--codebase", "fp1", "--by", "persona", "--json")
	var rows []map[string]any
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}
	if len(rows) != 2 {
		t.Errorf("rows = %d; want 2 (default-claude + default-antigravity)", len(rows))
	}
	for _, r := range rows {
		if _, has := r["source"]; has {
			t.Errorf("--by persona row should NOT carry source field; got %+v", r)
		}
	}
}

func TestFindingStats_BadByRejected(t *testing.T) {
	cmd := newFindingStatsCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--by", "bogus"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for --by bogus")
	}
	if !strings.Contains(err.Error(), "--by") {
		t.Errorf("error should name --by; got: %v", err)
	}
	if got := ExitCode(err); got != ExitUsage {
		t.Errorf("ExitCode = %d; want %d (usage)", got, ExitUsage)
	}
}

func TestFindingStats_BadSourceRejected(t *testing.T) {
	cmd := newFindingStatsCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--source", "bogus"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for --source bogus")
	}
	if !strings.Contains(err.Error(), "--source") {
		t.Errorf("error should name --source; got: %v", err)
	}
	if got := ExitCode(err); got != ExitUsage {
		t.Errorf("ExitCode = %d; want %d (usage)", got, ExitUsage)
	}
}

func TestFindingStats_SourceWithNonPresetByRejected(t *testing.T) {
	cmd := newFindingStatsCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--by", "persona", "--source", "built-in"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error: --source is preset-only")
	}
	if !strings.Contains(err.Error(), "--source") || !strings.Contains(err.Error(), "preset") {
		t.Errorf("error should explain the preset-only constraint; got: %v", err)
	}
	if got := ExitCode(err); got != ExitUsage {
		t.Errorf("ExitCode = %d; want %d (usage)", got, ExitUsage)
	}
}

func TestFindingStats_BadSinceRejected(t *testing.T) {
	cmd := newFindingStatsCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--since", "0d"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for --since 0d")
	}
	if got := ExitCode(err); got != ExitUsage {
		t.Errorf("ExitCode = %d; want %d (usage)", got, ExitUsage)
	}
}

func TestFindingStats_AllTimeWindow(t *testing.T) {
	store, _ := setupFindingTest(t)
	mustRecord(t, store, "r1", "fp1", "pr-review", oneFinding("default-claude", "x"))
	store.Close()

	// Empty --since must be passed positionally to bypass the 90d default.
	out := runCmd(t, "finding", "stats", "--codebase", "fp1", "--since", "", "--json")
	var rows []map[string]any
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}
	if len(rows) != 1 {
		t.Errorf("rows = %d; want 1 (all-time should include the row)", len(rows))
	}
}

func TestFindingStats_SourceFilter_BuiltinOnly(t *testing.T) {
	store, _ := setupFindingTest(t)
	mustRecord(t, store, "r1", "fp1", "pr-review", oneFinding("default-claude", "x"))
	mustRecord(t, store, "r2", "fp1", "ghost-preset", oneFinding("default-claude", "y"))
	store.Close()

	out := runCmd(t, "finding", "stats", "--codebase", "fp1", "--source", "built-in", "--json")
	var rows []map[string]any
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}
	for _, r := range rows {
		if r["group"] == "ghost-preset" {
			t.Error("--source built-in should filter out unregistered preset")
		}
	}
	// pr-review must remain.
	found := false
	for _, r := range rows {
		if r["group"] == "pr-review" {
			found = true
		}
	}
	if !found {
		t.Errorf("pr-review row missing under --source built-in; got rows: %v", rows)
	}
}

func TestFindingStats_SourceUserMatchesUserPreset(t *testing.T) {
	store, _ := setupFindingTest(t)
	mustRecord(t, store, "r1", "fp1", "my-user-preset", oneFinding("default-claude", "x"))
	store.Close()

	// Register the user preset on the default registry so source enrichment
	// classifies it as `user`. Restore registry state on cleanup.
	reg := swarm.DefaultRegistry()
	registered, _ := reg.RegisterUserPresets(map[string]*swarm.UserPreset{
		"my-user-preset": {
			Preset: swarm.Preset{Name: "my-user-preset"},
			Source: swarm.SourceProject,
		},
	})
	t.Cleanup(func() {
		for _, n := range registered {
			reg.Remove(n)
		}
	})

	out := runCmd(t, "finding", "stats", "--codebase", "fp1", "--source", "user", "--json")
	var rows []map[string]any
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}
	found := false
	for _, r := range rows {
		if r["group"] == "my-user-preset" && r["source"] == "user" {
			found = true
		}
	}
	if !found {
		t.Errorf("my-user-preset row missing under --source user; got: %v", rows)
	}
}

// TestParseStatsByFlag_AcceptsValid keeps the parser tightly coupled
// to the findings.StatsBy enum so a future reshuffle stays caught.
func TestParseStatsByFlag_AcceptsValid(t *testing.T) {
	for _, s := range []string{"preset", "persona", "provider"} {
		got, err := parseStatsByFlag(s)
		if err != nil {
			t.Errorf("%q errored: %v", s, err)
		}
		if string(got) != s {
			t.Errorf("%q: got %q, want %q", s, got, s)
		}
	}
}

