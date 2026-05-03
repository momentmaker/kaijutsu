package install

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/momentmaker/kaijutsu/cli/internal/skill"
)

func TestInstallCopiesSkillDirToBothFamilies(t *testing.T) {
	tmp := t.TempDir()

	src := filepath.Join(tmp, "src", "decide")
	if err := os.MkdirAll(filepath.Join(src, "scripts"), 0755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(src, "SKILL.md"), "---\nname: decide\n---\nbody")
	mustWrite(t, filepath.Join(src, "scripts", "x.sh"), "#!/bin/sh")

	sk := &skill.Skill{Name: "decide", Agents: []string{"claude", "codex", "gemini"}}
	root := filepath.Join(tmp, "proj")

	if err := Install(src, root, []string{"claude", "codex"}, sk); err != nil {
		t.Fatal(err)
	}

	wants := []string{
		filepath.Join(root, ".claude", "skills", "decide", "SKILL.md"),
		filepath.Join(root, ".agents", "skills", "decide", "SKILL.md"),
		filepath.Join(root, ".claude", "skills", "decide", "scripts", "x.sh"),
		filepath.Join(root, ".agents", "skills", "decide", "scripts", "x.sh"),
	}
	for _, w := range wants {
		if _, err := os.Stat(w); err != nil {
			t.Errorf("missing %s: %v", w, err)
		}
	}
}

func TestInstallSingleAgentWritesOneDir(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src", "x")
	os.MkdirAll(src, 0755)
	mustWrite(t, filepath.Join(src, "SKILL.md"), "body")

	sk := &skill.Skill{Name: "x", Agents: []string{"codex", "gemini"}}
	root := filepath.Join(tmp, "proj")
	if err := Install(src, root, []string{"codex"}, sk); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(root, ".agents", "skills", "x", "SKILL.md")); err != nil {
		t.Errorf("expected codex install: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude")); !os.IsNotExist(err) {
		t.Errorf("expected no .claude dir, got %v", err)
	}
}

func TestInstallNoOverlapErrors(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "skill")
	os.MkdirAll(src, 0755)
	sk := &skill.Skill{Name: "x", Agents: []string{"gemini"}}
	if err := Install(src, tmp, []string{"claude"}, sk); err == nil {
		t.Error("expected error for no overlap, got nil")
	}
}

func TestRemove(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src", "x")
	os.MkdirAll(src, 0755)
	mustWrite(t, filepath.Join(src, "SKILL.md"), "body")

	sk := &skill.Skill{Name: "x", Agents: []string{"claude", "codex"}}
	root := filepath.Join(tmp, "proj")
	if err := Install(src, root, []string{"claude", "codex"}, sk); err != nil {
		t.Fatal(err)
	}
	if err := Remove(root, "x", []string{"claude", "codex"}); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{
		filepath.Join(root, ".claude", "skills", "x"),
		filepath.Join(root, ".agents", "skills", "x"),
	} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("%s should be removed, got %v", p, err)
		}
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
