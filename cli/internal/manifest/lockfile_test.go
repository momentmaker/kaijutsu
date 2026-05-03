package manifest

import (
	"path/filepath"
	"testing"
)

func TestLockfilePathFieldRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "kaijutsu.lock.json")
	lf := NewLockfile([]string{"claude"})
	v := "1.0.0"
	lf.Skills["pr-review"] = LockEntry{
		Version:   &v,
		Source:    "momentmaker/kaijutsu",
		Ref:       "abc1234",
		Path:      "skills/core/pr-review",
		Integrity: "sha256-deadbeef",
	}
	if err := lf.Save(p); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadLockfile(p)
	if err != nil {
		t.Fatal(err)
	}
	got := loaded.Skills["pr-review"]
	if got.Path != "skills/core/pr-review" {
		t.Errorf("path round-trip: got %q", got.Path)
	}
	if got.Integrity != "sha256-deadbeef" {
		t.Errorf("integrity round-trip: got %q", got.Integrity)
	}
}

func TestLockEntryNullVersionPersists(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "kaijutsu.lock.json")
	lf := NewLockfile([]string{"claude"})
	lf.Skills["sha-pinned"] = LockEntry{
		Version: nil, // SHA-pinned, no semver
		Source:  "owner/repo",
		Ref:     "abc1234",
	}
	if err := lf.Save(p); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadLockfile(p)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Skills["sha-pinned"].Version != nil {
		t.Errorf("expected nil version, got %v", loaded.Skills["sha-pinned"].Version)
	}
}
