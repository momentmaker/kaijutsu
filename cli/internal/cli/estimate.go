package cli

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/momentmaker/kaijutsu/cli/internal/agents"
	"github.com/momentmaker/kaijutsu/cli/internal/swarm"
)

// runEstimate prints a per-persona cost projection table and returns
// without invoking any agent. Reused by every swarm subcommand under
// the --estimate flag.
//
// Two sources of providers:
//   - personas mode (--personas): resolved Provider per persona
//   - legacy mode: built-in cli providers from agents.BuiltinProviders,
//     filtered to swarm.AvailableAgents() (matches what the legacy
//     fan-out would dispatch)
//
// Output is deterministic (sorted by persona name) so users can grep
// it in scripts; same input → same output.
func runEstimate(stdout, stderr io.Writer, projectRoot string, preset *swarm.Preset, ictx *swarm.InputContext, f commonSwarmFlags) error {
	projections, err := buildEstimateProjections(projectRoot, preset, ictx, f)
	if err != nil {
		return err
	}
	if len(projections) == 0 {
		return fmt.Errorf("--estimate: no providers resolved (legacy mode requires at least one of claude/codex/gemini installed; personas mode requires --personas with valid names)")
	}

	// Sort for deterministic output.
	sort.Slice(projections, func(i, j int) bool {
		return projections[i].PersonaName < projections[j].PersonaName
	})

	fmt.Fprintf(stdout, "%-30s %-12s %6s %6s  %s\n", "PERSONA", "DRIVER", "IN_TOK", "OUT_TOK", "COST")
	fmt.Fprintln(stdout, strings.Repeat("-", 78))
	var total float64
	missingRateCard := 0
	for _, cp := range projections {
		fmt.Fprintln(stdout, agents.FormatProjection(cp))
		if cp.HasRateCard {
			total += cp.CostUSD
		} else {
			missingRateCard++
		}
	}
	fmt.Fprintln(stdout, strings.Repeat("-", 78))
	fmt.Fprintf(stdout, "%-30s %-12s %6s %6s  $%.4f total\n", "TOTAL", "", "", "", total)

	worstAccuracy := agents.EstimateAccuracyTokenizerPct
	for _, cp := range projections {
		if cp.AccuracyPct > worstAccuracy {
			worstAccuracy = cp.AccuracyPct
		}
	}
	tokenizerLabel := "tiktoken-go"
	if worstAccuracy >= agents.EstimateAccuracyFallbackPct {
		tokenizerLabel = "tiktoken-go for openai-compat / char-count fallback for the rest"
	}
	footer := fmt.Sprintf("\nestimate: %s (±%d%% worst case across rows); output assumed at %d tokens.",
		tokenizerLabel, worstAccuracy, agents.OutputTokensEstimate)
	if missingRateCard > 0 {
		footer += fmt.Sprintf(" %d provider(s) missing rate card → cost shown as 0.", missingRateCard)
	}
	if f.maxCostUSD > 0 && total > f.maxCostUSD {
		fmt.Fprintf(stderr, "\nwarning: projected $%.4f exceeds --max-cost $%.4f%s\n", total, f.maxCostUSD, footer)
	} else {
		fmt.Fprintln(stdout, footer)
	}
	return nil
}

// buildEstimateProjections resolves providers and tokenizes prompts
// for every persona / legacy agent, returning one CostProjection per.
func buildEstimateProjections(projectRoot string, preset *swarm.Preset, ictx *swarm.InputContext, f commonSwarmFlags) ([]agents.CostProjection, error) {
	if len(f.personas) > 0 {
		return personaProjections(projectRoot, preset, ictx, f.personas)
	}
	return legacyProjections(preset, ictx)
}

func personaProjections(projectRoot string, preset *swarm.Preset, ictx *swarm.InputContext, personaNames []string) ([]agents.CostProjection, error) {
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
			return nil, fmt.Errorf("--estimate --personas: persona %q not found. Defined personas: %s", name, listPersonas(resolved.Personas))
		}
		provider, ok := resolved.Providers[persona.Provider]
		if !ok {
			return nil, fmt.Errorf("--estimate --personas %q: provider %q not enabled. Enabled providers: %s", name, persona.Provider, listProviders(resolved.Providers))
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
			fullPrompt = persona.SystemPrompt + userSentinel + body
		}
		out = append(out, agents.ProjectCost(name, provider, fullPrompt))
	}
	return out, nil
}

func legacyProjections(preset *swarm.Preset, ictx *swarm.InputContext) ([]agents.CostProjection, error) {
	available := swarm.AvailableAgents()
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
