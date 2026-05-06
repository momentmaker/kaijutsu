# ADR: Driver abstraction for swarm participants

**Date:** 2026-05-05
**Status:** Accepted (pending implementation in v0.6.0)
**Spec:** [`docs/specs/2026-05-05-v0.6.0-multi-provider-agents.md`](../specs/2026-05-05-v0.6.0-multi-provider-agents.md)
**Author:** rubberduck

## Context

`jutsu swarm` (shipped in v0.4 / generalized in v0.5) hard-codes three native CLIs: `claude`, `codex`, `gemini`. Adding any other provider (DeepSeek, GLM, Kimi, local Ollama, static analyzers) requires either:

1. Forcing users to set `ANTHROPIC_BASE_URL` to point Claude Code at a different model — leaks metadata to Anthropic via telemetry endpoints that don't respect the override, and inherits Claude Code's harness behavior (which underperforms with smaller models per benchmarks).
2. Asking the user to install a new CLI per provider — most providers don't have one.
3. Forking the swarm code per integration — combinatorial explosion.

The existing `swarm.Agent` interface in `cli/internal/swarm/agent.go` already treats agents as opaque "give-prompt → get-raw-output" boxes. Generalizing this is the natural next step.

## Decision

Introduce an `AgentDriver` interface in a new `cli/internal/agents/` package with **four** concrete drivers shipped in v0.6:

| Driver | Mechanism | Use case |
|---|---|---|
| `cli` | shell out to native CLI | claude, codex, gemini (existing v0.5 behavior) |
| `http` | speak openai-compat / anthropic-compat protocol via Go `net/http` | deepseek, glm, kimi, ollama, any compat endpoint |
| `cli-compat` | wrap a native CLI with `BASE_URL`+`KEY` override + telemetry-disable env | when the user wants claude's prompt-engineering with a different model |
| `mcp` | invoke a configured MCP server's `analyze` tool | static analyzers (semgrep, eslint) as deterministic swarm peers |

The interface is intentionally minimal: `Name()`, `Driver()`, `Invoke(ctx, prompt, opts)`. No streaming, no batching, no per-call retry — those belong in higher layers (or are deferred to v0.7).

### Provider configuration moves into a YAML file

Two-layer config replaces the hard-coded list:

- **Global catalog** at `~/.kaijutsu/agents.yaml` — provider definitions (URL, model, key env name) shared across every project.
- **Project config** at `<repo>/.kaijutsu/agents.yaml` — `enabled:` list + project-only overrides + project-specific personas. Checked into the repo.
- **Secrets** never in YAML; always env-var indirection (`api_key_env: DEEPSEEK_API_KEY`).

### Personas are first-class

A persona = (provider, optional model override, system prompt, tags). The `--personas` flag dispatches personas, not raw providers. Personas are the unit of swarm participant identity — what gets recorded in cache keys, what skills can require via `requires_persona_tags`. Built-in `default-claude` / `default-codex` / `default-gemini` reproduce v0.5 behavior.

## Why a driver interface, not per-call protocol switching?

Three reasons:

1. **Existing extension point.** The swarm pipeline already treats agents as opaque boxes. We're not introducing a new abstraction; we're naming an implicit one and letting it carry more weight.
2. **Drivers have genuinely different cost/leak/perf profiles.** `cli` calls a binary on the user's machine with no metadata leak; `http` is a direct API call that costs measured tokens; `cli-compat` is `cli` plus an apology for the metadata leakage; `mcp` is deterministic and free. Pretending these are interchangeable hides the trade-offs from the user. Naming each one explicitly forces UX decisions (warnings, cost reporting, telemetry kill-envs) into the right place.
3. **Future driver shapes are cleaner with a stable interface.** v0.7+ wants plugin drivers (third-party `.so` or sidecar process). v0.8+ wants live-registry providers. A protocol-switch in v0.6 would have to be ripped out for either.

## Why HTTP-direct instead of always going through a CLI?

For non-native providers (DeepSeek, GLM, etc.) the choice is:

- **HTTP-direct** (chosen): Go's `net/http`, no CLI install, no telemetry surface beyond the API itself, stable JSON output without `--output-format` negotiation, prompt-cache-aware via Anthropic `cache_control` and OpenAI prefix-padding, cost tracking trivial (read `usage` from the response).
- **CLI-as-harness** (rejected as default; available as `cli-compat` opt-in): inherits Claude Code's tool-use behavior (problematic with smaller models per HN/terminal-bench data), telemetry endpoints leak metadata to Anthropic regardless of `ANTHROPIC_BASE_URL`, requires installing the harness CLI everywhere.

