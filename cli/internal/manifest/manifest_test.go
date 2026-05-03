package manifest

import (
	"path/filepath"
	"testing"
)

func TestManifestRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "kaijutsu.json")
	m := New([]string{"claude", "codex"})
	m.Dependencies["foo"] = "^1.0"
	if err := m.Save(p); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Version != SchemaVersion {
		t.Errorf("version = %d, want %d", loaded.Version, SchemaVersion)
	}
	if loaded.Dependencies["foo"] != "^1.0" {
		t.Errorf("dependency lost: %+v", loaded.Dependencies)
	}
	if len(loaded.Agents) != 2 {
		t.Errorf("expected 2 agents, got %v", loaded.Agents)
	}
}

func TestLockfileRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "kaijutsu.lock.json")
	lf := NewLockfile([]string{"claude"})
	v := "1.0.0"
	lf.Skills["foo"] = LockEntry{Version: &v, Source: "x/y", Ref: "abc", Integrity: "sha256-..."}
	if err := lf.Save(p); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadLockfile(p)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Skills["foo"].Ref != "abc" {
		t.Errorf("ref lost: %+v", loaded.Skills["foo"])
	}
	if *loaded.Skills["foo"].Version != "1.0.0" {
		t.Errorf("version lost: %+v", loaded.Skills["foo"])
	}
}

func TestManifestValidate(t *testing.T) {
	m := New([]string{"claude"})
	if err := m.Validate(); err != nil {
		t.Errorf("expected valid, got %v", err)
	}
	bad := &Manifest{Version: 1}
	if err := bad.Validate(); err == nil {
		t.Errorf("expected invalid (no agents), got nil")
	}
}

func TestLockfileValidate(t *testing.T) {
	lf := NewLockfile([]string{"claude"})
	if err := lf.Validate(); err != nil {
		t.Errorf("empty lockfile should validate, got %v", err)
	}
	v := "1.0.0"
	lf.Skills["foo"] = LockEntry{Version: &v} // missing source + ref
	if err := lf.Validate(); err == nil {
		t.Errorf("expected error for missing source/ref")
	}
	bad := &Lockfile{Version: 99}
	if err := bad.Validate(); err == nil {
		t.Errorf("expected error for bad version")
	}
}
