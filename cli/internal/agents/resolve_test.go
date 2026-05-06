package agents

import (
	"sort"
	"strings"
	"testing"
)

func TestResolve_LegacyMixWhenBothEmpty(t *testing.T) {
	r, err := Resolve(&GlobalConfig{}, &ProjectConfig{})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"claude", "codex", "gemini"}
	got := append([]string(nil), r.Enabled...)
	sort.Strings(got)
	sort.Strings(want)
	if !equalStrings(got, want) {
		t.Errorf("Enabled = %v, want %v (legacy v0.5 mix)", got, want)
	}
	for _, name := range want {
		if _, ok := r.Providers[name]; !ok {
			t.Errorf("Providers missing %q", name)
		}
	}
}

func TestResolve_BuiltinPersonasRegistered(t *testing.T) {
	r, err := Resolve(&GlobalConfig{}, &ProjectConfig{})
	if err != nil {
		t.Fatal(err)
	}
	wantNames := []string{
		"default-claude", "default-codex", "default-gemini",
		"paranoid-security-claude", "pragmatic-codex",
		"architecture-purist-gemini", "brainstorm-creative-claude",
	}
	for _, name := range wantNames {
		p, ok := r.Personas[name]
		if !ok {
			t.Errorf("built-in persona %q not registered", name)
			continue
		}
		if p.Name != name {
			t.Errorf("persona %q has Name = %q", name, p.Name)
		}
	}
	// Default personas must have empty system prompt — preserves
	// v0.5 cache-key compat.
	for _, n := range []string{"default-claude", "default-codex", "default-gemini"} {
		if r.Personas[n].SystemPrompt != "" {
			t.Errorf("default persona %q has non-empty system_prompt; cache-key compat broken", n)
		}
	}
}

func TestResolve_AutoSynthesizeDefaultPersona(t *testing.T) {
	// User declares deepseek as enabled but doesn't define a
	// default-deepseek persona. Resolver should auto-synthesize.
	global := &GlobalConfig{
		Providers: map[string]*Provider{
			"deepseek": {Name: "deepseek", Driver: DriverHTTP, Protocol: "openai-compat", BaseURL: "https://api.deepseek.com/v1", Model: "ds-1"},
		},
	}
	project := &ProjectConfig{Enabled: []string{"deepseek"}}
	r, err := Resolve(global, project)
	if err != nil {
		t.Fatal(err)
	}
	p, ok := r.Personas["default-deepseek"]
	if !ok {
		t.Fatal("default-deepseek not auto-synthesized")
	}
	if p.Provider != "deepseek" {
		t.Errorf("default-deepseek.Provider = %q, want %q", p.Provider, "deepseek")
	}
	if p.SystemPrompt != "" {
		t.Errorf("default-deepseek.SystemPrompt = %q, want empty", p.SystemPrompt)
	}
}

func TestResolve_ProjectOverridesGlobal(t *testing.T) {
	global := &GlobalConfig{
		Providers: map[string]*Provider{
			"deepseek": {Name: "deepseek", Driver: DriverHTTP, Model: "deepseek-coder", BaseURL: "https://api.deepseek.com/v1"},
		},
	}
	project := &ProjectConfig{
		Enabled:   []string{"deepseek"},
		Overrides: map[string]*Provider{"deepseek": {Model: "deepseek-reasoner"}},
	}
	r, err := Resolve(global, project)
	if err != nil {
		t.Fatal(err)
	}
	if got := r.Providers["deepseek"].Model; got != "deepseek-reasoner" {
		t.Errorf("Model = %q, want override %q", got, "deepseek-reasoner")
	}
	// Non-overridden field must survive.
	if got := r.Providers["deepseek"].BaseURL; got != "https://api.deepseek.com/v1" {
		t.Errorf("BaseURL not preserved through merge: %q", got)
	}
}

func TestResolve_ProjectPersonaOverridesGlobalThenBuiltin(t *testing.T) {
	global := &GlobalConfig{
		Personas: map[string]*Persona{
			"paranoid-security-claude": {Provider: "claude", SystemPrompt: "global override"},
		},
	}
	project := &ProjectConfig{
		Personas: map[string]*Persona{
			"paranoid-security-claude": {Provider: "claude", SystemPrompt: "project override"},
		},
	}
	r, err := Resolve(global, project)
	if err != nil {
		t.Fatal(err)
	}
	got := r.Personas["paranoid-security-claude"].SystemPrompt
	if got != "project override" {
		t.Errorf("SystemPrompt = %q, want project override (full replacement, not merge)", got)
	}
}

func TestResolve_EnabledNotInAnyLayerErrors(t *testing.T) {
	project := &ProjectConfig{Enabled: []string{"nonexistent-provider"}}
	_, err := Resolve(&GlobalConfig{}, project)
	if err == nil {
		t.Fatal("expected error for unknown enabled provider; got nil")
	}
}

