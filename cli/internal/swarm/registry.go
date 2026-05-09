package swarm

import (
	"fmt"
	"sort"
	"sync"
)

// PresetRegistry holds the set of known presets. Phase 2 Stage 1
// migrates from the hardcoded `switch` in PresetFor to a registry-
// backed lookup so Stages 3–7 can add presets without touching
// PresetFor's body. Built-in presets register themselves via
// init(); skill-loaded prompts overlay onto the registered preset
// at lookup time via LoadPresetWithSkillOverrides.
//
// v0.13.0 Preset SDK (RegisterUserPresets) layers user-defined
// presets on top of the built-ins; user presets are stored alongside
// built-ins in the same `presets` map but tracked in a parallel
// `userSources` map for help-text rendering provenance.
//
// The registry is safe for concurrent reads after init; writes only
// happen during package init (and during tests via Register).
type PresetRegistry struct {
	mu          sync.RWMutex
	presets     map[string]*Preset
	userSources map[string]UserPresetSource // v0.13: name → user:project | user:home; absent for built-ins
}

// defaultRegistry holds the package-level built-ins. Tests can build
// fresh registries via NewPresetRegistry; production code uses the
// default via PresetFor.
var defaultRegistry = NewPresetRegistry()

// NewPresetRegistry returns an empty registry. Useful for tests that
// want to assert behavior without polluting the default registry.
func NewPresetRegistry() *PresetRegistry {
	return &PresetRegistry{
		presets:     map[string]*Preset{},
		userSources: map[string]UserPresetSource{},
	}
}

// Register adds (or replaces) a preset by Name. Returns the registry
// for chaining. Panics if p.Name is empty — that's a bug in the
// caller, not a runtime condition.
func (r *PresetRegistry) Register(p *Preset) *PresetRegistry {
	if p == nil || p.Name == "" {
		panic("swarm.PresetRegistry.Register: preset must be non-nil and have a Name")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.presets[p.Name] = p
	return r
}

// Find returns the preset registered under name, or an error listing
// the available preset names. The error is intended to be surfaced
// directly to the user.
func (r *PresetRegistry) Find(name string) (*Preset, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if p, ok := r.presets[name]; ok {
		return p, nil
	}
	return nil, fmt.Errorf("unknown preset %q. Available: %s", name, formatNames(r.namesLocked()))
}

// Names returns the registered preset names sorted alphabetically.
// Used by `jutsu swarm --help` for preset discovery.
func (r *PresetRegistry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.namesLocked()
}

func (r *PresetRegistry) namesLocked() []string {
	out := make([]string, 0, len(r.presets))
	for n := range r.presets {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

func formatNames(names []string) string {
	if len(names) == 0 {
		return "(none registered)"
	}
	out := names[0]
	for _, n := range names[1:] {
		out += ", " + n
	}
	return out
}

// RegisterUserPresets adds the given user presets to the registry.
// Hard-errors on built-in name collision per spec Decision #2.
//
// Returns:
//   - registered: names of user presets that registered cleanly
//   - collisions: errors for entries that collided with built-ins
//     (or with each other across project/home — though
//     LoadUserPresets's project-shadows-home rule means same-name
//     cross-file collisions resolve to one entry before reaching
//     here)
//
// Caller iterates `registered` to add cobra subcommands; never
// string-matches against `collisions` to filter what's safe to
// register. This eliminates the brittle error-string-matching
// pattern flagged by plan doc-review.
func (r *PresetRegistry) RegisterUserPresets(m map[string]*UserPreset) (registered []string, collisions []error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Stable iteration order so test assertions on `registered`
	// don't depend on map-iteration order.
	names := make([]string, 0, len(m))
	for n := range m {
		names = append(names, n)
	}
	sort.Strings(names)

	for _, name := range names {
		up := m[name]
		// Only built-ins existed in the registry pre-call (or other
		// already-registered user presets — possible if RegisterUserPresets
		// is called multiple times in a single run; treat that the same as
		// built-in collision).
		if _, exists := r.presets[name]; exists {
			collisions = append(collisions, fmt.Errorf("user preset %q collides with an already-registered preset (built-ins: %s); pick a different name", name, formatNames(builtinNamesLocked(r))))
			continue
		}
		// Register the embedded Preset; track source separately.
		preset := up.Preset // shallow copy to avoid pinning the *UserPreset
		r.presets[name] = &preset
		r.userSources[name] = up.Source
		registered = append(registered, name)
	}
	return registered, collisions
}

// UserSource returns the source of a user-registered preset (or
// empty string for built-ins / unknown names). Used by cobra
// help-text rendering to tag user presets with [user:project] or
// [user:home].
func (r *PresetRegistry) UserSource(name string) UserPresetSource {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.userSources[name]
}

// builtinNamesLocked returns the registered preset names whose
// source is NOT user (i.e. registered via init's Register calls).
// Caller MUST hold r.mu.
func builtinNamesLocked(r *PresetRegistry) []string {
	out := make([]string, 0)
	for name := range r.presets {
		if _, isUser := r.userSources[name]; !isUser {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// PresetFor preserves the Phase-1 entry point but is now backed by
// the default registry. Existing call sites stay unchanged; new
// call sites can use the registry directly when they need to
// enumerate presets.
func PresetFor(name string) (*Preset, error) {
	return defaultRegistry.Find(name)
}

// BuiltinPresets returns the built-in presets in registration order
// (sorted alphabetically). Stage-1 returns only pr-review; Stages
// 3–7 add doc-review, brainstorm, refactor-plan, security-audit.
func BuiltinPresets() []*Preset {
	names := defaultRegistry.Names()
	out := make([]*Preset, 0, len(names))
	for _, n := range names {
		p, err := defaultRegistry.Find(n)
		if err == nil {
			out = append(out, p)
		}
	}
	return out
}

func init() {
	defaultRegistry.Register(&prReviewPreset)
	defaultRegistry.Register(&docReviewPreset)
	defaultRegistry.Register(&brainstormPreset)
	defaultRegistry.Register(&refactorPlanPreset)
	defaultRegistry.Register(&securityAuditPreset)
	defaultRegistry.Register(&dreamPreset)
	defaultRegistry.Register(&reversePreset)
	// v0.12.0 Tier A presets
	defaultRegistry.Register(&testGapPreset)
	defaultRegistry.Register(&bugReproPreset)
	defaultRegistry.Register(&codeArchaeologyPreset)
}
