package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// autopilot_test.go — v0.11.0 Stage 2 — covers the cobra group's
// load-bearing behaviors: init refuses overwrite without --force,
// status reads state, abort cleans state, resume reads state +
// reports next phase, run enforces hard ceiling.
//
// Phase orchestration itself lives in the skill body (SKILL.md);
// these tests cover the CLI surface contract.

func mustChdir(t *testing.T, dir string) {
	t.Helper()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
}

func TestAutopilotInit_WritesDefaults(t *testing.T) {
	tmp := t.TempDir()
	prev, _ := os.Getwd()
	mustChdir(t, tmp)
	defer os.Chdir(prev)

	cmd := newAutopilotCmd()
	cmd.SetArgs([]string{"init"})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("autopilot init: %v\nstderr: %s", err, stderr.String())
	}
	cfg := filepath.Join(tmp, ".kaijutsu", "autopilot.yaml")
	body, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatalf("read written config: %v", err)
	}
	for _, must := range []string{"gates:", "brainstorm:", "spec_review:", "plan_review:", "final_review:", "cost:", "max_total_usd: 20.00"} {
		if !strings.Contains(string(body), must) {
			t.Errorf("written config missing %q", must)
		}
	}
}

func TestAutopilotInit_RefusesOverwriteWithoutForce(t *testing.T) {
	tmp := t.TempDir()
	prev, _ := os.Getwd()
	mustChdir(t, tmp)
	defer os.Chdir(prev)

	if err := os.MkdirAll(filepath.Join(tmp, ".kaijutsu"), 0o755); err != nil {
		t.Fatal(err)
	}
	existing := []byte("# user-edited\n")
	cfg := filepath.Join(tmp, ".kaijutsu", "autopilot.yaml")
	if err := os.WriteFile(cfg, existing, 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := newAutopilotCmd()
	cmd.SetArgs([]string{"init"})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error when config exists without --force")
	}
	// Existing content must be preserved.
	body, _ := os.ReadFile(cfg)
	if !bytes.Equal(body, existing) {
		t.Errorf("init mutated existing config; body=%q want=%q", body, existing)
	}

	// With --force, overwrite succeeds.
	cmd2 := newAutopilotCmd()
	cmd2.SetArgs([]string{"init", "--force"})
	cmd2.SetOut(&stdout)
	cmd2.SetErr(&stderr)
	if err := cmd2.Execute(); err != nil {
		t.Fatalf("--force should allow overwrite: %v", err)
	}
	body, _ = os.ReadFile(cfg)
	if !strings.Contains(string(body), "gates:") {
		t.Error("--force did not write the default config")
	}
}

func TestAutopilotStatus_ReadsState(t *testing.T) {
	tmp := t.TempDir()
	prev, _ := os.Getwd()
	mustChdir(t, tmp)
	defer os.Chdir(prev)

	// No state file → "no autopilot run in progress".
	cmd := newAutopilotCmd()
	cmd.SetArgs([]string{"status"})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "no autopilot run in progress") {
		t.Errorf("expected 'no autopilot run in progress'; got %q", stdout.String())
	}

	// Write a state file; status should print its content.
	if err := os.MkdirAll(filepath.Join(tmp, ".kaijutsu"), 0o755); err != nil {
		t.Fatal(err)
	}
	stateBody := "---\nintent: test\nphase: spec\nslug: demo\n---\n"
	if err := os.WriteFile(filepath.Join(tmp, ".kaijutsu", "autopilot-state.md"), []byte(stateBody), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout.Reset()
	stderr.Reset()
	cmd2 := newAutopilotCmd()
	cmd2.SetArgs([]string{"status"})
	cmd2.SetOut(&stdout)
	cmd2.SetErr(&stderr)
	if err := cmd2.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "phase: spec") {
		t.Errorf("expected status to surface 'phase: spec'; got %q", stdout.String())
	}
}

