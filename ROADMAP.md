# Roadmap

## v0.1 — first public release ✅

Working consume-and-author loop. No bells, no eval gate, no static site.

- [x] Repo scaffolding, LICENSE, README, CONTRIBUTING, SECURITY
- [x] JSON schemas: skill, lockfile, project manifest
- [x] Domain placeholders for `kaijutsu.dev` and `kaijutsu.org`
- [x] `docs/multi-agent.md` — confirmed install paths for Claude, Codex, Gemini
- [x] `jutsu` CLI MVP: `init`, `install`, `list`, `remove`
- [x] Lockfile + `upgrade` + remote source resolution via GitHub tarball API
- [x] 9 seed skills under `skills/core/`: `pr-review`, `readme-update`, `decide`, `journal`, `polish`, `unstuck`, `scope-check`, `session-retro`, `agent-doctor`
- [x] `publish`, `lint`, `search`, `info` commands
- [x] Sigstore signing workflow for `skills/core/` on release tag (sign-core.yml)
- [x] Goreleaser pipeline for jutsu binaries + Homebrew tap on release tag (release-jutsu.yml)
- [x] `https://kaijutsu.dev/install.sh` published from /docs (GitHub Pages)
- [x] `v0.1.0` tag pushed; release + sign workflows fired
- [x] `momentmaker/homebrew-tap` bootstrapped, formula auto-published

## v0.2 — composability + adversarial review

- [x] Adversarial `pr-review` (5x blunder hunt, posts inline PR comments via `gh`, idempotency markers)
- [x] Skill composability via `deps.skills` — recursive install with cycle detection, `installedAs` lockfile field for orphan tracking
- [x] 7 new primitive skills: `blunder-hunt`, `lie-to-them`, `convergence-detect`, `deslop`, `dispatch-parallel`, `multi-model-synth`, `project-memory`
- [x] Every existing skill elevated with substantive content deltas (parallel modes, multi-model fan-out, cost-tier classification, memory-first patterns, etc.)
- [x] JSON sidecar pattern documented in SCHEMA.md
- [x] Cross-skill memory contract via `project-memory` skill
- [x] Skill-evals skeleton (`docs/skill-evals.md`) — runner deferred to v0.3
- [x] Schema forward-compat — older CLIs accept newer manifest/lockfile/registry versions instead of hard-rejecting
- [x] `SECURITY.md` — threat model, reporting flow, current safeguards, known stubs

## v0.3 — trust + automation

- [x] **Sigstore enforcement**: hard-fails `jutsu install` on signature mismatch when `expected-signer` is set. `dcg` skill opted in starting v0.3.1; `--no-verify` override for environments without cosign
- [x] **Permission prompts**: install-time prompt when a skill declares sensitive `permissions` (`bash: true`, `network: true`, `fs-write: full`) instead of advisory-only. `--yes` bypass.
- [x] **Hooks as first-class artifacts**: extend `skill.yaml` with a `hooks` block; install translates and registers hooks into `~/.<agent>/settings.json`; remove cleans them up. First user: a destructive-command-guard (DCG) hook bundle.
- [x] **`jutsu eval`**: shipped v0.10.0 (single-skill parity with agent-skills-eval upstream + 3 kaijutsu-native swarm-shape extensions: persona, preset, swarm-skill). Stateful `--strict` via `--baseline-from <git-ref>` against prior tag's `eval-baseline.json`. CI workflow `eval-skills.yml` runs as harness gate (Go tests + cobra wiring + JSON parse-check). v0.10.1 polish patch added `--dry-run` to install/upgrade/remove/publish + error-message enumeration sweep. v0.11.0 replaced the v0.10 Stage 2 stubs (preset / swarm-skill) with real `swarm.RunPipeline` integration; the extraction also enables autopilot v2.
- [x] **`autopilot` v2 skill**: shipped v0.11.0. Distributed via `jutsu install autopilot`. 6-phase intent-to-PR pipeline (BRAINSTORM → SPEC → PLAN → BUILD → SHIP → LEARN), 2 gates (post-brainstorm + GitHub PR review), multi-agent adversarial review at every artifact stage, anti-sycophancy via `jutsu swarm dream` at brainstorm, reverse-drift gate before PR opens, layered cost cap ($20 soft / $100 hard ceiling / per-shell env override). Replaces the user-scope v1 skill at `~/.claude/skills/autopilot`; install pipeline archives any pre-existing SKILL.md to a sibling `.archived/` dir. New `jutsu autopilot init|status|abort|resume|run` cobra group.
- [x] **3 new built-in personas** (v0.11.0): `claim-auditor-claude`, `cross-file-gemini`, `perf-purist-codex`. CLI-backed; available without HTTP API keys.
- [x] **`agent-doctor` rich-layout port**: ships `scripts/{lib,doctor,cleanup}.sh`, `references/{directory-map,cleanup-tiers,protected-paths}.md`, `runbooks/recover-from-trash.md`. Generalized for ~/.claude, ~/.codex, ~/.gemini, ~/.agents.
- [x] **Cascade-aware `jutsu remove`**: dynamically computes the dep graph by walking each installed skill's on-disk `skill.yaml`. Refuses to remove a skill another depends on; `--cascade` removes orphans transitively.
- [x] **`jutsu publish` automation**: `--auto` shells out to `gh` for fork + clone + branch + push + PR. Default still prints manual steps.
- [x] **Trigger-conflict lint**: `jutsu lint` reports cross-skill overlaps in trigger phrases (slash commands + quoted phrases) extracted from descriptions. `jutsu list --conflicts` checks installed skills.
- [x] **Reconcile skill version vs registry tag display + lockfile schema**: lockfile gains a `tag` field separate from `version`. `jutsu list` shows skill internal version + tag side-by-side; `jutsu info` adds a `tag:` row. `recordInstall` now stores the skill's `skill.yaml` version as `Version` (not the resolved tag's semver). `loadByLockEntry` plumbs the tag back through so sync-time sigstore verify can fire. Forward-compat: older lockfiles with no `tag` field still load; verify gracefully no-ops with a warning.
- [ ] **GitHub API rate-limit handling**: surface a clearer error + `GITHUB_TOKEN` hint when 403-rate-limited. install.sh already supports `GITHUB_TOKEN`; the `jutsu` CLI should too.
- [ ] **Static skill catalog at `kaijutsu.dev`**: searchable index, agent-compat matrix, install copy-buttons; auto-built from the registry on every release

