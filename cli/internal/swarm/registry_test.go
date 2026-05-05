package swarm

import (
	"strings"
	"testing"
)

func TestPresetRegistry_FindAfterRegister(t *testing.T) {
	r := NewPresetRegistry()
	p := &Preset{Name: "demo", Description: "demo preset"}
	r.Register(p)
	got, err := r.Find("demo")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got.Description != "demo preset" {
		t.Errorf("expected 'demo preset', got %q", got.Description)
	}
}

func TestPresetRegistry_FindMissing(t *testing.T) {
	r := NewPresetRegistry()
	r.Register(&Preset{Name: "alpha"})
	r.Register(&Preset{Name: "beta"})
	_, err := r.Find("zeta")
	if err == nil {
		t.Fatal("expected error for missing preset")
	}
	if !strings.Contains(err.Error(), "alpha") || !strings.Contains(err.Error(), "beta") {
		t.Errorf("error should list available presets, got %q", err)
	}
}

func TestPresetRegistry_NamesSorted(t *testing.T) {
	r := NewPresetRegistry()
	r.Register(&Preset{Name: "zeta"})
	r.Register(&Preset{Name: "alpha"})
	r.Register(&Preset{Name: "mu"})
	got := r.Names()
	want := []string{"alpha", "mu", "zeta"}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("[%d] got %q want %q", i, got[i], w)
		}
	}
}

func TestPresetRegistry_RegisterReplaces(t *testing.T) {
	r := NewPresetRegistry()
	r.Register(&Preset{Name: "x", Description: "v1"})
	r.Register(&Preset{Name: "x", Description: "v2"})
	got, _ := r.Find("x")
	if got.Description != "v2" {
		t.Errorf("expected v2, got %q", got.Description)
	}
}

func TestPresetRegistry_PanicsOnEmptyName(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on empty Name")
		}
	}()
	NewPresetRegistry().Register(&Preset{Name: ""})
}

func TestBuiltinPresets_IncludesPRReview(t *testing.T) {
	got := BuiltinPresets()
	found := false
	for _, p := range got {
		if p.Name == "pr-review" {
			found = true
			if p.InputKind != InputDiff {
				t.Errorf("pr-review should be InputDiff, got %d", p.InputKind)
			}
			if p.cachePathSegment() != "pr-review-runs" {
				t.Errorf("expected default cachePathSegment 'pr-review-runs', got %q", p.cachePathSegment())
			}
		}
	}
	if !found {
		t.Error("pr-review not registered in builtins")
	}
}

func TestPresetFor_BackwardCompat(t *testing.T) {
	// PresetFor was the Phase-1 entry point; it should keep working
	// after the registry refactor for callers that haven't migrated.
	p, err := PresetFor("pr-review")
	if err != nil {
		t.Fatalf("PresetFor pr-review: %v", err)
	}
	if p.Name != "pr-review" {
		t.Errorf("got name %q", p.Name)
	}
}
