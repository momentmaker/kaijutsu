package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLoadLocal_FindsCommunityTier covers the v0.16.1 fix: skills under
// skills/community/<name>/ must be installable via the local-registry
// dev path, not just skills/core/<name>/. Before the fix, loadLocal
// only probed skills/core, making community-tier skills (e.g.
// editorial-review) invisible to `jutsu install --registry`.
func TestLoadLocal_FindsCommunityTier(t *testing.T) {
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "skills", "community", "editorial-review")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	yaml := `name: editorial-review
version: 0.1.0
license: MIT
layout: flat
description: "Four-pass essay review."
agents: [claude]
permissions:
  bash: false
  network: false
  fs-write: false
`
	if err := os.WriteFile(filepath.Join(dir, "skill.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	l, err := loadLocal(tmp, "editorial-review")
	if err != nil {
		t.Fatalf("loadLocal: %v", err)
	}
	if l.skill.Name != "editorial-review" {
		t.Errorf("skill name = %q, want editorial-review", l.skill.Name)
	}
	if l.path != "skills/community/editorial-review" {
		t.Errorf("path = %q, want skills/community/editorial-review", l.path)
	}
}

// TestLoadLocal_CoreTakesPrecedence: when both core and community
// versions exist (shouldn't happen in practice, but defensive), core
// wins. Pins the probe order so a future contributor noticing the
// duplication doesn't silently flip the precedence.
func TestLoadLocal_CoreTakesPrecedence(t *testing.T) {
	tmp := t.TempDir()
	for _, tier := range []string{"core", "community"} {
		dir := filepath.Join(tmp, "skills", tier, "dup")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		yaml := "name: dup\nversion: 0.1.0\nlicense: MIT\nlayout: flat\ndescription: \"x\"\nagents: [claude]\npermissions:\n  bash: false\n  network: false\n  fs-write: false\n"
		if err := os.WriteFile(filepath.Join(dir, "skill.yaml"), []byte(yaml), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	l, err := loadLocal(tmp, "dup")
	if err != nil {
		t.Fatal(err)
	}
	if l.path != "skills/core/dup" {
		t.Errorf("path = %q, want skills/core/dup (core must take precedence)", l.path)
	}
}

func TestLoadLocal_NotFoundMentionsBothTiers(t *testing.T) {
	tmp := t.TempDir()
	_, err := loadLocal(tmp, "nonexistent")
	if err == nil {
		t.Fatal("expected NotFoundError")
	}
	if ExitCode(err) != ExitNotFound {
		t.Errorf("ExitCode = %d, want %d", ExitCode(err), ExitNotFound)
	}
	if !strings.Contains(err.Error(), "core") || !strings.Contains(err.Error(), "community") {
		t.Errorf("error should mention both tiers; got: %v", err)
	}
}
