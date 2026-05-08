// persona_dispatch.go — v0.11.0 — moved from cli pkg as part of the
// runSwarmPipeline → swarm.RunPipeline extraction. Exposes the
// persona-driven dispatch primitives so non-cobra callers (eval,
// autopilot) can invoke swarm pipelines without going through the
// cli package.
package swarm

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/momentmaker/kaijutsu/cli/internal/agents"
)

// PersonaAdapter wraps an agents.AgentDriver to satisfy swarm.Agent,
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
// pipeline calls OverlayPersonaCosts to replace estimates with
// captured values.
type PersonaAdapter struct {
	PersonaName string
	// ProviderName is the underlying provider (e.g. "claude", "deepseek")
	// the persona dispatches through. Distinct from driver kind: two
	// personas may share a driver kind (cli) but route to different
	// providers (claude vs gemini). v0.7 recorder uses this to populate
	// the `provider` column so per-tuple weight math can scope by
	// (provider, persona) without re-resolving the registry.
	ProviderName string
	Driver       agents.AgentDriver

	// last captures the most recent Invoke's Result so the pipeline
	// can recover real cost + cache status. Mutex guards the field
	// because the adapter MAY in principle be reused (e.g. debate
	// pass-2). Per-call atomicity is sufficient — we never read while
	// writing concurrently.
	mu   sync.Mutex
	last agents.Result
}

func (a *PersonaAdapter) Name() AgentName {
	return AgentName(a.PersonaName)
}

func (a *PersonaAdapter) Run(ctx context.Context, prompt string, budget float64) (string, error) {
	res, err := a.Driver.Invoke(ctx, prompt, agents.InvokeOpts{MaxBudgetUSD: budget})
	a.mu.Lock()
	a.last = res
	a.mu.Unlock()
	return res.Raw, err
}

// LastResult returns the most recent driver Result (cost + cache
// status). Empty when Run has not been called.
func (a *PersonaAdapter) LastResult() agents.Result {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.last
}

// AssemblePersonaJobs loads agents.yaml resolved config and builds
// one swarm.Job per requested persona. System prompts (when set on a
// persona) are prepended via the UserSentinel so HTTP drivers can
// route them into the protocol's first-class system field; cli
// drivers pass the concatenated prompt through unchanged.
//
// Errors with a clear hint when a persona is unknown OR when a
// persona references a provider not in the resolved enabled list
// (run `jutsu agent enable <provider>` to fix).
func AssemblePersonaJobs(projectRoot string, preset *Preset, ictx *InputContext, personaNames []string) ([]Job, []*PersonaAdapter, error) {
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

	jobs := make([]Job, 0, len(personaNames))
	adapters := make([]*PersonaAdapter, 0, len(personaNames))
	for _, name := range personaNames {
		persona, ok := resolved.Personas[name]
		if !ok {
			return nil, nil, fmt.Errorf("--personas: persona %q not found. Defined personas: %s. (Or run `jutsu agent list --personas`.)", name, ListPersonas(resolved.Personas))
		}
		provider, ok := resolved.Providers[persona.Provider]
		if !ok {
			return nil, nil, fmt.Errorf("--personas %q: references provider %q which is not enabled. Enabled providers: %s. Add to .kaijutsu/agents.yaml or run `jutsu agent enable %s`", name, persona.Provider, ListProviders(resolved.Providers), persona.Provider)
		}
		// Persona override application — currently just Model; future
		// fields (per-persona timeout, header overrides) extend
		// agents.ApplyPersonaOverrides. Clone-on-override prevents
		// bleeding into other personas backed by the same provider.
		provider = agents.ApplyPersonaOverrides(provider, persona)
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
			fullPrompt = persona.SystemPrompt + UserSentinel + body
		}

		adapter := &PersonaAdapter{PersonaName: name, ProviderName: persona.Provider, Driver: driver}
		adapters = append(adapters, adapter)
		jobs = append(jobs, Job{Agent: adapter, Prompt: fullPrompt})
	}
	return jobs, adapters, nil
}

// UserSentinel mirrors the constant in agents/http_driver.go. Kept in
// sync via a TestPersonaSentinelMatchesAgentSentinel test.
const UserSentinel = "\n\n<<<USER>>>\n\n"

// PickPersonaSynthesizer returns the swarm.Agent for the synthesizer
// in persona-driven mode. Selection rules:
//  1. If `want` matches a persona name in `personas` whose result has
//     no error, use it.
//  2. Otherwise, the first non-erroring persona in the input order.
//
// Returns nil when no persona has a successful result — caller falls
// back to JSON dump.
func PickPersonaSynthesizer(want string, results []AgentResult, personas []*PersonaAdapter) Agent {
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

// StampDriverKind populates AgentResult.Driver from each persona's
// underlying driver. Synthesizer reads this to tag mcp findings as
// [deterministic] and apply the info-severity floor.
func StampDriverKind(results []AgentResult, personas []*PersonaAdapter) {
	byName := map[string]*PersonaAdapter{}
	for _, p := range personas {
		byName[string(p.Name())] = p
	}
	for i, r := range results {
		if p, ok := byName[r.Agent]; ok {
			results[i].Driver = string(p.Driver.Driver())
		}
	}
}

// OverlayPersonaCosts replaces each AgentResult's estimated Cost with
// the real cost reported by the underlying driver, when available. The
// HTTP driver populates Result.CostUSD from the API's usage block;
// cli drivers report 0 (cost stays as the EstimateCostUSD char-count
// fallback computed in swarm.parallel.FanOut). Mutates results in
// place; safe to call after FanOut completes.
func OverlayPersonaCosts(results []AgentResult, personas []*PersonaAdapter) {
	byName := map[string]*PersonaAdapter{}
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

// ListPersonas returns a stable, comma-joined string of persona
// names defined in agents.yaml. Empty map → "(none defined)".
// Mirrors cli/error_enum.go::listPersonas — kept here too to avoid
// the swarm pkg depending on cli pkg.
func ListPersonas(m map[string]*agents.Persona) string {
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

// ListProviders returns a stable, comma-joined string of enabled
// provider names. Empty map → "(none enabled)".
func ListProviders(m map[string]*agents.Provider) string {
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
