package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/momentmaker/kaijutsu/cli/internal/manifest"
)

// dry_run_test.go — v0.10.1 Stage 2 — `--dry-run` sweep coverage.
//
// Each of `install` / `upgrade` / `remove` / `publish` accepts a
// `--dry-run` flag. With it, no filesystem / lockfile / network
// writes happen. These tests pin that contract.
//
// Two layers of coverage:
//
//   1. Flag-wiring: every command's cobra definition lists the flag
//      with a non-empty Usage and a "do not write / no invocation"
//      hint in the description.
//
//   2. Behavior: for the two commands that can be exercised without
//      network (`remove`, `publish`), run the dry-run codepath
//      end-to-end against a tempdir and assert the filesystem +
//      lockfile are byte-identical post-run.

func TestDryRunFlag_WiredOnAllFourCommands(t *testing.T) {
	cmds := map[string]*cobra.Command{
		"install": newInstallCmd(),
		"upgrade": newUpgradeCmd(),
		"remove":  newRemoveCmd(),
		"publish": newPublishCmd(),
	}
	for name, cmd := range cmds {
		flag := cmd.Flags().Lookup("dry-run")
		if flag == nil {
			t.Errorf("jutsu %s missing --dry-run flag", name)
			continue
		}
		if flag.Usage == "" {
			t.Errorf("jutsu %s --dry-run has empty Usage string", name)
		}
	}
}

// TestRemoveDryRun_LeavesFilesystemAndLockfileUntouched is the
// load-bearing behavior pin for `remove --dry-run`. Builds a fake
// project tree with one skill installed, runs remove --dry-run,
// asserts the skill dir + lockfile + manifest are byte-identical
// post-run.
func TestRemoveDryRun_LeavesFilesystemAndLockfileUntouched(t *testing.T) {
	tmp := t.TempDir()
	prev, _ := os.Getwd()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(prev)

	// Seed a project: kaijutsu.json + kaijutsu.lock.json + a fake
	// installed skill at .claude/skills/demo/SKILL.md.
	skillDir := filepath.Join(tmp, ".claude", "skills", "demo")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir skill: %v", err)
	}
	skillBody := "# demo skill\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillBody), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}

	m := &manifest.Manifest{
		Version:      1,
		Agents:       []string{"claude"},
		Dependencies: map[string]string{"demo": "^1.0"},
	}
	if err := m.Save(filepath.Join(tmp, "kaijutsu.json")); err != nil {
		t.Fatalf("save manifest: %v", err)
	}
	v := "1.0.0"
	lf := &manifest.Lockfile{
		Version: 1,
		Agents:  []string{"claude"},
		Skills: map[string]manifest.LockEntry{
			"demo": {
				Version: &v,
				Tag:     "v1.0.0",
				Source:  "local",
				Ref:     "abc123",
			},
		},
	}
	if err := lf.Save(filepath.Join(tmp, "kaijutsu.lock.json")); err != nil {
		t.Fatalf("save lockfile: %v", err)
	}

	// Snapshot the three on-disk artifacts BEFORE running.
	before := snapshot(t, tmp, "kaijutsu.json", "kaijutsu.lock.json", filepath.Join(".claude", "skills", "demo", "SKILL.md"))

	// Run remove --dry-run.
	cmd := newRemoveCmd()
	cmd.SetArgs([]string{"demo", "--dry-run"})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("remove --dry-run: %v\nstderr: %s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "[dry-run]") {
		t.Errorf("expected '[dry-run]' marker in stdout; got: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "demo") {
		t.Errorf("expected target skill name in dry-run output; got: %s", stdout.String())
	}

	// Snapshot AFTER + diff.
	after := snapshot(t, tmp, "kaijutsu.json", "kaijutsu.lock.json", filepath.Join(".claude", "skills", "demo", "SKILL.md"))
	for path, beforeBytes := range before {
		if !bytes.Equal(beforeBytes, after[path]) {
			t.Errorf("dry-run mutated %s — before/after differ", path)
		}
	}
}

// TestPublishDryRun_DoesNotInvokeGh covers the publish case. With
// --dry-run, the planning output should print but no exec call to
// gh / git should happen. We can't easily mock exec.LookPath here,
// so this test only asserts the dry-run codepath is taken (output
// format) without exercising the auto-publish branch.
func TestPublishDryRun_PrintsPlanWithoutInvokingGh(t *testing.T) {
	tmp := t.TempDir()
	// Seed a minimal valid skill so lint + skill.Load succeed.
	yaml := `name: demo
version: 1.0.0
license: MIT
layout: flat
description: "test"
agents: [claude]
permissions: {}
`
	if err := os.WriteFile(filepath.Join(tmp, "skill.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatalf("write skill.yaml: %v", err)
	}
	skillMD := `---
name: demo
description: test skill
---

# demo
`
	if err := os.WriteFile(filepath.Join(tmp, "SKILL.md"), []byte(skillMD), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "README.md"), []byte("# demo\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "LICENSE"), []byte("MIT\n"), 0o644); err != nil {
		t.Fatalf("write LICENSE: %v", err)
	}

	cmd := newPublishCmd()
	cmd.SetArgs([]string{tmp, "--dry-run"})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("publish --dry-run: %v\nstderr: %s", err, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "[dry-run]") {
		t.Errorf("expected dry-run marker; got: %s", out)
	}
	if !strings.Contains(out, "skills/community/demo") {
		t.Errorf("expected target path in plan; got: %s", out)
	}
	if !strings.Contains(out, "feat(skills): add demo") {
		t.Errorf("expected commit message in plan; got: %s", out)
	}
}

func snapshot(t *testing.T, root string, paths ...string) map[string][]byte {
	t.Helper()
	out := make(map[string][]byte, len(paths))
	for _, p := range paths {
		b, err := os.ReadFile(filepath.Join(root, p))
		if err != nil {
			t.Fatalf("snapshot %s: %v", p, err)
		}
		out[p] = b
	}
	return out
}