func TestResolve_MissingEnvKeyReported(t *testing.T) {
	t.Setenv("DEEPSEEK_API_KEY_TEST", "") // ensure unset
	global := &GlobalConfig{
		Providers: map[string]*Provider{
			"deepseek": {Name: "deepseek", Driver: DriverHTTP, BaseURL: "https://x", APIKeyEnv: "DEEPSEEK_API_KEY_TEST"},
		},
	}
	project := &ProjectConfig{Enabled: []string{"deepseek"}}
	r, err := Resolve(global, project)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.MissingKeys) != 1 || r.MissingKeys[0] != "deepseek" {
		t.Errorf("MissingKeys = %v, want [deepseek]", r.MissingKeys)
	}
}

func TestResolve_GlobalProvidersWithoutEnabled_FallsBackToCatalog(t *testing.T) {
	// Global declares deepseek, no project enabled. Resolver should
	// fall back to enabling all global providers (alphabetical).
	global := &GlobalConfig{
		Providers: map[string]*Provider{
			"deepseek": {Name: "deepseek", Driver: DriverHTTP, BaseURL: "https://x", Model: "x"},
		},
	}
	r, err := Resolve(global, &ProjectConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Enabled) != 1 || r.Enabled[0] != "deepseek" {
		t.Errorf("Enabled = %v, want [deepseek]", r.Enabled)
	}
}

func TestResolve_RejectsUnknownDriverKind(t *testing.T) {
	global := &GlobalConfig{
		Providers: map[string]*Provider{
			"weird": {Name: "weird", Driver: "clil"}, // typo
		},
	}
	project := &ProjectConfig{Enabled: []string{"weird"}}
	_, err := Resolve(global, project)
	if err == nil {
		t.Fatal("expected error for unknown driver kind; got nil")
	}
	if !strings.Contains(err.Error(), "unknown driver kind") {
		t.Errorf("error = %v; want substring 'unknown driver kind'", err)
	}
}

func TestResolve_RejectsMissingDriverKind(t *testing.T) {
	global := &GlobalConfig{
		Providers: map[string]*Provider{
			"naked": {Name: "naked"}, // no Driver
		},
	}
	project := &ProjectConfig{Enabled: []string{"naked"}}
	_, err := Resolve(global, project)
	if err == nil {
		t.Fatal("expected error for missing driver kind; got nil")
	}
	if !strings.Contains(err.Error(), "no driver kind") {
		t.Errorf("error = %v; want substring 'no driver kind'", err)
	}
}

// TestMergeProvider_DeepCopiesMaps locks in the invariant that
// mergeProvider deep-copies map fields from src. Without this, a
// project Override map shared between two enabled providers (or
// re-used across resolutions) would alias the resolved provider's
// state and a later mutation would leak back.
func TestMergeProvider_DeepCopiesMaps(t *testing.T) {
	src := &Provider{
		Env:    map[string]string{"FOO": "bar"},
		EnvKey: map[string]string{"VAR": "ENV"},
		Headers: map[string]string{"H": "V"},
		HeadersLiteral: map[string]string{"L": "M"},
	}
	dst := &Provider{}
	mergeProvider(dst, src)

	// Mutate src AFTER merge — dst must not observe.
	src.Env["FOO"] = "MUTATED"
	src.EnvKey["VAR"] = "MUTATED"
	src.Headers["H"] = "MUTATED"
	src.HeadersLiteral["L"] = "MUTATED"

	if dst.Env["FOO"] != "bar" {
		t.Errorf("dst.Env aliased src.Env: got %q, want %q", dst.Env["FOO"], "bar")
	}
	if dst.EnvKey["VAR"] != "ENV" {
		t.Errorf("dst.EnvKey aliased src.EnvKey: got %q, want %q", dst.EnvKey["VAR"], "ENV")
	}
	if dst.Headers["H"] != "V" {
		t.Errorf("dst.Headers aliased src.Headers: got %q, want %q", dst.Headers["H"], "V")
	}
	if dst.HeadersLiteral["L"] != "M" {
		t.Errorf("dst.HeadersLiteral aliased src.HeadersLiteral: got %q, want %q", dst.HeadersLiteral["L"], "M")
	}
}

// TestMergeProvider_DeepCopiesCost confirms the same invariant for
// the *CostRates pointer field.
func TestMergeProvider_DeepCopiesCost(t *testing.T) {
	src := &Provider{Cost: &CostRates{InputPerMtok: 1.0}}
	dst := &Provider{}
	mergeProvider(dst, src)
	src.Cost.InputPerMtok = 99.0
	if dst.Cost.InputPerMtok != 1.0 {
		t.Errorf("dst.Cost aliased src.Cost: got %v, want 1.0", dst.Cost.InputPerMtok)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
