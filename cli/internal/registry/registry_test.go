package registry

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveFromIndex(t *testing.T) {
	idx := &Index{
		Version: 1,
		Skills: map[string]IndexEntry{
			"fancy": {Source: "github.com/someone/fancy-skill", Path: "."},
			"nested": {Source: "owner/repo", Path: "skills/nested"},
		},
	}

	r, err := Resolve(idx, "github.com/momentmaker/kaijutsu", "fancy")
	if err != nil {
		t.Fatal(err)
	}
	if r.Source.Owner != "someone" || r.Source.Repo != "fancy-skill" {
		t.Errorf("got %v", r.Source)
	}
	if r.Path != "" {
		t.Errorf("expected empty path, got %q", r.Path)
	}

	r2, err := Resolve(idx, "momentmaker/kaijutsu", "nested")
	if err != nil {
		t.Fatal(err)
	}
	if r2.Path != "skills/nested" {
		t.Errorf("got path %q", r2.Path)
	}
}

func TestResolveFallsBackToCore(t *testing.T) {
	idx := &Index{Version: 1, Skills: map[string]IndexEntry{}}
	r, err := Resolve(idx, "momentmaker/kaijutsu", "decide")
	if err != nil {
		t.Fatal(err)
	}
	if r.Source.String() != "momentmaker/kaijutsu" || r.Path != "skills/core/decide" {
		t.Errorf("got %v %q", r.Source, r.Path)
	}
}

func TestLoadAndParse(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "index.json")
	body := `{"version": 1, "skills": {"foo": {"source": "owner/repo"}}}`
	os.WriteFile(p, []byte(body), 0644)
	idx, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if idx.Skills["foo"].Source != "owner/repo" {
		t.Errorf("got %+v", idx.Skills)
	}
}

func TestLoadFutureVersionWarnsButAccepts(t *testing.T) {
	// Forward-compat: a registry index from a newer CLI must still parse.
	// Newer CLIs may add fields the current CLI ignores; rejecting outright
	// would prevent older clients from coexisting with newer-format data.
	tmp := t.TempDir()
	p := filepath.Join(tmp, "index.json")
	os.WriteFile(p, []byte(`{"version": 99, "skills": {}}`), 0644)
	idx, err := Load(p)
	if err != nil {
		t.Fatalf("expected forward-compat acceptance, got %v", err)
	}
	if idx.Version != 99 {
		t.Errorf("version round-trip lost: %d", idx.Version)
	}
}

func TestLoadInvalidVersion(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "index.json")
	os.WriteFile(p, []byte(`{"version": 0, "skills": {}}`), 0644)
	if _, err := Load(p); err == nil {
		t.Error("expected error on version 0")
	}
}