The HN thread that prompted this design discussion (item 48002136 about DeepClaude) is itself evidence: the most-upvoted technical comment in that thread was about Claude-Code-as-harness *underperforming* other harnesses. So we ship `cli-compat` as the escape hatch with a runtime warning, but `http` is the default for new providers.

## Why MCP-as-peer instead of MCP-as-tool?

MCP servers are typically integrated as tools that an LLM can call mid-conversation. We're using them differently: an MCP server is a **swarm peer** — a deterministic alternative reviewer. When semgrep + claude + codex independently flag the same line at issue severity, that's near-certain real signal. The synthesizer benefits from a non-LLM voice in the disagreement table.

The `analyze(args: {preset, severity_vocab, input}) → findings[]` contract is intentionally narrow: one tool, one purpose, one output shape. We're not building a generic MCP-tool-orchestration framework; we're letting deterministic analyzers participate in the swarm.

## Backward compatibility

v0.5 users get byte-identical behavior with no config changes:

- No `agents.yaml` files present → loader synthesizes `enabled: [claude, codex, gemini]` from a hardcoded fallback.
- `kaijutsu.json.agents` (legacy) still read with a deprecation warning.
- Cache keys for the v0.5 default mix (cli driver, default personas with empty system_prompt) match v0.5's keys byte-for-byte. Any deviation (new driver, persona with system prompt, model override) salts a new key by design.

## Consequences

### Positive

- Pluggable provider model without tying the registry to one runtime.
- Zero-cost `mcp` peers expand swarm signal quality without per-call spend.
- Persona shape sets up v0.7 persona-as-installable-artifact.
- Prompt-cache awareness unlocks 50%+ cost cut for iterative review loops.
- Cleaner mental model: "swarm participant" stops meaning "Anthropic/OpenAI/Google CLI" and starts meaning "anything that implements `AgentDriver`."

### Negative

- Three new drivers in one release is a meaningful scope increase. Stage 1's pure-refactor framing is the mitigation: the interface lands first with cli driver, then drivers stack on incrementally.
- `cli-compat` exists but is the path of least resistance for users with a single native CLI installed. We mitigate by ordering driver choices in docs and `agent add` UX as `http > cli > cli-compat`, plus a runtime warning when `cli-compat` runs.
- Provider rate cards go stale; we ship a fixture-refresh script and a CI cron, but `--estimate` projections will drift between releases.
- More YAML for users to learn. Mitigated by `jutsu agent add <name>` doing the writing; users only edit YAML when they want fine control.

### Neutral / deferred

- Streaming output is a non-goal in v0.6; deferred to v0.7. Real consequence: `--full` debate mode still has the 30s-of-silence UX we have today.
- Per-provider retry policy deferred to v0.7. v0.6 surfaces 429/5xx as `Result.Err` and continues.
- Plugin drivers deferred to v0.7+. v0.6 ships exactly the four named drivers; third-party drivers wait for a stable plugin API.

## Alternatives considered

1. **Single protocol shim, no driver kinds.** Always treat providers as HTTP regardless of source. Rejected: loses the cli driver's value (no API key needed, no token cost reporting from the CLI's stderr) and forces every provider through HTTP even when a CLI is the better path.
2. **Plugin drivers in v0.6.** Ship only the interface; defer all four reference drivers to plugins. Rejected: nothing to demo on day one, and the four reference drivers are the proof that the interface is well-designed. Save the plugin layer for after we have ground-truth on what drivers actually need.
3. **Skip `cli-compat` entirely.** Force users wanting "claude harness with deepseek model" to choose http instead. Rejected: there are real reasons to want claude's prompt engineering even at the cost of the metadata leak (e.g. comparing model behaviors under identical harness). Ship the option with a warning rather than ban it.
4. **MCP servers as tools (not peers).** Let LLMs call MCP servers mid-conversation. Rejected: doesn't solve the problem we have. We want a deterministic *peer* in the swarm, not another tool an LLM might or might not call. The peer pattern is what makes static analysis findings comparable with LLM findings.

## Implementation note

The spec's 7-stage build sequence is intentionally backward-compat-first:

- Stage 1 is pure refactor (cli driver only, all v0.5 tests pass).
- Stage 2 adds YAML loading + personas without enabling any new drivers.
- Stages 3-6 each add one driver kind incrementally.
- Stage 7 is docs/release.

If Stage 6 (MCP) hits unexpected complexity, it can slip to v0.6.x without holding back the main feature; Stages 1-5 are the core.

## Review

This ADR was authored alongside the v0.6.0 spec, which went through 5 rounds of `jutsu swarm doc-review` (full-mode, claude+gemini reviewers). Convergence reached at zero issue-level findings; remaining single-reviewer minors are conversation-starters.
