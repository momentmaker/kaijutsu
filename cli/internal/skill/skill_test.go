package skill

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadValid(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "skill.yaml")
	body := `
name: pr-review
version: 1.0.0
license: MIT
layout: flat
description: "Reviews a PR."
agents: [claude, codex, gemini]
permissions:
  bash: true
  network: false
  fs-write: scoped
`
	os.WriteFile(p, []byte(body), 0644)

	s, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if s.Name != "pr-review" {
		t.Errorf("name = %q", s.Name)
	}
	if !s.Permissions.Bash {
		t.Error("bash perm lost")
	}
	if s.Permissions.FsWrite != "scoped" {
		t.Errorf("fs-write = %v", s.Permissions.FsWrite)
	}
}

func TestLoadMissingRequired(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "skill.yaml")
	os.WriteFile(p, []byte("name: x\n"), 0644)
	if _, err := Load(p); err == nil {
		t.Error("expected validation error, got nil")
	}
}

func TestLoadBadLayout(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "skill.yaml")
	body := `
name: x
version: 1.0.0
license: MIT
layout: weird
agents: [claude]
`
	os.WriteFile(p, []byte(body), 0644)
	if _, err := Load(p); err == nil {
		t.Error("expected error for invalid layout")
	}
}
