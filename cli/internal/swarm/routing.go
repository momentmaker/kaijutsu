// routing.go — v0.9 per-skill provider routing dispatcher.
//
// Two-phase resolution + Phase C invariant. Spec §4 +
// IMPLEMENTATION_PLAN.md Stage 5 are the source of truth.
//
// The cli layer parses skill.yaml's `routing:` block (skill.Routing
// type) and passes the per-persona + default lists to
// ResolvePersonaProvider as plain Go types — keeps swarm package
// from importing the cli/skill package (which itself imports
// nothing from swarm — currently a clean separation we preserve).
package swarm

import (
	"errors"
	"fmt"
)

// RoutingHint is the swarm-package-local representation of a skill's
// `routing:` declaration. The cli layer maps skill.Routing into this
// shape before calling ResolvePersonaProvider so the swarm package
// stays independent of cli/skill.
//
// Empty PerPersona + empty Default = "no routing declared", same
// behavior as Phase A step 3 (registry default fallback).
type RoutingHint struct {
	PerPersona map[string][]string
	Default    []string
}

// ErrNoDispatchablePersonas is the Phase C invariant trigger:
// after Phase B drops every persona under --strict-routing OFF,
// the run can't proceed with zero agents. Hard-fail prevents the
// silent zero-dispatch trap (empty output indistinguishable from
// "no findings").
var ErrNoDispatchablePersonas = errors.New("no personas dispatchable: all preferred providers unavailable")

// ResolvePersonaProvider returns the runnable provider for the given
// persona under the v0.9 3-phase resolution algorithm:
//
//   Phase A — declarative resolution (no availability check):
//     1. RoutingHint.PerPersona[persona] non-empty → use that list
//     2. Else RoutingHint.Default non-empty → use that list
//     3. Else fall back to fallbackRegistry — registry's default
//        mapping for the persona (one entry, the v0.6 default).
//   Phase A always produces a non-empty list.
//
//   Phase B — runtime availability:
//     4. Walk preferred list; first provider in `available` wins.
//     5. None available:
//        - strict=true → return error listing missing providers.
//        - strict=false → return ("", nil); caller drops the
//          persona with a warning.
//
// Phase C (zero-dispatch invariant) is the CALLER's responsibility:
// after iterating personas, if zero survived under strict=false,
// the caller must hard-fail with ErrNoDispatchablePersonas. Phase
// C lives at the caller because it spans the persona loop.
func ResolvePersonaProvider(
	hint *RoutingHint,
	persona string,
	available []string,
	fallbackRegistry func(persona string) []string,
	strict bool,
) (provider string, err error) {
	preferred := phaseAResolve(hint, persona, fallbackRegistry)

	availSet := make(map[string]bool, len(available))
	for _, a := range available {
		availSet[a] = true
	}
	for _, p := range preferred {
		if availSet[p] {
			return p, nil
		}
	}
	if strict {
		return "", fmt.Errorf(
			"routing: persona %q requires preferred provider(s) %v which are unavailable; --strict-routing is set, hard-failing",
			persona, preferred,
		)
	}
	// Non-strict: drop persona, return ("", nil). Caller logs +
	// continues; Phase C invariant catches zero-survivor case.
	return "", nil
}

// phaseAResolve is Phase A (no availability check). Returns the
// preferred-provider list for the persona.
//
// Per spec: empty list and omitted field are equivalent at every
// level. PerPersona[persona] missing OR empty → fall through to
// Default; Default missing OR empty → fall through to registry.
func phaseAResolve(
	hint *RoutingHint,
	persona string,
	fallbackRegistry func(persona string) []string,
) []string {
	if hint != nil {
		if list, ok := hint.PerPersona[persona]; ok && len(list) > 0 {
			return list
		}
		if len(hint.Default) > 0 {
			return hint.Default
		}
	}
	if fallbackRegistry != nil {
		if list := fallbackRegistry(persona); len(list) > 0 {
			return list
		}
	}
	// No routing AND no registry fallback — extremely degenerate
	// (means the persona registry is empty too). Return empty so
	// Phase B trivially fails over to Phase C.
	return nil
}
