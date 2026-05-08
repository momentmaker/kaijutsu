package skill

import (
	"os"
	"path/filepath"
	"strings"
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

// TestErrorEnum_HookEvent covers the v0.10.1 enumeration sweep:
// invalid hook event must surface every valid event name in the
// error message so authors can self-correct without consulting docs.
func TestErrorEnum_HookEvent(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "skill.yaml")
	body := `
name: demo-skill
version: 1.0.0
license: MIT
layout: flat
agents: [claude]
hooks:
  - id: bad-event
    event: not-a-real-event
    matcher: "*"
    script: scripts/x.sh
`
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := Load(p)
	if err == nil {
		t.Fatal("expected error for invalid hook event")
	}
	msg := err.Error()
	for _, must := range []string{"pre-tool-use", "post-tool-use", "session-start"} {
		if !strings.Contains(msg, must) {
			t.Errorf("hook-event error %q missing valid event %q", msg, must)
		}
	}
}