func TestAutopilotAbort_CleansArtifacts(t *testing.T) {
	tmp := t.TempDir()
	prev, _ := os.Getwd()
	mustChdir(t, tmp)
	defer os.Chdir(prev)

	statePath := filepath.Join(tmp, ".kaijutsu", "autopilot-state.md")
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(statePath, []byte("---\nphase: build\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := newAutopilotCmd()
	cmd.SetArgs([]string{"abort", "--yes"})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("abort --yes: %v\nstderr: %s", err, stderr.String())
	}
	if _, err := os.Stat(statePath); !os.IsNotExist(err) {
		t.Errorf("abort did not remove state file: %v", err)
	}
}

func TestAutopilotResume_ResumesFromLastPhase(t *testing.T) {
	tmp := t.TempDir()
	prev, _ := os.Getwd()
	mustChdir(t, tmp)
	defer os.Chdir(prev)

	statePath := filepath.Join(tmp, ".kaijutsu", "autopilot-state.md")
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		t.Fatal(err)
	}
	stateBody := "---\nintent: \"demo task\"\nphase: plan\nslug: demo\n---\n"
	if err := os.WriteFile(statePath, []byte(stateBody), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := newAutopilotCmd()
	cmd.SetArgs([]string{"resume"})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := stdout.String()
	if !strings.Contains(out, "phase: plan") {
		t.Errorf("resume should surface state body; got %q", out)
	}
	if !strings.Contains(out, "/autopilot inside your agent CLI") {
		t.Errorf("resume should hint at slash-command; got %q", out)
	}
}

func TestAutopilotResume_NoStateFails(t *testing.T) {
	tmp := t.TempDir()
	prev, _ := os.Getwd()
	mustChdir(t, tmp)
	defer os.Chdir(prev)

	cmd := newAutopilotCmd()
	cmd.SetArgs([]string{"resume"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error when no state file exists")
	}
}

func TestAutopilotRun_RequiresYes(t *testing.T) {
	tmp := t.TempDir()
	prev, _ := os.Getwd()
	mustChdir(t, tmp)
	defer os.Chdir(prev)

	cmd := newAutopilotCmd()
	cmd.SetArgs([]string{"run", "test intent"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error: --yes required for non-interactive run")
	}
}

func TestAutopilotRun_AcceptsYesWithMaxCost(t *testing.T) {
	tmp := t.TempDir()
	prev, _ := os.Getwd()
	mustChdir(t, tmp)
	defer os.Chdir(prev)

	cmd := newAutopilotCmd()
	cmd.SetArgs([]string{"run", "test intent", "--yes", "--max-cost", "5.0"})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("run --yes --max-cost 5: %v", err)
	}
	if !strings.Contains(stdout.String(), "max_cost=$5.00") {
		t.Errorf("expected stdout to show max_cost=$5.00; got %q", stdout.String())
	}
}

// TestAutopilotCostCap_HardCeilingRejectsAbove covers the load-bearing
// guarantee: --max-cost above MaxAutopilotCostUSD is rejected
// regardless of any yaml content. Even if a malicious .kaijutsu/
// autopilot.yaml sets max_total_usd: 10000, the CLI flag check
// blocks excess at the entry point.
func TestAutopilotCostCap_HardCeilingRejectsAbove(t *testing.T) {
	tmp := t.TempDir()
	prev, _ := os.Getwd()
	mustChdir(t, tmp)
	defer os.Chdir(prev)

	cmd := newAutopilotCmd()
	// 1000 > MaxAutopilotCostUSD ($100). Should reject.
	cmd.SetArgs([]string{"run", "test", "--yes", "--max-cost", "1000"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error when --max-cost exceeds hard ceiling")
	}
}

// TestAutopilotCostCap_EnvOverrideRaisesCeiling covers the per-shell
// override path. With KAIJUTSU_AUTOPILOT_HARD_CAP_OVERRIDE=200 in the
// environment, --max-cost 150 should be accepted (below the new
// ceiling of $200, above the default $100).
func TestAutopilotCostCap_EnvOverrideRaisesCeiling(t *testing.T) {
	tmp := t.TempDir()
	prev, _ := os.Getwd()
	mustChdir(t, tmp)
	defer os.Chdir(prev)

	t.Setenv(AutopilotEnvHardCapOverride, "200")

	cmd := newAutopilotCmd()
	cmd.SetArgs([]string{"run", "test", "--yes", "--max-cost", "150"})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("env override should raise ceiling: %v", err)
	}
	if !strings.Contains(stdout.String(), "max_cost=$150.00") {
		t.Errorf("expected $150 max_cost honored; got %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "hard_ceiling=$200.00") {
		t.Errorf("expected ceiling raised to $200; got %q", stdout.String())
	}
}

// TestAutopilotCostCap_MaxCostFlagRaisesSoftCap pins the contract
// from doc-review: --max-cost works as a soft-cap raise WITHIN the
// hard ceiling. Doc-review #minor flagged this had no test.
func TestAutopilotCostCap_MaxCostFlagRaisesSoftCap(t *testing.T) {
	tmp := t.TempDir()
	prev, _ := os.Getwd()
	mustChdir(t, tmp)
	defer os.Chdir(prev)

	cmd := newAutopilotCmd()
	cmd.SetArgs([]string{"run", "test", "--yes", "--max-cost", "50"})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("--max-cost 50 should be accepted (within $100 ceiling): %v", err)
	}
	if !strings.Contains(stdout.String(), "max_cost=$50.00") {
		t.Errorf("expected --max-cost 50 honored; got %q", stdout.String())
	}
}

// TestAutopilotResolveHardCap_DefaultWhenEnvUnset pins the default
// behavior: no env var → MaxAutopilotCostUSD constant.
func TestAutopilotResolveHardCap_DefaultWhenEnvUnset(t *testing.T) {
	t.Setenv(AutopilotEnvHardCapOverride, "")
	if got := resolveHardCap(); got != MaxAutopilotCostUSD {
		t.Errorf("default ceiling = %v, want %v", got, MaxAutopilotCostUSD)
	}
}

// TestAutopilotResolveHardCap_IgnoresMalformedEnv pins safety: a
// non-numeric env var doesn't crash + falls back to the default.
func TestAutopilotResolveHardCap_IgnoresMalformedEnv(t *testing.T) {
	t.Setenv(AutopilotEnvHardCapOverride, "not-a-number")
	if got := resolveHardCap(); got != MaxAutopilotCostUSD {
		t.Errorf("malformed env should fall back to default; got %v", got)
	}
}

// TestAutopilotResolveHardCap_IgnoresNegativeEnv pins safety: a
// negative env value falls back to default rather than disabling
// the ceiling.
func TestAutopilotResolveHardCap_IgnoresNegativeEnv(t *testing.T) {
	t.Setenv(AutopilotEnvHardCapOverride, "-1")
	if got := resolveHardCap(); got != MaxAutopilotCostUSD {
		t.Errorf("negative env should fall back; got %v", got)
	}
}

// TestAutopilotRun_TestModeWritesPRJSON pins the CI test-mode
// contract from the spec: with KAIJUTSU_AUTOPILOT_TEST_MODE=1, the
// run command writes a planned-PR JSON to .kaijutsu/autopilot-pr.json
// instead of invoking gh. CI tests assert against this file.
func TestAutopilotRun_TestModeWritesPRJSON(t *testing.T) {
	tmp := t.TempDir()
	prev, _ := os.Getwd()
	mustChdir(t, tmp)
	defer os.Chdir(prev)

	t.Setenv(AutopilotEnvTestMode, "1")

	cmd := newAutopilotCmd()
	cmd.SetArgs([]string{"run", "add a hello world endpoint", "--yes", "--max-cost", "10"})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("test-mode run: %v\nstdout: %s", err, stdout.String())
	}

	path := filepath.Join(tmp, ".kaijutsu", "autopilot-pr.json")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read planned PR: %v", err)
	}
	var planned struct {
		Intent      string   `json:"intent"`
		Title       string   `json:"title"`
		Branch      string   `json:"branch"`
		Labels      []string `json:"labels"`
		MaxCostUSD  float64  `json:"max_cost_usd"`
		HardCeiling float64  `json:"hard_ceiling_usd"`
		TestMode    bool     `json:"test_mode"`
	}
	if err := json.Unmarshal(body, &planned); err != nil {
		t.Fatalf("parse planned PR: %v\nbody: %s", err, body)
	}
	if planned.Intent != "add a hello world endpoint" {
		t.Errorf("planned.Intent = %q", planned.Intent)
	}
	if !strings.HasPrefix(planned.Title, "autopilot:") {
		t.Errorf("planned.Title = %q, want prefix 'autopilot:'", planned.Title)
	}
	if planned.MaxCostUSD != 10 {
		t.Errorf("planned.MaxCostUSD = %v, want 10", planned.MaxCostUSD)
	}
	if planned.HardCeiling != MaxAutopilotCostUSD {
		t.Errorf("planned.HardCeiling = %v, want %v", planned.HardCeiling, MaxAutopilotCostUSD)
	}
	if !planned.TestMode {
		t.Error("planned.TestMode must be true in test mode")
	}
	if planned.Branch == "" {
		t.Error("planned.Branch must be non-empty")
	}
	if !strings.Contains(stdout.String(), "test-mode: planned PR written to") {
		t.Errorf("stdout missing test-mode marker; got %q", stdout.String())
	}
}

// TestAutopilotRun_NonTestModeDoesNotWritePRJSON pins the contract
// boundary: without the env var, no .kaijutsu/autopilot-pr.json is
// produced. Production runs go through the agent CLI orchestration
// path, not this CLI command.
func TestAutopilotRun_NonTestModeDoesNotWritePRJSON(t *testing.T) {
	tmp := t.TempDir()
	prev, _ := os.Getwd()
	mustChdir(t, tmp)
	defer os.Chdir(prev)

	t.Setenv(AutopilotEnvTestMode, "")

	cmd := newAutopilotCmd()
	cmd.SetArgs([]string{"run", "test", "--yes", "--max-cost", "5"})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("non-test-mode run: %v", err)
	}

	path := filepath.Join(tmp, ".kaijutsu", "autopilot-pr.json")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("non-test-mode should not produce planned PR JSON; stat err = %v", err)
	}
}
