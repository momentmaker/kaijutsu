package skill

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSynthesizeFromSKILLMD_HappyPath(t *testing.T) {
	tmp := t.TempDir()
	skillMD := filepath.Join(tmp, "SKILL.md")
	licenseFile := filepath.Join(tmp, "LICENSE")

	body := `---
name: vanilla-skill
description: A skill that ships only SKILL.md, no skill.yaml.
---

# Vanilla skill

Body content.
`
	os.WriteFile(skillMD, []byte(body), 0644)
	os.WriteFile(licenseFile, []byte(`MIT License

Copyright (c) 2026 Some Author

Permission is hereby granted...`), 0644)

	sk, err := SynthesizeFromSKILLMD(skillMD, licenseFile)
	if err != nil {
		t.Fatal(err)
	}
	if sk.Name != "vanilla-skill" {
		t.Errorf("name = %q; want %q", sk.Name, "vanilla-skill")
	}
	if sk.Description == "" {
		t.Error("description is empty")
	}
	if sk.License != "MIT" {
		t.Errorf("license = %q; want MIT", sk.License)
	}
	if sk.Permissions.Bash || sk.Permissions.Network {
		t.Errorf("permissions should default to false, got %+v", sk.Permissions)
	}
	if v := sk.Permissions.FsWrite; v != false {
		t.Errorf("fs-write should default to false, got %v", v)
	}
}

func TestSynthesizeFromSKILLMD_RejectsNonAllowlistedLicense(t *testing.T) {
	tmp := t.TempDir()
	skillMD := filepath.Join(tmp, "SKILL.md")
	licenseFile := filepath.Join(tmp, "LICENSE")
	os.WriteFile(skillMD, []byte("---\nname: x-skill\ndescription: d\n---\nbody"), 0644)
	os.WriteFile(licenseFile, []byte("GPLv3\n... copyleft text ..."), 0644)
	if _, err := SynthesizeFromSKILLMD(skillMD, licenseFile); err == nil {
		t.Error("expected error on non-allowlisted license")
	}
}

func TestSynthesizeFromSKILLMD_DetectsApache(t *testing.T) {
	tmp := t.TempDir()
	skillMD := filepath.Join(tmp, "SKILL.md")
	licenseFile := filepath.Join(tmp, "LICENSE")
	os.WriteFile(skillMD, []byte("---\nname: ap-skill\ndescription: d\n---\nbody"), 0644)
	os.WriteFile(licenseFile, []byte(`Apache License
Version 2.0, January 2004
http://www.apache.org/licenses/`), 0644)
	sk, err := SynthesizeFromSKILLMD(skillMD, licenseFile)
	if err != nil {
		t.Fatal(err)
	}
	if sk.License != "Apache-2.0" {
		t.Errorf("license = %q; want Apache-2.0", sk.License)
	}
}

func TestSynthesizeFromSKILLMD_RejectsMissingFrontmatter(t *testing.T) {
	tmp := t.TempDir()
	skillMD := filepath.Join(tmp, "SKILL.md")
	os.WriteFile(skillMD, []byte("# Just a heading, no frontmatter"), 0644)
	if _, err := SynthesizeFromSKILLMD(skillMD, ""); err == nil {
		t.Error("expected error on missing frontmatter")
	}
}

func TestSynthesizeFromSKILLMD_RejectsBadName(t *testing.T) {
	tmp := t.TempDir()
	skillMD := filepath.Join(tmp, "SKILL.md")
	os.WriteFile(skillMD, []byte("---\nname: BadCamelCase\ndescription: d\n---\nbody"), 0644)
	if _, err := SynthesizeFromSKILLMD(skillMD, ""); err == nil {
		t.Error("expected error on non-kebab-case name")
	}
}
