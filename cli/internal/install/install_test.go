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

	sk := &skill.Skill{Name: "decide", Agents: []string{"claude", "codex", "antigravity"}}
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
	src := filepath.Join(tmp, "src", "demo")
	os.MkdirAll(src, 0755)
	mustWrite(t, filepath.Join(src, "SKILL.md"), "body")

	sk := &skill.Skill{Name: "demo", Agents: []string{"codex", "antigravity"}}
	root := filepath.Join(tmp, "proj")
	if err := Install(src, root, []string{"codex"}, sk); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(root, ".agents", "skills", "demo", "SKILL.md")); err != nil {
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
	sk := &skill.Skill{Name: "demo", Agents: []string{"antigravity"}}
	if err := Install(src, tmp, []string{"claude"}, sk); err == nil {
		t.Error("expected error for no overlap, got nil")
	}
}

func TestInstallRejectsTraversalName(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src")
	os.MkdirAll(src, 0755)
	sk := &skill.Skill{Name: "../evil", Agents: []string{"claude"}}
	if err := Install(src, tmp, []string{"claude"}, sk); err == nil {
		t.Error("expected validation error for traversal name, got nil")
	}
}

func TestRemove(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src", "demo")
	os.MkdirAll(src, 0755)
	mustWrite(t, filepath.Join(src, "SKILL.md"), "body")

	sk := &skill.Skill{Name: "demo", Agents: []string{"claude", "codex"}}
	root := filepath.Join(tmp, "proj")
	if err := Install(src, root, []string{"claude", "codex"}, sk); err != nil {
		t.Fatal(err)
	}
	if err := Remove(root, "demo", []string{"claude", "codex"}); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{
		filepath.Join(root, ".claude", "skills", "demo"),
		filepath.Join(root, ".agents", "skills", "demo"),
	} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("%s should be removed, got %v", p, err)
		}
	}
}

func TestRemoveRejectsTraversalName(t *testing.T) {
	tmp := t.TempDir()
	if err := Remove(tmp, "../evil", []string{"claude"}); err == nil {
		t.Error("expected validation error, got nil")
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

// TestInstallAutopilot_ArchivesPreExistingSkillMd covers the v0.11.0
// breaking-change handler: pre-existing autopilot SKILL.md gets
// copied to a sibling .archived/ dir before the install pipeline
// overwrites the target. Sibling-not-child placement matters
// because Install does RemoveAll(fullDest) before copy — a child
// .archived/ would be nuked.
func TestInstallAutopilot_ArchivesPreExistingSkillMd(t *testing.T) {
	tmp := t.TempDir()

	src := filepath.Join(tmp, "src", "autopilot")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(src, "SKILL.md"), "v2 content\n")

	root := filepath.Join(tmp, "home")
	// Pre-seed a v1 SKILL.md at the install destination — simulates
	// an existing user install at ~/.claude/skills/autopilot/.
	preExistingDir := filepath.Join(root, ".claude", "skills", "autopilot")
	preExistingPath := filepath.Join(preExistingDir, "SKILL.md")
	v1Body := "v1 SKILL.md content with user edits\n"
	mustWrite(t, preExistingPath, v1Body)

	sk := &skill.Skill{Name: "autopilot", Agents: []string{"claude"}}
	if err := Install(src, root, []string{"claude"}, sk); err != nil {
		t.Fatal(err)
	}

	// The new SKILL.md should be in place.
	body, err := os.ReadFile(preExistingPath)
	if err != nil {
		t.Fatalf("read post-install SKILL.md: %v", err)
	}
	if string(body) != "v2 content\n" {
		t.Errorf("post-install SKILL.md = %q, want v2 content", body)
	}

	// The pre-existing SKILL.md should be preserved at the SIBLING
	// .archived/ dir.
	archivedPath := filepath.Join(root, ".claude", "skills", "autopilot.archived", "SKILL.md")
	archivedBody, err := os.ReadFile(archivedPath)
	if err != nil {
		t.Fatalf("read archived SKILL.md: %v", err)
	}
	if string(archivedBody) != v1Body {
		t.Errorf("archived SKILL.md = %q, want %q", archivedBody, v1Body)
	}
}

// TestInstallAutopilot_NoArchiveWhenNoPreExisting covers the
// idempotent path: a fresh autopilot install (no existing SKILL.md)
// must not create an empty .archived/ dir.
func TestInstallAutopilot_NoArchiveWhenNoPreExisting(t *testing.T) {
	tmp := t.TempDir()

	src := filepath.Join(tmp, "src", "autopilot")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(src, "SKILL.md"), "v2 content\n")

	root := filepath.Join(tmp, "home")
	sk := &skill.Skill{Name: "autopilot", Agents: []string{"claude"}}
	if err := Install(src, root, []string{"claude"}, sk); err != nil {
		t.Fatal(err)
	}

	archDir := filepath.Join(root, ".claude", "skills", "autopilot.archived")
	if _, err := os.Stat(archDir); !os.IsNotExist(err) {
		t.Errorf("expected no .archived dir on fresh install; stat err = %v", err)
	}
}

// TestInstallAutopilot_NonAutopilotSkillsSkipArchive covers the
// scope-limit: only autopilot triggers archival. A different skill
// with a pre-existing SKILL.md at its install path goes through
// normal RemoveAll-then-copy without archiving.
func TestInstallAutopilot_NonAutopilotSkillsSkipArchive(t *testing.T) {
	tmp := t.TempDir()

	src := filepath.Join(tmp, "src", "decide")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(src, "SKILL.md"), "v2 decide\n")

	root := filepath.Join(tmp, "home")
	preExistingPath := filepath.Join(root, ".claude", "skills", "decide", "SKILL.md")
	mustWrite(t, preExistingPath, "v1 decide content\n")

	sk := &skill.Skill{Name: "decide", Agents: []string{"claude"}}
	if err := Install(src, root, []string{"claude"}, sk); err != nil {
		t.Fatal(err)
	}

	// No archive sibling for non-autopilot skills.
	archDir := filepath.Join(root, ".claude", "skills", "decide.archived")
	if _, err := os.Stat(archDir); !os.IsNotExist(err) {
		t.Errorf("non-autopilot skill should not produce .archived dir; got stat err = %v", err)
	}
}
