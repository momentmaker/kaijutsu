# Changelog

All notable changes to kaijutsu (the registry + skills) and `jutsu` (the CLI). The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses [SemVer](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.6.0] — 2026-05-06

Multi-provider agents. The biggest CLI surface change since v0.4 — `jutsu swarm` is no longer locked to claude/codex/gemini. Driver abstraction lets users plug in any OpenAI-compat HTTP provider (DeepSeek, GLM, Kimi, local Ollama), wrap a native CLI with env overrides via `cli-compat`, or invoke MCP servers as deterministic peers alongside LLMs.

Spec: [`docs/specs/2026-05-05-v0.6.0-multi-provider-agents.md`](docs/specs/2026-05-05-v0.6.0-multi-provider-agents.md).
ADR: [`docs/decisions/2026-05-05-driver-abstraction.md`](docs/decisions/2026-05-05-driver-abstraction.md).

### Added

- **`AgentDriver` interface** in `cli/internal/agents/` — formalizes what a swarm participant is. Four concrete drivers: `cli`, `http`, `cli-compat`, `mcp`.
- **`http` driver** — direct OpenAI-compat + Anthropic-compat HTTP, no SDK dependencies. Anthropic ephemeral cache_control on system block (≥50% cost reduction on cache hit per spec AC). OpenAI prompt-cache awareness via `prompt_tokens_details.cached_tokens`. Both protocols extract real billed cost from response usage.
- **`cli-compat` driver** — wraps a base cli with env override + telemetry-kill defaults. Per-process one-shot warning makes the metadata-leak risk explicit (suppressible via `--no-telemetry-warning`).
- **`mcp` driver (stdio)** — JSON-RPC 2.0 handshake (initialize + notifications/initialized + tools/call) against MCP servers. Free at the API level (Result.CostUSD = 0). Reference stub server in `cli/internal/agents/testdata/stub_mcp_server/`. semgrep-mcp config example in `cli/internal/agents/mcp_examples.md` (doc-only).
- **`agents.yaml` two-layer config** — `~/.kaijutsu/agents.yaml` (global catalog) + `<repo>/.kaijutsu/agents.yaml` (project enabled list + overrides). Schema version 1 enforced. Loader hard-fails on unsupported versions with migration hint.
- **Personas as first-class swarm participants** — 7 built-ins shipped: 3 `default-*` with empty system_prompt (v0.5 cache compat) + 4 reference flavored (`paranoid-security-claude`, `pragmatic-codex`, `architecture-purist-gemini`, `brainstorm-creative-claude`). Auto-synthesis of `default-<provider>` for any enabled provider lacking one. Personas can declare tags (`security`, `architecture`, etc.) for skill `requires_persona_tags` gates.
- **`--personas <name>...` flag** on every swarm subcommand. Dispatches the named personas in parallel; legacy v0.5 path unchanged when flag absent.
- **`--estimate` dry-run** — per-persona cost projection table with input/output token estimates and total. Char-count tokenizer (±20% accuracy per spec D9). Stale rate-card warning at 90+ days. Aggregate budget warning when projected exceeds `--max-cost`.
- **`--no-telemetry-warning` flag** to suppress the cli-compat one-shot stderr warning.
- **`jutsu agent` subcommand group** — full surface:
  - `list [--personas]` — resolved providers + personas in deterministic table.
  - `doctor` — driver-aware health probes: cli `<cmd> --version`, cli-compat (binary + env), http (env + GET `/models`), mcp (Stage 6 stub).
  - `add <name> [--global] [--driver kind ...]` — catalog or non-catalog provider declaration.
  - `enable <name>` / `disable <name>` — toggle project enabled list (config preserved).
  - `remove <name> [--global] [--force]` — hard delete; `--global` honors `JUTSU_REPO_SCAN_ROOTS` env var (colon-separated paths, max-depth 4) for cross-repo reference detection; refuses without `--force` when references found.
  - `test <name>` — driver-aware probe: cli/cli-compat `--version`, http GET `/models` with 1-token completion fallback, mcp deferred.
  - `migrate [--prefer legacy|yaml|merge]` — converts deprecated `kaijutsu.json.agents` to `agents.yaml` `enabled:`. Idempotent on subsequent runs.
- **Vendored provider catalog**: claude/codex/gemini cli + deepseek/glm/kimi/ollama-local http with rate cards (date 2026-05-06). `jutsu agent add deepseek` writes the catalog default; users override via `--driver` flags or YAML edit.
- **`SaveGlobalConfig` + `SaveProjectConfig`** in `cli/internal/agents/load.go` — yaml.v3 round-trip with schema version preservation.
- **`SanitizeForLog`** in `cli/internal/agents/sanitize.go` — redacts known sensitive env values (`*_API_KEY`, `*_TOKEN`, `*_SECRET`, `*_PASSWORD`, plus bare provider key names) from log strings.

### Changed

- **`jutsu swarm <preset>`** — `--personas` is the recommended dispatch mode. Legacy v0.5 path (auto-detect native CLIs) remains the default when `--personas` is absent and produces byte-identical cache keys for backward compat.
- **`InvokeOpts`** gained `ExtraEnv map[string]string` — used by cli-compat driver to inject BASE_URL / API_KEY / telemetry-kill envs on the child process.
- **`Result`** struct gained `Driver DriverKind` + `Err string` + `CacheStatus CacheStatus` — drivers report cost + cache outcome; pipeline overlays real billed cost over the EstimateCostUSD char-count fallback when HTTP driver reports `Result.CostUSD > 0`.
- **`Provider`** type extended with cli-compat (`BaseCLI`, `Env`, `EnvKey`), MCP (`Transport`, `Command`, `Endpoint`, `Headers`, `HeadersLiteral`, `ToolName`, `TimeoutSec`), and `Cost *CostRates` (rate card) fields.
- **`kaijutsu.json.agents`** field is now deprecated in favor of `<repo>/.kaijutsu/agents.yaml` `enabled:` list. `agent migrate` automates the conversion. Removal scheduled for v0.7.

### Deprecated

- `kaijutsu.json.agents` field. Use `agents.yaml enabled:` (or run `jutsu agent migrate`).

### Notes

- **Backward compatibility:** users without `agents.yaml` files get byte-identical v0.5 cache keys. Existing `jutsu swarm pr-review` invocations require zero config changes.
- **Cost/leak/perf trade-offs:** the spec ADR explains why each driver kind exists. `cli-compat` is the path of least resistance for "use deepseek through claude CLI" but inherits Claude Code's harness behavior + leaks metadata via undocumented telemetry endpoints; `http` is the recommended default for non-native providers.
- **What's NOT shipped (deferred to v0.7+):** synthesizer `[deterministic]` tag rendering for MCP findings, MCP info-severity floor, MCP http transport, OpenAI prompt-cache padding to the 1024-token threshold, retry policy, streaming partial findings, quality fingerprinting (SQLite), persona registry, multi-stage swarm pipelines.

## [0.5.0] — 2026-05-05

`jutsu swarm` Phase 2: pluggable presets (pr-review, doc-review, brainstorm, refactor-plan, security-audit) on the same primitive. See `git log v0.4.1..v0.5.0` for the full set.

## [0.4.x] — earlier

See git history. Highlights: `jutsu swarm` primitive (Phase 1 — pr-review only), Sigstore enforcement, permission prompts, hooks as first-class artifacts, cascade-aware `jutsu remove`, `jutsu publish --auto`.
