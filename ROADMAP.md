# Roadmap

## v0 — Public Pre-Alpha (in progress)

Working consume-and-author loop. No bells, no eval gate, no static site.

- [x] Repo scaffolding, LICENSE, README, CONTRIBUTING
- [x] JSON schemas: skill, lockfile, project manifest
- [x] Domain placeholders for `kaijutsu.dev` and `kaijutsu.org`
- [ ] `docs/multi-agent.md` — confirmed install paths for Claude, Codex, Gemini
- [ ] `jutsu` CLI MVP: `init`, `install`, `list`, `remove` (local monorepo only)
- [ ] Lockfile + `upgrade` + remote source resolution via GitHub tarball API
- [ ] 9 seed skills under `skills/core/`: `pr-review`, `readme-update`, `decide`, `journal`, `polish`, `unstuck`, `scope-check`, `session-retro`, `agent-doctor`
- [ ] `publish`, `lint`, `search`, `info` commands
- [ ] Sigstore signing for `skills/core/` on release
- [ ] `momentmaker/homebrew-tap` + `https://kaijutsu.dev/install.sh`

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
