// error_enum.go — v0.10.1 Stage 3 — small helpers for enumerating
// the valid set in error messages. Per the agent-native CLI audit:
// every error rejecting an enum value should name the valid set
// inline so the agent (or human) can self-correct in one retry
// instead of having to read --help or trial-and-error.
package cli

import (
	"sort"
	"strings"

	"github.com/momentmaker/kaijutsu/cli/internal/agents"
)

// listPersonas returns a stable, comma-joined string of persona
// names defined in agents.yaml. Empty map → "(none defined)".
func listPersonas(m map[string]*agents.Persona) string {
	if len(m) == 0 {
		return "(none defined)"
	}
	names := make([]string, 0, len(m))
	for n := range m {
		names = append(names, n)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

// listProviders returns a stable, comma-joined string of enabled
// provider names. Empty map → "(none enabled)".
func listProviders(m map[string]*agents.Provider) string {
	if len(m) == 0 {
		return "(none enabled)"
	}
	names := make([]string, 0, len(m))
	for n := range m {
		names = append(names, n)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}
