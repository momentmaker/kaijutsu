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

## v0.4 — community + telemetry

- [ ] `skills/community/` opens for PRs (auto-merge on green CI + maintainer approval)
- [ ] CODEOWNERS by category for trusted maintainers
- [ ] Anonymized opt-in telemetry: install counts, error rates, skill pairings → drives bundle recommendations
- [ ] `jutsu translate` — convert a skill from one agent's format to another (best-effort)
- [ ] Multi-model API plumbing in the CLI (so `multi-model-synth` doesn't have to fake it)
- [ ] `git-commit` and `reorient` skills (deferred from v0.1)

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