## v0.4 — multi-agent flagship + community + telemetry

- [x] **`jutsu swarm` primitive + multi-agent `pr-review`** (Phase 1): orchestrates claude/codex/gemini in parallel with tailored per-agent prompts, optional Pass-2 round-robin debate (`--full`), lie-to-them filter on synthesis (`--strict`), disagreement-table output, edit-in-place PR comment via `gh` (`--post-comment`), pre-flight secrets scan, per-repo consent gate (`.kaijutsu/pr-review.yaml` `allow-multi-model: true`), `--replay <sha>` from cache. pr-review skill bumps to v1.0.0 rich layout with overrideable per-agent prompts.
- [ ] **`jutsu swarm` Phase 2**: pluggable presets — `brainstorm`, `refactor-plan`, `security-audit` on the same primitive.
- [ ] `skills/community/` opens for PRs (auto-merge on green CI + maintainer approval)
- [ ] CODEOWNERS by category for trusted maintainers
- [ ] Anonymized opt-in telemetry: install counts, error rates, skill pairings → drives bundle recommendations
- [ ] `jutsu translate` — convert a skill from one agent's format to another (best-effort)
- [x] `jutsu eval` (deferred from v0.3) — shipped v0.10.0; see v0.3 entry above for details.
- [ ] `git-commit` and `reorient` skills (deferred from v0.1)

## v0.6 — multi-provider agents (driver abstraction) ✅

Spec: `docs/specs/2026-05-05-v0.6.0-multi-provider-agents.md`. ADR: `docs/decisions/2026-05-05-driver-abstraction.md`.

