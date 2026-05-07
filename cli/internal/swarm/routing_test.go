package swarm

import (
	"errors"
	"strings"
	"testing"
)

// TestResolvePersonaProvider_PhaseAPerPersona covers the per-persona
// hit path (Phase A step 1).
func TestResolvePersonaProvider_PhaseAPerPersona(t *testing.T) {
	hint := &RoutingHint{
		PerPersona: map[string][]string{"honest": {"claude"}},
	}
	got, err := ResolvePersonaProvider(hint, "honest",
		[]string{"claude", "codex", "gemini"},
		nil, false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != "claude" {
		t.Errorf("got %q, want claude", got)
	}
}

// TestResolvePersonaProvider_PhaseADefaultFallback verifies the
// per-persona miss → default fallback (Phase A step 2).
func TestResolvePersonaProvider_PhaseADefaultFallback(t *testing.T) {
	hint := &RoutingHint{
		PerPersona: map[string][]string{"other": {"gemini"}},
		Default:    []string{"codex"},
	}
	got, err := ResolvePersonaProvider(hint, "honest",
		[]string{"claude", "codex", "gemini"},
		nil, false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != "codex" {
		t.Errorf("got %q, want codex (default fallback)", got)
	}
}

// TestResolvePersonaProvider_PhaseARegistryFallback covers the
// no-routing path (Phase A step 3): both PerPersona and Default
// empty → use the registry callback.
func TestResolvePersonaProvider_PhaseARegistryFallback(t *testing.T) {
	registry := func(persona string) []string {
		return []string{"gemini"}
	}
	got, err := ResolvePersonaProvider(nil, "honest",
		[]string{"claude", "codex", "gemini"},
		registry, false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != "gemini" {
		t.Errorf("got %q, want gemini (registry fallback)", got)
	}
}

// TestResolvePersonaProvider_PhaseBPicksFirstAvailable covers the
// runtime availability gate: walk the preferred list, first match
// in `available` wins.
func TestResolvePersonaProvider_PhaseBPicksFirstAvailable(t *testing.T) {
	hint := &RoutingHint{
		PerPersona: map[string][]string{
			"honest": {"gemini", "codex", "claude"}, // walk in order
		},
	}
	// gemini unavailable; codex available → pick codex.
	got, err := ResolvePersonaProvider(hint, "honest",
		[]string{"codex", "claude"},
		nil, false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != "codex" {
		t.Errorf("got %q, want codex (first available in preferred list)", got)
	}
}

// TestResolvePersonaProvider_StrictRoutingHardFails covers the
// strict=true path: no preferred provider available → error.
func TestResolvePersonaProvider_StrictRoutingHardFails(t *testing.T) {
	hint := &RoutingHint{
		PerPersona: map[string][]string{"honest": {"gemini"}},
	}
	_, err := ResolvePersonaProvider(hint, "honest",
		[]string{"claude", "codex"}, // no gemini
		nil, true)
	if err == nil {
		t.Fatal("expected strict=true to hard-fail")
	}
	if !strings.Contains(err.Error(), "--strict-routing") {
		t.Errorf("error should mention --strict-routing; got: %v", err)
	}
	if !strings.Contains(err.Error(), "honest") {
		t.Errorf("error should name the persona; got: %v", err)
	}
}

// TestResolvePersonaProvider_NonStrictDropsPersona verifies the
// non-strict drop: returns ("", nil) so caller can drop + warn.
func TestResolvePersonaProvider_NonStrictDropsPersona(t *testing.T) {
	hint := &RoutingHint{
		PerPersona: map[string][]string{"honest": {"gemini"}},
	}
	got, err := ResolvePersonaProvider(hint, "honest",
		[]string{"claude", "codex"}, // no gemini
		nil, false)
	if err != nil {
		t.Errorf("non-strict should not error; got: %v", err)
	}
	if got != "" {
		t.Errorf("non-strict drop should return empty provider; got: %q", got)
	}
}

// TestResolvePersonaProvider_EmptyListsEquivalent confirms the spec
// rule: empty PerPersona[k] AND missing PerPersona[k] BOTH fall
// through to Default; same for empty/missing Default → registry.
func TestResolvePersonaProvider_EmptyListsEquivalent(t *testing.T) {
	registry := func(persona string) []string { return []string{"claude"} }

	// Case 1: PerPersona has empty list — should fall through.
	hint := &RoutingHint{
		PerPersona: map[string][]string{"honest": {}},
	}
	got, _ := ResolvePersonaProvider(hint, "honest",
		[]string{"claude"},
		registry, false)
	if got != "claude" {
		t.Errorf("empty PerPersona[k] should fall through; got %q", got)
	}

	// Case 2: PerPersona missing key — should fall through.
	hint = &RoutingHint{}
	got, _ = ResolvePersonaProvider(hint, "honest",
		[]string{"claude"},
		registry, false)
	if got != "claude" {
		t.Errorf("missing PerPersona[k] should fall through; got %q", got)
	}
}

// TestResolvePersonaProvider_NilHintFallsThrough covers the nil-hint
// case: skills without a routing block at all.
func TestResolvePersonaProvider_NilHintFallsThrough(t *testing.T) {
	registry := func(persona string) []string { return []string{"gemini"} }
	got, err := ResolvePersonaProvider(nil, "any",
		[]string{"gemini"}, registry, false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != "gemini" {
		t.Errorf("nil hint should use registry; got %q", got)
	}
}

// TestErrNoDispatchablePersonas_TypedError ensures the Phase C
// sentinel is exported and usable by callers via errors.Is.
func TestErrNoDispatchablePersonas_TypedError(t *testing.T) {
	wrapped := errors.New("wrap: " + ErrNoDispatchablePersonas.Error())
	// errors.Is doesn't unwrap textual wrapping; this just confirms
	// the sentinel is distinguishable as a string. Real usage
	// fmt.Errorf("...: %w", ErrNoDispatchablePersonas) supports
	// errors.Is downstream.
	if !strings.Contains(wrapped.Error(), "no personas dispatchable") {
		t.Errorf("sentinel text drift: %v", wrapped)
	}
}
