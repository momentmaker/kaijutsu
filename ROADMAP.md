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
- [ ] **Permission prompts**: install-time prompt when a skill declares sensitive `permissions` (`bash: true`, `network: true`, `fs-write: full`) instead of advisory-only
- [ ] **Hooks as first-class artifacts**: extend `skill.yaml` with a `hooks` block; install translates and registers hooks into `~/.<agent>/settings.json`; remove cleans them up. First user: a destructive-command-guard (DCG) hook bundle.
- [ ] **`jutsu eval`**: per-skill eval runner that consumes `evals/cases.yaml`; CI integration via `lint-skills.yml`
- [ ] **`agent-doctor` rich-layout port**: ship `scripts/lib.sh`, `scripts/doctor.sh`, `scripts/cleanup.sh`, `references/{directory-map,cleanup-tiers,protected-paths}.md`, `runbooks/recover-from-trash.md`
- [ ] **Cascade-aware `jutsu remove`**: dynamically compute the dep graph; warn before removing a skill another installed skill depends on; `--cascade` to remove orphans too
- [ ] **`jutsu publish` automation**: shell out to `gh` to fork-and-PR instead of just printing instructions
- [ ] **Trigger-conflict lint**: warn when an installed skill's trigger phrases overlap with another's
- [ ] **Reconcile skill version vs registry tag display + lockfile schema**: today `jutsu list` shows the registry tag (e.g., `0.2.1`) for every installed skill while `jutsu info` shows the skill's internal version from `skill.yaml` (e.g., `pr-review` is internally `0.3.0`). Lockfile schema needs a `tag` field separate from `version`; CLI should display both with clear labels. Closes a side-effect gap: sigstore verify currently skips during `jutsu install` (no args, lockfile sync) because the tag isn't stored — sync verify becomes possible after the schema bump.
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
