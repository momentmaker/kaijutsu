# Changelog

All notable changes to kaijutsu (the registry + skills) and `jutsu` (the CLI). The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses [SemVer](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.7.0] — 2026-05-06

Quality fingerprinting + confidence-weighted synthesizer. `jutsu swarm` now learns from your accept/dismiss actions: every finding goes into a local SQLite store at `~/.kaijutsu/findings.db`, and the synthesizer weights each agent's vote by its observed precision per (provider, persona, preset, codebase) tuple. The 51st run finally weights gemini's pattern-consistency lens differently from the 50 you've already dismissed.

Spec: [`docs/specs/2026-05-06-v0.7.0-quality-fingerprinting.md`](docs/specs/2026-05-06-v0.7.0-quality-fingerprinting.md).
ADR: [`docs/decisions/2026-05-06-quality-fingerprinting.md`](docs/decisions/2026-05-06-quality-fingerprinting.md).

### Added

- **Local SQLite findings store** at `~/.kaijutsu/findings.db` (mode `0600`, WAL journal). Pure-Go driver via `modernc.org/sqlite` — no CGo, goreleaser cross-compile stays clean. Forward-only migration loader (`cli/internal/findings/migrations/NNNN_*.sql`); `schema_version` table tracks applied migrations. Minimum SQLite version: 3.8 (partial-index syntax `WHERE user_action IS NULL`).
- **Codebase fingerprint** — 5-step resolution chain (`cli/internal/findings/fingerprint.go`): git origin → upstream → alphabetical-first remote → `local-git:<hash>` → `local-fs:<hash>`. Anchors on `(remote, work-tree-basename)` so different fork dirs get distinct fps but identical clones across machines collide.
- **`jutsu finding` subcommand group**:
  - `list [--run <id>] [--pending] [--all-codebases] [--codebase <fp>]` — defaults to most recent run for current cwd's codebase.
  - `accept <id> [--reason "..."] [--cross-codebase]` / `dismiss <id> [--reason "..."] [--cross-codebase]` — cross-codebase guard refuses by default when the targeted finding's `codebase_fp` differs from cwd's.
  - `stats [--codebase <fp>] [--all-codebases]` — per-(provider, persona, preset) precision report. Tuples with < 10 actioned findings render as `(insufficient data)` + `0.70 (bootstrap)` weight; ≥ 10 render the clamped precision. Header includes the current codebase fp for copy-paste into other commands.
  - `clear [--codebase <fp>] [--older-than 90d] [--yes] [--dry-run] [--all-codebases]` — destructive; defaults to dry-run unless `--yes`. Days-suffix duration parser (`90d`, `12h`, `30m`); negative durations rejected so `--older-than -1h` can't resolve to a future cutoff. Runs `VACUUM` after delete.
  - `export <path> [--codebase <fp>] [--all-codebases]` — portable JSON dump with top-level `schema_version: 1` for forward-compat imports.
- **`Weighter.WeightFor(provider, persona, preset, codebaseFp)`** in `cli/internal/findings/weighter.go` — three-state algorithm:
  - Cold-start (DB absent OR 0 actioned for tuple) → `1.0` (byte-identical v0.6.2 behavior).
  - Bootstrap (1 ≤ actioned < 10) → `0.7` (slight skepticism without dismissing).
  - Mature (actioned ≥ 10) → `accepted/(accepted+dismissed)`, clamped to `[0.05, 1.0]`.
  - Sliding window of 200 actioned findings per tuple, measured by `action_at` — drift catches up after ~50 actioned findings against a new model.
- **Synthesizer integration** — `clusterFindings` secondary sort uses `weighted_consensus = sum(unique reporter weights)` (each agent contributes its weight EXACTLY once per cluster, even if it merged multiple findings). Tiebreaker order: weighted_consensus desc → ConsensusOf desc → severity desc → key asc. Cold-start (no weights data) collapses byte-for-byte to v0.6.2 ordering.
- **`--show-weights`** flag on every swarm subcommand. Off by default in v0.7 — adopt weights via `jutsu finding stats` first; v0.7.x or v0.8 may flip the default.
- **Synthesizer prompt `weights:` section** — emitted only when at least one weight differs from 1.0. Cold-start prompts are byte-identical to v0.6.2.
- **Best-effort recording** — DB write failures emit a single stderr warning line and the swarm pipeline continues. Quality fingerprinting is non-critical; a corrupt or read-only DB never blocks the markdown render.
- **`KAIJUTSU_FINDINGS_DB` env override** — used by tests for isolation; user-facing override via `--db` flag deferred to v0.7.x.

### Changed

- **`swarm.Synthesize` + `SynthesizeWithDebate` signatures** — both now take a `SynthOpts` struct (Weights + ShowWeights). Empty SynthOpts reproduces v0.6.2 behavior byte-for-byte. `--replay` passes empty SynthOpts so cached output stays reproducible across weight updates.
- **`personaAdapter`** gains `providerName` so the recorder + weighter can resolve provider per persona without re-loading the agents registry.

### Privacy

- **No network calls** from any `jutsu finding` subcommand. Enforced by an import-list test (`finding_test.go`): `cli/internal/cli/finding.go` MUST NOT import `net/*`, bare `net`, or `golang.org/x/net/*`. Catches both intentional telemetry and accidental drag-ins.
- **`jutsu finding clear`** wipes targeted rows + runs `VACUUM` to reclaim disk. `jutsu finding export` writes local JSON only — never POSTs anywhere.
- **`~/.kaijutsu/findings.db`** lives in `$HOME`, not in any repo, never reachable from git history. Mode `0600` re-asserted on every Open.

### Notes

- **Cold-start UX**: synthesis behavior is byte-identical to v0.6.2 for users with no findings DB OR with a fresh DB that hasn't accumulated any actioned findings yet. The feature delivers value after ~10 actioned findings per tuple — `jutsu finding stats` shows progress toward that threshold.
- **`--full` mode caveat**: the recorder hook fires after Pass-1 fan-out, before `swarm.Debate`. Pass-2 `[new]`/`[disputes]`/`[agreed]` revisions don't get DB rows in v0.7. Weight math is unaffected (agent identity is preserved across passes); summary text divergence is the visible cost. Move scheduled for v0.7.x.
- **Performance**: `BenchmarkWeightFor` against a 50K-row DB returns in ~80μs per lookup on an Apple M3 (spec budget < 5ms).
- **Schema lock-in mitigated** by the migration framework + forward-only design + ADR captures the rationale.

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
