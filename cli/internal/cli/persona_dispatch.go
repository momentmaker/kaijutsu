package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/momentmaker/kaijutsu/cli/internal/agents"
	"github.com/momentmaker/kaijutsu/cli/internal/swarm"
)

// personaAdapter wraps an agents.AgentDriver to satisfy swarm.Agent,
// the legacy v0.5 dispatch interface that swarm.FanOut + swarm.Job
// still consume. The adapter carries the persona's resolved Name so
// downstream stderr / synthesis output references the persona instead
// of the underlying provider.
//
// v0.6 Stage 3b — Stage 5 may collapse this into a unified registry
// once the swarm pipeline migrates fully to agents.AgentDriver.
type personaAdapter struct {
	personaName string
	driver      agents.AgentDriver
}

func (a *personaAdapter) Name() swarm.AgentName {
	return swarm.AgentName(a.personaName)
}

func (a *personaAdapter) Run(ctx context.Context, prompt string, budget float64) (string, error) {
	res, err := a.driver.Invoke(ctx, prompt, agents.InvokeOpts{MaxBudgetUSD: budget})
	return res.Raw, err
}

// assemblePersonaJobs loads agents.yaml resolved config and builds
// one swarm.Job per requested persona. System prompts (when set on a
// persona) are prepended via the userSentinel so HTTP drivers can
// route them into the protocol's first-class system field; cli
// drivers pass the concatenated prompt through unchanged.
//
// Returns ErrPersonaProviderNotEnabled when a persona references a
// provider that isn't in the resolved enabled list (caller surfaces
// to user with a hint to `jutsu agent enable <provider>`).
func assemblePersonaJobs(projectRoot string, preset *swarm.Preset, ictx *swarm.InputContext, personaNames []string) ([]swarm.Job, []*personaAdapter, error) {
	global, err := agents.LoadGlobalConfig()
	if err != nil {
		return nil, nil, fmt.Errorf("load global agents.yaml: %w", err)
	}
	project, err := agents.LoadProjectConfig(projectRoot)
	if err != nil {
		return nil, nil, fmt.Errorf("load project agents.yaml: %w", err)
	}
	resolved, err := agents.Resolve(global, project)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve agents config: %w", err)
	}

	jobs := make([]swarm.Job, 0, len(personaNames))
	adapters := make([]*personaAdapter, 0, len(personaNames))
	for _, name := range personaNames {
		persona, ok := resolved.Personas[name]
		if !ok {
			return nil, nil, fmt.Errorf("--personas: persona %q not found. Run `jutsu agent list --personas` to see available", name)
		}
		provider, ok := resolved.Providers[persona.Provider]
		if !ok {
			return nil, nil, fmt.Errorf("--personas %q: references provider %q which is not enabled. Add to enabled list in .kaijutsu/agents.yaml or run `jutsu agent enable %s`", name, persona.Provider, persona.Provider)
		}
		driver, err := agents.BuildDriver(provider)
		if err != nil {
			return nil, nil, fmt.Errorf("--personas %q: build driver: %w", name, err)
		}

		// Preset prompt-template lookup is by underlying agent type
		// (claude/codex/gemini) — presets ship per-agent prompts in
		// PerAgent[AgentName]; persona names map back to provider
		// names which match the AgentName constants for the three
		// native CLIs. For v0.6 only personas backed by claude /
		// codex / gemini providers find a template; HTTP-only
		// providers (deepseek, glm, etc.) route via their backing
		// AgentName when the persona uses one of the three roles —
		// otherwise this errors with a clear message.
		tmplKey := swarm.AgentName(persona.Provider)
		tmpl, ok := preset.PerAgent[tmplKey]
		if !ok {
			return nil, nil, fmt.Errorf("--personas %q: preset %q has no prompt template for provider %q (preset templates are keyed by claude/codex/gemini). v0.6.x will add per-persona template overrides; for now, persona must back a known agent role", name, preset.Name, persona.Provider)
		}
		body := fmt.Sprintf(tmpl, ictx.Body)

		// Prepend persona system_prompt via the sentinel. HTTP driver
		// splits on this and routes the system half into the
		// protocol's `system` field; cli driver concatenates and
		// sends as-is.
		fullPrompt := body
		if persona.SystemPrompt != "" {
			fullPrompt = persona.SystemPrompt + userSentinel + body
		}

		adapter := &personaAdapter{personaName: name, driver: driver}
		adapters = append(adapters, adapter)
		jobs = append(jobs, swarm.Job{Agent: adapter, Prompt: fullPrompt})
	}
	return jobs, adapters, nil
}

// userSentinel mirrors the constant in agents/http_driver.go. Kept in
// sync via a TestPersonaSentinelMatchesAgentSentinel test (Stage 5+
// will canonicalize this in one place).
const userSentinel = "\n\n<<<USER>>>\n\n"

// pickPersonaSynthesizer returns the swarm.Agent for the synthesizer
// in persona-driven mode. Selection rules:
//   1. If `want` matches a persona name in `personas` whose result has
//      no error, use it.
//   2. Otherwise, the first non-erroring persona in the input order.
//
// Returns nil when no persona has a successful result — caller falls
// back to JSON dump.
func pickPersonaSynthesizer(want string, results []swarm.AgentResult, personas []*personaAdapter) swarm.Agent {
	resultErrByName := map[string]string{}
	for _, r := range results {
		resultErrByName[r.Agent] = r.Err
	}
	if want != "" {
		for _, p := range personas {
			if string(p.Name()) == want && resultErrByName[want] == "" {
				return p
			}
		}
	}
	for _, p := range personas {
		if resultErrByName[string(p.Name())] == "" {
			return p
		}
	}
	return nil
}

// providerLabelsForPersonas surfaces a per-persona consent-prompt
// label list. Used by the consent gate when --personas is set.
func providerLabelsForPersonas(personas []*personaAdapter) []string {
	labels := make([]string, 0, len(personas))
	seen := map[string]bool{}
	for _, p := range personas {
		l := fmt.Sprintf("%s (driver=%s)", p.personaName, p.driver.Driver())
		if seen[l] {
			continue
		}
		seen[l] = true
		labels = append(labels, l)
	}
	return labels
}

// ensureProjectRootForPersonas wraps os.Getwd to produce a usable
// projectRoot for assemblePersonaJobs callers in the swarm pipeline.
// Centralized so the CLI subcommands don't repeat the same Getwd
// fallback logic.
func ensureProjectRootForPersonas(projectRoot string) string {
	if projectRoot != "" {
		return projectRoot
	}
	if cwd, err := os.Getwd(); err == nil {
		return cwd
	}
	return "."
}