- [x] **`AgentDriver` interface** + 4 concrete drivers: `cli` (claude/codex/gemini singletons), `http` (OpenAI-compat + Anthropic-compat with prompt-cache awareness), `cli-compat` (wraps a native CLI with env override + telemetry-kill), `mcp` (JSON-RPC stdio against MCP servers).
- [x] **`agents.yaml` two-layer config**: `~/.kaijutsu/agents.yaml` (global catalog) + `<repo>/.kaijutsu/agents.yaml` (project enabled list + overrides). `version: 1` schema.
- [x] **Personas as first-class**: 7 built-ins (3 default-* with empty system_prompt for v0.5 cache compat + 4 reference flavored: paranoid-security-claude, pragmatic-codex, architecture-purist-gemini, brainstorm-creative-claude). Auto-synth of `default-<provider>` for any enabled provider lacking one.
- [x] **`--personas` flag** wired into all 5 swarm subcommands (pr-review, doc-review, brainstorm, refactor-plan, security-audit). Personas dispatched in parallel, system prompts prepended via sentinel split by HTTP driver into protocol's first-class system field.
- [x] **`--estimate` dry-run**: char-count tokenizer (±20% accuracy), per-persona cost projection table, TOTAL row, stale-rate-card warning at 90+ days.
- [x] **`--no-telemetry-warning`** suppression flag for the cli-compat one-shot warning.
- [x] **`jutsu agent` subcommand group**: list (with `--personas`), doctor (driver-aware probes: cli `--version`, http `/models`, mcp stub), add (catalog + non-catalog paths), enable, disable, remove (with cross-repo scan via `JUTSU_REPO_SCAN_ROOTS`), test (driver-aware: cli/cli-compat `--version`, http `/models` + 1-token completion fallback), migrate (kaijutsu.json → agents.yaml with `--prefer legacy|yaml|merge`).
- [x] **Vendored provider catalog**: claude/codex/gemini cli + deepseek/glm/kimi/ollama-local http with rate cards.
- [x] **Stub MCP server** in `cli/internal/agents/testdata/stub_mcp_server/main.go` for driver tests; semgrep-mcp config doc-only in `cli/internal/agents/mcp_examples.md`.
- [x] **Cache-key compat**: legacy v0.5 mix produces byte-identical keys; non-default mix salted with driver+persona identity.

## v0.7 — quality fingerprinting (the long-term moat) ✅

- [x] **Quality fingerprinting + confidence-weighted synthesizer**: local SQLite at `~/.kaijutsu/findings.db` records user accept/dismiss per (provider, persona, preset, codebase-fingerprint) tuple. Synthesizer weights agent votes by observed precision via 3-state algo (cold=1.0 / bootstrap=0.7 / mature=clamped precision) over a sliding 200-action window. `jutsu finding {list,accept,dismiss,stats,clear,export}` CLI surface. `--show-weights` opt-in for synthesizer column annotations. No-network privacy boundary enforced by import-list test.

## v0.7.x — incremental followups

- [x] **`--db` flag** for `jutsu finding *` (shipped v0.8.3).
- [ ] **Configurable bootstrap weight + window size** — `~/.kaijutsu/findings.toml` once we have real-world tuning data.
- [ ] **Auto-prune policy** — `prune.toml` with default-365d eviction; v0.7 ships manual `clear --older-than` only.
- [ ] **`jutsu finding repair`** — vacuum + integrity check + best-effort recovery from corrupt DB.
- [ ] **Rule-based skip + skill-author skip-policy schema**: deterministic heuristic gate (lockfile/comment-only/whitespace auto-skip), `skip_rules:` in agents.yaml.
- [ ] **MCP http transport** (deferred from v0.6 Stage 6).
- [ ] **`--estimate` provider-native tokenizers** (tiktoken-go fallback already tight at ±5% for openai-compat).

## v0.8 — pre-implementation interrogation primitive ✅

- [x] **v0.8.0 — `dream` skill + `swarm dream` preset** — pre-implementation interrogation through 4-8 cognitive lenses (4 base: honest/fit/gaps/wild + 4 opt-in extras: adversary/inverse/status-quo/time). Anti-sycophancy gates baked into every prompt. Standalone `/dream` walks lenses sequentially; `jutsu swarm dream` dispatches the N×lenses matrix and synthesizes into 4 sections (load-bearing / cross-lens consensus / lens-unique / lens-blind-spots warning). Records to v0.7 findings DB via `[lens:<name>]` summary prefix; no schema migration. `--lenses` flag controls the lens set; default `--max-cost` raised to 3.00 (5.00 with `--lenses=all`). `--mode full` rejected until v0.8.x defines a dream-debate template.
- [x] **v0.8.1 — patch + curation** — dream Wild lens `%!s(MISSING)` fix. Core skills tier curated 28 → 17; 6 to community (code-simplification, security-and-hardening, journal, session-retro, readme-update, dcg); 5 deleted (decide, agent-doctor, multi-model-synth, lie-to-them, project-memory). Cleaner first-party canon for fresh users + agents discovering kaijutsu.
- [x] **v0.8.2 — agent-first design lens + 4 discoverability features** — `agent-first, human-friendly` codified in AGENTS.md as the default design lens. New helpers + commands: `output.AutoFormat()` (TTY/pipe auto-flip pattern), `jutsu describe` (JSON catalog of full CLI surface for fresh agents), `jutsu suggest <task>` (keyword-rank skills against a task description), `jutsu init` AGENTS.md fragment (idempotent marker block teaching fresh agents how to use jutsu). `docs/project-memory.md` restored as schema convention doc.
- [x] **v0.8.3 — quick-wins bundle (7 items)** — `--db` flag for `jutsu finding *`, `jutsu dream` subcommand group (list / clear), dream graveyard auto-write, synthesizer-coda truncation post-processor (programmatic backstop to HARD STOP rule), runtime lens-prefix validation (recorder skips malformed dream rows), per-file anti-sycophancy regression test for skill markdown, recorder hook moves below Debate (--full mode now records merged Pass-1⊕Pass-2). Closes most of the v0.7.x + v0.8.x backlog.

