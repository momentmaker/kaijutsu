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

// TestRegisterUserPresets_RejectsBuiltinShadow pins spec Decision #2:
// user preset name colliding with a built-in is hard-rejected; the
// built-in is unaffected. Returns (registered, collisions).
func TestRegisterUserPresets_RejectsBuiltinShadow(t *testing.T) {
	r := NewPresetRegistry()
	r.Register(&Preset{Name: "pr-review", DefaultPrompt: "%s"})

	users := map[string]*UserPreset{
		"pr-review": {
			Preset: Preset{Name: "pr-review", DefaultPrompt: "%s"},
			Source: SourceProject,
		},
	}
	registered, collisions := r.RegisterUserPresets(users)
	if len(registered) != 0 {
		t.Errorf("expected 0 registered; got %v", registered)
	}
	if len(collisions) != 1 {
		t.Fatalf("expected 1 collision; got %d", len(collisions))
	}
	if !strings.Contains(collisions[0].Error(), "pr-review") {
		t.Errorf("collision error should name 'pr-review'; got %v", collisions[0])
	}
	// Built-in still findable.
	p, err := r.Find("pr-review")
	if err != nil || p.Name != "pr-review" {
		t.Errorf("built-in should still be findable after rejection; got %v %v", p, err)
	}
}

// TestRegisterUserPresets_RegistersValidEntries pins the happy path.
func TestRegisterUserPresets_RegistersValidEntries(t *testing.T) {
	r := NewPresetRegistry()
	r.Register(&Preset{Name: "pr-review", DefaultPrompt: "%s"})

	users := map[string]*UserPreset{
		"my-tight-review": {
			Preset: Preset{Name: "my-tight-review", DefaultPrompt: "%s"},
			Source: SourceProject,
		},
	}
	registered, collisions := r.RegisterUserPresets(users)
	if len(collisions) != 0 {
		t.Errorf("unexpected collisions: %v", collisions)
	}
	if len(registered) != 1 || registered[0] != "my-tight-review" {
		t.Errorf("registered = %v, want [my-tight-review]", registered)
	}
	p, err := r.Find("my-tight-review")
	if err != nil {
		t.Fatalf("registered preset should be findable: %v", err)
	}
	if p.Name != "my-tight-review" {
		t.Errorf("Name = %q", p.Name)
	}
	if r.UserSource("my-tight-review") != SourceProject {
		t.Errorf("UserSource = %q, want SourceProject", r.UserSource("my-tight-review"))
	}
	if r.UserSource("pr-review") != "" {
		t.Errorf("built-in UserSource should be empty; got %q", r.UserSource("pr-review"))
	}
}

// TestRegisterUserPresets_PartialFailurePreservesValidEntries pins
// that a mix of valid + colliding names registers the valid ones
// while returning errors for the rest.
func TestRegisterUserPresets_PartialFailurePreservesValidEntries(t *testing.T) {
	r := NewPresetRegistry()
	r.Register(&Preset{Name: "pr-review", DefaultPrompt: "%s"})

	users := map[string]*UserPreset{
		"pr-review": {
			Preset: Preset{Name: "pr-review", DefaultPrompt: "%s"},
			Source: SourceProject,
		},
		"my-good-one": {
			Preset: Preset{Name: "my-good-one", DefaultPrompt: "%s"},
			Source: SourceProject,
		},
	}
	registered, collisions := r.RegisterUserPresets(users)
	if len(registered) != 1 || registered[0] != "my-good-one" {
		t.Errorf("registered = %v, want [my-good-one]", registered)
	}
	if len(collisions) != 1 {
		t.Errorf("expected 1 collision; got %d: %v", len(collisions), collisions)
	}
}
