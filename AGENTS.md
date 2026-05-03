# AGENTS.md — Contributor guidance for AI coding agents

This file is read by OpenAI Codex CLI and is the canonical contributor guidance for any AI agent working **on** kaijutsu itself (not for users of the published `jutsu` CLI). Claude Code reads `CLAUDE.md` and Gemini CLI reads `GEMINI.md`; both point here.

## Project at a glance

kaijutsu is an MIT-licensed, agent-agnostic registry and CLI for AI coding agent skills. The CLI is named `jutsu`. The registry is this repo. Everything is pre-alpha.

- **Local checkout**: `~/GitHub/momentmaker/kaijutsu/`
- **Remote**: `git@github.com:momentmaker/kaijutsu.git`
- **Public site**: https://kaijutsu.dev (placeholder during v0)
- **Authoritative spec**: see [`SCHEMA.md`](./SCHEMA.md) and [`docs/multi-agent.md`](./docs/multi-agent.md)
- **Live plan**: `~/.claude/plans/so-i-have-this-staged-treehouse.md` (local to maintainer machine)

## Build sequence

The plan is staged 1 → 5. See `ROADMAP.md` for the public version.

1. Repo scaffolding + schemas + multi-agent research **(in progress / current stage)**
2. Go CLI MVP: `init`, `install`, `list`, `remove`
3. Lockfile + `upgrade` + remote sources via GitHub tarball API
4. Author 9 seed skills under `skills/core/`
5. `publish`, `lint`, `search`, `info` + Sigstore signing for core + Homebrew tap + `install.sh`

## Conventions

- **Skill layout**: see [`SCHEMA.md`](./SCHEMA.md). Single `SKILL.md` per skill (Anthropic Agent Skills standard, works for all three target agents). Per-agent overrides only when behavior must genuinely differ — drop them under `overrides/<agent>/`.
- **Install paths**:
  - Claude Code → `.claude/skills/<name>/`
  - Codex + Gemini → `.agents/skills/<name>/` (single write covers both)
- **License**: MIT (or compatible: BSD-2/3, ISC, Apache-2.0). Each source file should carry an SPDX header where applicable.
- **CLI language**: Go. Single static binary. Reuse `cobra`, `spf13/afero`, `go-git`. Shell out to `cosign` for verification, `gh` for PR creation, `goreleaser` for cross-compile + Homebrew tap.
- **Trust**: core skills are Sigstore-signed in CI on tag. Authors do not self-assert `signed: true` in `skill.yaml`; the CLI derives it from a successful `cosign verify-blob` against the declared `expected-signer`.

## Working in this repo

- Always read `SCHEMA.md` and `docs/multi-agent.md` before changing skill schema or install behavior.
- Match existing patterns; don't introduce new conventions without strong justification.
- Don't add features, fallbacks, or abstractions beyond what the current stage requires.
- No emojis in code or commits unless the maintainer asks.
- Commits should be small, focused, and explain *why* in the body. Use [Conventional Commits](https://www.conventionalcommits.org/) prefixes.
- Never bypass commit hooks (`--no-verify`).
- The maintainer reviews every PR personally; CI must be green first. There is no review SLA.

## What's currently empty / stubbed

- `cli/` is empty until Stage 2.
- `skills/core/` is empty until Stage 4.
- `skills/community/` opens after v0.1.
- `.github/workflows/lint-skills.yml` is a no-op skeleton until the CLI lands.
- The static site at `kaijutsu.dev` is a placeholder; v0 has no real registry UI.

## Don't do

- Don't write code that calls or depends on the real `jutsu` CLI before Stage 2 lands; it doesn't exist yet.
- Don't author seed skills in `skills/core/` before Stage 4. The schema may still shift.
- Don't sign anything yet; Sigstore arrives in Stage 5.
- Don't accept third-party PRs to `skills/community/` until v0.1.
