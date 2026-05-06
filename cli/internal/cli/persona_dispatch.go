package cli

import (
	"context"
	"fmt"
	"sync"

	"github.com/momentmaker/kaijutsu/cli/internal/agents"
	"github.com/momentmaker/kaijutsu/cli/internal/swarm"
)

// personaAdapter wraps an agents.AgentDriver to satisfy swarm.Agent,
// the legacy v0.5 dispatch interface that swarm.FanOut + swarm.Job
// still consume. The adapter carries the persona's resolved Name so
// downstream stderr / synthesis output references the persona instead
// of the underlying provider.
//
// The adapter ALSO captures the underlying driver's reported
// Result.CostUSD + CacheStatus on each Run so the swarm pipeline can
// recover the real cost (HTTP driver reports billed cost from the
// API's usage block; the legacy parallel.go path falls back to
// EstimateCostUSD which is char-count heuristic). After FanOut the
// pipeline calls overlayPersonaCosts to replace estimates with
// captured values.
//
// v0.6 Stage 3b — Stage 5 may collapse this into a unified registry
// once the swarm pipeline migrates fully to agents.AgentDriver.
type personaAdapter struct {
	personaName string
	driver      agents.AgentDriver

	// last captures the most recent Invoke's Result so the pipeline
	// can recover real cost + cache status. Mutex guards the field
	// because the adapter MAY in principle be reused (e.g. debate
	// pass-2). Per-call atomicity is sufficient — we never read while
	// writing concurrently.
	mu   sync.Mutex
	last agents.Result
}

func (a *personaAdapter) Name() swarm.AgentName {
	return swarm.AgentName(a.personaName)
}

func (a *personaAdapter) Run(ctx context.Context, prompt string, budget float64) (string, error) {
	res, err := a.driver.Invoke(ctx, prompt, agents.InvokeOpts{MaxBudgetUSD: budget})
	a.mu.Lock()
	a.last = res
	a.mu.Unlock()
	return res.Raw, err
}

// LastResult returns the most recent driver Result (cost + cache
// status). Empty when Run has not been called.
func (a *personaAdapter) LastResult() agents.Result {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.last
}

// assemblePersonaJobs loads agents.yaml resolved config and builds
// one swarm.Job per requested persona. System prompts (when set on a
// persona) are prepended via the userSentinel so HTTP drivers can
// route them into the protocol's first-class system field; cli
// drivers pass the concatenated prompt through unchanged.
//
// Errors with a clear hint when a persona is unknown OR when a
// persona references a provider not in the resolved enabled list
// (run `jutsu agent enable <provider>` to fix).
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
		// Persona.Model override (v0.6.2): a persona may override the
		// provider's default model — e.g. a heavyweight reasoning
		// persona pinning to `deepseek-v4-pro` while the rest of the
		// project stays on flash. Clone the provider to avoid
		// mutating the resolved registry that other personas share.
		if persona.Model != "" {
			cloned := *provider
			cloned.Model = persona.Model
			provider = &cloned
		}
		driver, err := agents.BuildDriver(provider)
		if err != nil {
			return nil, nil, fmt.Errorf("--personas %q: build driver: %w", name, err)
		}

		// Preset prompt-template lookup with fallback chain:
		// PerAgent[provider] → DefaultPrompt → PerAgent[AgentClaude].
		// New HTTP providers (deepseek/glm/kimi/anything via
		// `jutsu agent add`) get the preset's DefaultPrompt — the
		// generalist version of the preset's purpose — instead of
		// hard-failing.
		tmpl, ok := preset.PromptFor(persona.Provider)
		if !ok {
			return nil, nil, fmt.Errorf("--personas %q: preset %q has no prompt template for provider %q and no DefaultPrompt set", name, preset.Name, persona.Provider)
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

// stampDriverKind populates AgentResult.Driver from each persona's
// underlying driver. Synthesizer reads this to tag mcp findings as
// [deterministic] and apply the info-severity floor.
func stampDriverKind(results []swarm.AgentResult, personas []*personaAdapter) {
	byName := map[string]*personaAdapter{}
	for _, p := range personas {
		byName[string(p.Name())] = p
	}
	for i, r := range results {
		if p, ok := byName[r.Agent]; ok {
			results[i].Driver = string(p.driver.Driver())
		}
	}
}

// overlayPersonaCosts replaces each AgentResult's estimated Cost with
// the real cost reported by the underlying driver, when available. The
// HTTP driver populates Result.CostUSD from the API's usage block;
// cli drivers report 0 (cost stays as the EstimateCostUSD char-count
// fallback computed in swarm.parallel.FanOut). Mutates results in
// place; safe to call after FanOut completes.
func overlayPersonaCosts(results []swarm.AgentResult, personas []*personaAdapter) {
	byName := map[string]*personaAdapter{}
	for _, p := range personas {
		byName[string(p.Name())] = p
	}
	for i, r := range results {
		p, ok := byName[r.Agent]
		if !ok {
			continue
		}
		last := p.LastResult()
		if last.CostUSD > 0 {
			results[i].Cost = last.CostUSD
		}
	}
}