## v0.8.x — committed targets

- [ ] **Anonymized opt-in telemetry** — share aggregated `(provider, persona, preset, precision)` tuples to a community dashboard. Requires consent flow + scrubbing protocol — separate spec + ADR.
- [ ] **PR-comment auto-detection** — `gh pr comments` parsing for `accept`/`dismiss` markers in the kaijutsu-pr-review block; closes the loop without requiring CLI subcommand.
- [ ] **Multi-machine sync** — optional sync of `findings.db` across user's machines via user-configured backend (S3 / gist / syncthing). User-managed, not jutsu-hosted.
- [ ] **Cross-codebase learning** — opt-in: weight a new codebase's tuples by similar codebases (same language signature). Privacy-sensitive — needs design.
- [ ] **Multi-stage swarm pipelines**: `jutsu swarm pipeline brainstorm-then-audit` chains presets with structured handoff; new `pipeline.yaml` DSL.
- [ ] **Live TUI with streaming partial findings**: each provider streams findings as generated; depends on streaming support landing in `http_driver`.
- [x] **Reverse swarm — spec-vs-impl drift detector**: shipped v0.9.0 (`jutsu swarm reverse --spec/--diff`; ADDED/OMITTED/CHANGED/AMBIGUOUS categories; deterministic oversize truncation; distinct marker prefix). Combined trigger + draft-PR softer-header deferred to v0.9.x.
- [ ] **Persona registry + `jutsu install persona:foo`**: personas as installable artifacts.
- [x] **Per-skill provider routing**: shipped v0.9.0 (`skill.yaml` `routing:` field + 3-phase resolver; `KAIJUTSU_DISABLE_AGENTS` escape hatch). `routing.disabled` user override deferred to v0.9.x.
- [x] **Recorder hook moves below Debate** — shipped v0.8.3.
- [x] **Dream-specific schema migration** — shipped v0.9.0 (`0002_lens.sql` adds `lens TEXT` + `position INTEGER`; LIKE-based backfill; `WeightForLens` 3-tier fallback; synthesizer reads per-lens weights via `SynthOpts.LensWeights`; `KAIJUTSU_DREAM_ADAPTIVE_LENS=off` killswitch).
- [x] **Dream-flavored Pass-2 debate** — shipped v0.9.0 (`[new]/[disputes]/[revised]/[agreed]` revision tags; cobra reject lifted; cost prompt + non-TTY hard-fail).
- [x] **Dream graveyard auto-write** — shipped v0.8.3 + v0.9.0 (lens-rotation rule on repeat-within-7d landed v0.9.0 alongside Pass-2).
- [x] **PR-comment auto-detection** — shipped v0.9.0 (`jutsu finding sync-pr <pr>` reply-keyword channel; dry-run default; idempotent re-run; `gh` subprocess timeout). Reaction channel + `--post-review` mode + `--auto-sync` deferred to v0.9.x.
- [x] **`jutsu dream clear --older-than 365d`** — shipped v0.8.3.
- [ ] **dream-as-eval-corpus** — closes the prediction loop: "did wild's predictions come true 6 months later?"
- [x] **Runtime lens-prefix validation** — shipped v0.8.3.
- [x] **Per-file anti-sycophancy regression test for `skills/core/dream/prompts/*.md`** — shipped v0.8.3.
- [x] **Programmatic synthesizer-coda truncation** — shipped v0.8.3 as `swarm.StripDreamCoda`.

## v0.12 — Tier A swarm presets

Three new presets that share a structural signature: *hypothesis generation + cross-agent ranking by evidence*. Multi-agent disagreement IS the differentiator (versus single-agent which produces the obvious answer + misses adversarial angles).

