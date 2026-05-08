// estimate.go — v0.11.0 — moved from cli pkg as part of the
// runSwarmPipeline → swarm.RunPipeline extraction. The cli pkg
// retains a thin wrapper (RunEstimate) that handles cobra-side
// stdout/stderr printing; swarm pkg owns the projection-building
// logic so non-cobra callers can build cost projections directly.
package swarm

import (
	"fmt"

	"github.com/momentmaker/kaijutsu/cli/internal/agents"
)

// EstimateOpts is the structured-args version of the
// commonSwarmFlags subset that BuildEstimateProjections cares about.
// Constructed by the cli wrapper from cobra flags; constructed by
// autopilot / eval directly.
type EstimateOpts struct {
	Personas   []string
	MaxCostUSD float64
}

// BuildEstimateProjections resolves providers and tokenizes prompts
// for every persona / legacy agent, returning one CostProjection per.
// Two sources of providers:
//   - personas mode (Personas non-empty): resolved Provider per persona
//   - legacy mode: built-in cli providers from agents.BuiltinProviders,
//     filtered to AvailableAgents() (matches what the legacy fan-out
//     would dispatch)
func BuildEstimateProjections(projectRoot string, preset *Preset, ictx *InputContext, opts EstimateOpts) ([]agents.CostProjection, error) {
	if len(opts.Personas) > 0 {
		return personaProjections(projectRoot, preset, ictx, opts.Personas)
	}
	return legacyProjections(preset, ictx)
}

func personaProjections(projectRoot string, preset *Preset, ictx *InputContext, personaNames []string) ([]agents.CostProjection, error) {
	global, err := agents.LoadGlobalConfig()
	if err != nil {
		return nil, err
	}
	project, err := agents.LoadProjectConfig(projectRoot)
	if err != nil {
		return nil, err
	}
	resolved, err := agents.Resolve(global, project)
	if err != nil {
		return nil, err
	}

	out := make([]agents.CostProjection, 0, len(personaNames))
	for _, name := range personaNames {
		persona, ok := resolved.Personas[name]
		if !ok {
			return nil, fmt.Errorf("--estimate --personas: persona %q not found. Defined personas: %s", name, ListPersonas(resolved.Personas))
		}
		provider, ok := resolved.Providers[persona.Provider]
		if !ok {
			return nil, fmt.Errorf("--estimate --personas %q: provider %q not enabled. Enabled providers: %s", name, persona.Provider, ListProviders(resolved.Providers))
		}
		// Same override application as persona dispatch — projections
		// reflect the model the actual swarm run uses. Cost rate
		// stays per-provider until v0.7's model-keyed rate cards
		// land; --estimate cost is approximate when persona overrides
		// model to a different tier (e.g. deepseek-v4-pro vs flash).
		provider = agents.ApplyPersonaOverrides(provider, persona)
		// Use the preset's resolved fallback chain so estimate
		// numbers reflect what the actual swarm dispatch would send.
		tmpl, ok := preset.PromptFor(persona.Provider)
		if !ok {
			tmpl = "%s"
		}
		body := fmt.Sprintf(tmpl, ictx.Body)
		fullPrompt := body
		if persona.SystemPrompt != "" {
			fullPrompt = persona.SystemPrompt + UserSentinel + body
		}
		out = append(out, agents.ProjectCost(name, provider, fullPrompt))
	}
	return out, nil
}

func legacyProjections(preset *Preset, ictx *InputContext) ([]agents.CostProjection, error) {
	available := AvailableAgents()
	if len(available) == 0 {
		return nil, nil
	}
	builtins := agents.BuiltinProviders()
	out := make([]agents.CostProjection, 0, len(available))
	for _, name := range available {
		provider, ok := builtins[string(name)]
		if !ok {
			continue
		}
		tmpl, ok := preset.PerAgent[name]
		if !ok {
			continue
		}
		body := fmt.Sprintf(tmpl, ictx.Body)
		out = append(out, agents.ProjectCost("default-"+string(name), provider, body))
	}
	return out, nil
}
