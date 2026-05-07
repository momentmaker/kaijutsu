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
- [ ] **`jutsu eval`**: per-skill eval runner that consumes `evals/cases.yaml`; CI integration via `lint-skills.yml`
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
- [ ] `jutsu eval` (deferred from v0.3) — per-skill eval runner that consumes `evals/cases.yaml`; CI integration via `lint-skills.yml`
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

- [ ] **`--db` flag** for `jutsu finding *` (env override `KAIJUTSU_FINDINGS_DB` ships in v0.7 for tests; user-facing flag deferred).
- [ ] **Configurable bootstrap weight + window size** — `~/.kaijutsu/findings.toml` once we have real-world tuning data.
- [ ] **Auto-prune policy** — `prune.toml` with default-365d eviction; v0.7 ships manual `clear --older-than` only.
- [ ] **`jutsu finding repair`** — vacuum + integrity check + best-effort recovery from corrupt DB.
- [ ] **Rule-based skip + skill-author skip-policy schema**: deterministic heuristic gate (lockfile/comment-only/whitespace auto-skip), `skip_rules:` in agents.yaml.
- [ ] **MCP http transport** (deferred from v0.6 Stage 6).
- [ ] **`--estimate` provider-native tokenizers** (tiktoken-go fallback already tight at ±5% for openai-compat).

## v0.8 — pre-implementation interrogation primitive ✅

- [x] **`dream` skill + `swarm dream` preset** — pre-implementation interrogation through 4-8 cognitive lenses (4 base: honest/fit/gaps/wild + 4 opt-in extras: adversary/inverse/status-quo/time). Anti-sycophancy gates baked into every prompt. Standalone `/dream` walks lenses sequentially; `jutsu swarm dream` dispatches the N×lenses matrix and synthesizes into 4 sections (load-bearing / cross-lens consensus / lens-unique / lens-blind-spots warning). Records to v0.7 findings DB via `[lens:<name>]` summary prefix; no schema migration. `--lenses` flag controls the lens set; default `--max-cost` raised to 3.00 (5.00 with `--lenses=all`). `--mode full` rejected until v0.8.x defines a dream-debate template.

## v0.8.x — committed targets

- [ ] **Anonymized opt-in telemetry** — share aggregated `(provider, persona, preset, precision)` tuples to a community dashboard. Requires consent flow + scrubbing protocol — separate spec + ADR.
- [ ] **PR-comment auto-detection** — `gh pr comments` parsing for `accept`/`dismiss` markers in the kaijutsu-pr-review block; closes the loop without requiring CLI subcommand.
- [ ] **Multi-machine sync** — optional sync of `findings.db` across user's machines via user-configured backend (S3 / gist / syncthing). User-managed, not jutsu-hosted.
- [ ] **Cross-codebase learning** — opt-in: weight a new codebase's tuples by similar codebases (same language signature). Privacy-sensitive — needs design.
- [ ] **Multi-stage swarm pipelines**: `jutsu swarm pipeline brainstorm-then-audit` chains presets with structured handoff; new `pipeline.yaml` DSL.
- [ ] **Live TUI with streaming partial findings**: each provider streams findings as generated; depends on streaming support landing in `http_driver`.
- [ ] **Reverse swarm — spec-vs-impl drift detector**: run on every PR that closes a spec; flag deviations from declared scope/non-goals.
- [ ] **Persona registry + `jutsu install persona:foo`**: personas as installable artifacts.
- [ ] **Per-skill provider routing**: skills declare preferred personas/providers; swarm picks accordingly.
- [ ] **Recorder hook moves below Debate** — Pass-2 `[new]` / `[disputes]` / `[agreed]` revisions get DB rows. v0.7 deferred while we collect signal on whether the Pass-1/Pass-2 summary divergence actually surprises users.
- [ ] **Dream-specific schema migration** — dedicated `lens TEXT` column on findings; recorder + Weighter populate it; per-lens precision tracking + adaptive lens-emphasis weighting in synthesizer.
- [ ] **Dream-flavored Pass-2 debate** — defines what "agents critique each other's lens cells" looks like; lifts `--mode full` reject.
- [ ] **Dream graveyard auto-write** — `~/.kaijutsu/dreams/` write helpers wired into the standalone /dream invocation; cross-codebase recall scoping; lens-rotation rule on repeat-dreams within 7 days.
- [ ] **`jutsu dream clear --older-than 365d`** — auto-prune for the dream graveyard.
- [ ] **dream-as-eval-corpus** — closes the prediction loop: "did wild's predictions come true 6 months later?"

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
