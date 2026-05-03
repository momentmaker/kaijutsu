package lint

import (
	"os"
	"path/filepath"
	"testing"
)

func writeSkill(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		full := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
}

const validSkillYaml = `name: demo
version: 1.0.0
license: MIT
layout: flat
description: "Demo skill."
agents: [claude]
permissions:
  bash: false
  network: false
  fs-write: false
`

const validSkillMd = `---
name: demo
description: Demo skill.
---
body
`

func TestLintHappyPath(t *testing.T) {
	dir := t.TempDir()
	writeSkill(t, dir, map[string]string{
		"skill.yaml": validSkillYaml,
		"SKILL.md":   validSkillMd,
	})
	r, err := Lint(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Issues) != 0 {
		t.Errorf("expected no issues, got %v", r.Issues)
	}
	if r.HasErrors() {
		t.Errorf("expected clean, has errors")
	}
}

func TestLintMismatchedName(t *testing.T) {
	dir := t.TempDir()
	writeSkill(t, dir, map[string]string{
		"skill.yaml": validSkillYaml,
		"SKILL.md": `---
name: not-demo
description: x
---
body
`,
	})
	r, err := Lint(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !r.HasErrors() {
		t.Errorf("expected error for name mismatch, got %v", r.Issues)
	}
}

func TestLintMissingSkillMd(t *testing.T) {
	dir := t.TempDir()
	writeSkill(t, dir, map[string]string{
		"skill.yaml": validSkillYaml,
	})
	r, err := Lint(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !r.HasErrors() {
		t.Errorf("expected error for missing SKILL.md")
	}
}

func TestLintRejectsIncompatibleLicense(t *testing.T) {
	dir := t.TempDir()
	yaml := `name: demo
version: 1.0.0
license: GPL-3.0
layout: flat
description: x
agents: [claude]
permissions:
  bash: false
  network: false
  fs-write: false
`
	writeSkill(t, dir, map[string]string{
		"skill.yaml": yaml,
		"SKILL.md":   validSkillMd,
	})
	r, err := Lint(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !r.HasErrors() {
		t.Errorf("expected license rejection, got %v", r.Issues)
	}
}

func TestLintMissingFrontmatter(t *testing.T) {
	dir := t.TempDir()
	writeSkill(t, dir, map[string]string{
		"skill.yaml": validSkillYaml,
		"SKILL.md":   "no frontmatter here\n",
	})
	r, err := Lint(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !r.HasErrors() {
		t.Errorf("expected frontmatter error")
	}
}