Picks survived a `jutsu swarm dream --mode full --lenses all` adversarial pass on the v0.12 candidate set (7 → 3 after dream's "swarm-value" filter killed `dep-review`/`api-review` as solo-agent territory and `migrate`/`postmortem` as too-high-stakes / harm-vector concerns).

- [x] **`jutsu swarm test-gap`** — shipped v0.12.0. Surfaces failure scenarios MISSING from existing tests. `--code <path> --tests <path>` flags. Synthesizer clusters by category (edge-case / race / resource / input / integration / state) + ranks by severity + cross-reviewer corroboration.
- [x] **`jutsu swarm bug-repro`** — shipped v0.12.0. Vague bug → ranked repro hypotheses across categories (state / race / env / input / version) + minimal-repro steps for top hypothesis. Positional bug description + optional `--files <paths>` for code context.
- [x] **`jutsu swarm code-archaeology`** — shipped v0.12.0. Explain WHY legacy code looks the way it does. `--code <path> [--git-log <since>]`. Multi-agent generates competing historical-context theories with git-log evidence; synthesizer marks corroborated / single-source / contested.

## v0.13 — Preset SDK

Configurable swarm orchestration. Users compose their own swarm shapes via `swarm.yaml` per-project rather than picking from a fixed catalog of subcommands. Reordered ahead of v0.13's original Persona SDK plan after dream surfaced 3-agent consensus that real user pain is **lens routing, not lens authoring** — a persona authoring SDK makes the discovery problem worse.

- [ ] **`swarm.yaml` per-project DSL** — defines swarm shape (preset + persona mix + mode + lenses + cost cap) under a user-chosen name. `jutsu swarm <name>` looks up project-local `swarm.yaml` first, falls back to built-in presets.
- [ ] **Curated example presets** in repo — reference implementations of `swarm.yaml` for the v0.12 Tier A presets so users see how to compose their own.
- [ ] **Hot-reload** — `swarm.yaml` re-read on each invocation; no `jutsu install` step needed.

## v0.13.x — Read-only persona browse

80% of persona-sharing benefit at 5% of full-SDK effort (per dream's status-quo lens). Defer the persona-authoring SDK until user data validates demand.

- [ ] **`jutsu agent persona browse`** — list curated built-ins + community examples. Output is paste-into-`agents.yaml` ready. No install pipeline.
- [ ] **`jutsu agent persona test <name>`** — dry-run validation. Sends a fixed test prompt through the persona; shows the synthesis. Validates lens BEFORE committing to a real swarm run.
- [ ] **`jutsu agent persona new <name>`** — interactive wizard scaffolding into `agents.yaml`. Pure ergonomics; no design risk.

## v0.14+ — Maybe Persona SDK

Conditional on:
1. **A/B evidence** that cross-corpus diversity (claude+gemini+deepseek with same system_prompt) outperforms multi-persona-on-one-provider. Dream flagged this premise as untested + possibly self-confirming bias from RLHF-aligned reviewers.
2. **User data** showing persona authoring (not routing) is the bottleneck. v0.13 Preset SDK addresses routing first; ship Persona SDK only if v0.13 surface usage shows authoring demand.
3. **Behavioral lint** infrastructure (not token-only regex) for catching self-defeating user prompts. Token-only lint = false safety per dream's gaps+adversary cross-lens consensus.

If those hold, scope:
- [ ] Cross-agent personas (`providers: []` field, omitting `provider:` for cross-agent dispatch)
- [ ] Persona-authoring wizard (full lifecycle, not just `persona new`)
- [ ] **Explicitly NOT** persona-as-skill installation — dream flagged as category error (behavior shaping vs capability tool); install pipeline stays scoped to skills only

## v1 — maturity

- [ ] Skill DNA / fingerprinting for semantic dedup
- [ ] Full multi-agent eval matrix in CI (Claude + Codex + Gemini per release)
- [ ] Skill remix (fork-with-lineage, derived skills track ancestry)
- [ ] Bounty board — "wanted: skill that does X" issues with rewards

## Maybe / wild

- [ ] Local LLM trigger eval — verify the description actually fires the skill on a downloaded model
- [ ] Cryptographic provenance for community skills (community-signed tier between unsigned and core-signed)
- [ ] In-CLI sandbox for executing skill scripts (e.g., bubblewrap on Linux, sandbox-exec on macOS)
- [ ] Browser companion (`jutsu web`) for a local UI
- [ ] `jutsu plan` — interactive multi-model brainstorm + synthesize + polish loop following Jeffrey Emanuel's flywheel
- [ ] `jutsu swarm` — staggered N-agent launcher with file reservations
