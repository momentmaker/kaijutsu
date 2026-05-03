# Roadmap

## v0 — Public Pre-Alpha (in progress)

Working consume-and-author loop. No bells, no eval gate, no static site.

- [x] Repo scaffolding, LICENSE, README, CONTRIBUTING
- [x] JSON schemas: skill, lockfile, project manifest
- [x] Domain placeholders for `kaijutsu.dev` and `kaijutsu.org`
- [ ] `docs/multi-agent.md` — confirmed install paths for Claude, Codex, Gemini
- [x] `jutsu` CLI MVP: `init`, `install`, `list`, `remove` (local monorepo only)
- [x] Lockfile + `upgrade` + remote source resolution via GitHub tarball API
- [x] 9 seed skills under `skills/core/`: `pr-review`, `readme-update`, `decide`, `journal`, `polish`, `unstuck`, `scope-check`, `session-retro`, `agent-doctor`
- [x] `publish`, `lint`, `search`, `info` commands
- [x] Sigstore signing workflow for `skills/core/` on release tag (sign-core.yml)
- [x] Goreleaser pipeline for jutsu binaries + Homebrew tap on release tag (release-jutsu.yml)
- [x] `https://kaijutsu.dev/install.sh` published from /docs (GitHub Pages)
- [ ] First v0.1.0 tag pushed (triggers the release + sign workflows for the first time)
- [ ] `momentmaker/homebrew-tap` repo created (one-time, by maintainer)

## v0.1 — Trust + Discovery

- [ ] Eval gate: author-supplied test prompts run in CI against Claude Sonnet 4.6 (and others where available)
- [ ] `skills/community/` opens for PRs (auto-merge on green CI + signed approval)
- [ ] Static site at `kaijutsu.dev` — searchable catalog, agent-compat matrix, install copy-buttons
- [ ] `git-commit` and `reorient` skills (deferred from v0)
- [ ] `jutsu translate` — convert a skill from one agent's format to another (best-effort)
- [ ] CODEOWNERS by category for trusted maintainers

## v1 — Maturity

- [ ] Skill DNA / fingerprinting for semantic dedup
- [ ] Full multi-agent eval matrix
- [ ] Composable skills (deps graph)
- [ ] Telemetry (opt-in, anonymized): popular skills, install pairings → bundle recommendations
- [ ] Bounty board: "wanted: skill that does X" issues with rewards
- [ ] Skill remix (fork-with-lineage, derived skills track ancestry)

## Maybe / Wild

- [ ] Local LLM trigger eval (verify the description actually fires the skill)
- [ ] Cryptographic provenance for community skills (community-signed tier between unsigned and core-signed)
- [ ] In-CLI sandbox for executing skill scripts
- [ ] Browser companion (`jutsu web`) for a local UI
