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
// The registry is safe for concurrent reads after init; writes only
// happen during package init (and during tests via Register).
type PresetRegistry struct {
	mu      sync.RWMutex
	presets map[string]*Preset
}

// defaultRegistry holds the package-level built-ins. Tests can build
// fresh registries via NewPresetRegistry; production code uses the
// default via PresetFor.
var defaultRegistry = NewPresetRegistry()

// NewPresetRegistry returns an empty registry. Useful for tests that
// want to assert behavior without polluting the default registry.
func NewPresetRegistry() *PresetRegistry {
	return &PresetRegistry{presets: map[string]*Preset{}}
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
}
