# AGENTS.md — Contributor guidance for AI coding agents

This file is read by OpenAI Codex CLI and is the canonical contributor guidance for any AI agent working **on** kaijutsu itself (not for users of the published `jutsu` CLI). Claude Code reads `CLAUDE.md` and Gemini CLI reads `GEMINI.md`; both point here.

## Project at a glance

kaijutsu is an MIT-licensed, agent-agnostic registry and CLI for AI coding agent skills. The CLI is named `jutsu`. The registry is this repo. Pre-alpha — public API may shift; pin versions if you depend on it.

- **Local checkout**: `~/GitHub/momentmaker/kaijutsu/`
- **Remote**: `git@github.com:momentmaker/kaijutsu.git`
- **Public site**: https://kaijutsu.dev (live; sitegen-driven catalog)
- **Authoritative spec**: see [`SCHEMA.md`](./SCHEMA.md) and [`docs/multi-agent.md`](./docs/multi-agent.md)
- **Roadmap**: [`ROADMAP.md`](./ROADMAP.md) (current state, deferrals, what's next)
- **Recent specs / decisions**: [`docs/specs/`](./docs/specs/) + [`docs/decisions/`](./docs/decisions/)

## Current state (as of v0.8.1)

- 17 core skills + 6 community + 8 third-party registry pointers (~46 catalog entries on kaijutsu.dev).
- 6 swarm presets shipping: pr-review, doc-review, brainstorm, refactor-plan, security-audit, dream.
- v0.6 multi-provider swarm (driver abstraction + agents.yaml + personas) — claude/codex/gemini cli + http (deepseek/glm/kimi/ollama-local) + cli-compat + mcp.
- v0.7 quality fingerprinting (local SQLite store at `~/.kaijutsu/findings.db`) + confidence-weighted synthesizer.
- v0.8 dream skill + swarm dream preset (pre-implementation interrogation through 4-8 cognitive lenses).
- v0.8.1 patch ships dream Wild lens %!s(MISSING) fix + 28→17 core curation.
- v0.8.2 in flight: agent-first lens (this section) + jutsu describe + jutsu suggest + jutsu init AGENTS.md fragment + AutoFormat helper.

See `ROADMAP.md` for v0.8.x followups + v0.9 candidates.

## Design principle: agent-first, human-friendly

Default lens for every design decision in this repo. Apply per-command, per-skill, per-output-format choice.

> **Rule**: machine-parseable + well-formed FIRST. Aesthetically pleasing for humans SECOND. They compose ~90% of the time. When they conflict, machine wins for primitive commands; human wins for synthesis output meant for direct review.

What this means in practice:

- **Output format auto-detection**: `isatty(stdout) AND no --format flag → markdown / table / colored`. Non-TTY (pipe, redirect, agent capture) → `JSON`. Explicit `--format json` / `--json` / `--format markdown` overrides. Pattern from `gh`, `jq -C`, `kubectl`.
- **Stable exit codes** + structured error messages with codes. Agents can self-recover; humans can grep.
- **Idempotent commands**. Re-run is safe. Critical for agent retry loops.
- **Deterministic file outputs** — sorted, stable. Avoids spurious diff churn.
- **No interactive prompts blocking pipelines.** `--yes` for non-interactive consent. TUI / fuzzy-finder UI is rejected on principle (breaks pipes, contradicts the lens).
- **Self-describing CLI**: `jutsu --describe` returns a JSON catalog of every command + flags + descriptions. Fresh agents ingest this to learn the surface. Humans can pipe through `jq` if curious.
- **Pipe-friendly composition**: `jutsu list --json | jq ...` works. Every primitive command supports JSON output.
- **Skills layer**: skills are markdown documents agents READ and ACT on. Already agent-first by design.
- **Synthesis output (e.g. `jutsu swarm <preset>`)**: markdown by default in TTY (human reads the review), JSON in pipe (next-stage agent consumes). Auto-flip applies — same rule.

What this is NOT:

- Not a charm-ecosystem TUI redesign. The dream session on the gum-UX question (2026-05-07) caught this: TUIs break pipes, narrow agent surface, contradict "boring + obvious" philosophy. We OPT OUT of charm/bubbletea/lipgloss for primary UX.
- Not "AI-only, humans secondary" branding. Humans still write the contributor docs, read the synthesis output, and ship the project. Agent-first is a DESIGN LENS, not a market position.
- Not a strict rule. Use judgment. Some commands legitimately have no machine consumer (`jutsu --version`); JSON output there is overkill. The rule applies where agent OR human consumption is real.

If a new command / output / behavior fails the lens, push back. The Why-line in the commit message should reference this principle when it's load-bearing.

## Conventions

- **Skill layout**: see [`SCHEMA.md`](./SCHEMA.md). Single `SKILL.md` per skill (Anthropic Agent Skills standard, works for all three target agents). Per-agent overrides only when behavior must genuinely differ — drop them under `overrides/<agent>/`.
- **Install paths**:
  - Claude Code → `.claude/skills/<name>/`
  - Codex + Gemini → `.agents/skills/<name>/` (single write covers both)
- **License**: MIT (or compatible: BSD-2/3, ISC, Apache-2.0). Each source file should carry an SPDX header where applicable.
- **CLI language**: Go. Single static binary. Reuse `cobra`, `spf13/afero`, `go-git`. Shell out to `cosign` for verification, `gh` for PR creation, `goreleaser` for cross-compile + Homebrew tap.
- **Trust**: core skills are Sigstore-signed in CI on tag. Authors do not self-assert `signed: true` in `skill.yaml`; the CLI derives it from a successful `cosign verify-blob` against the declared `expected-signer`.

## Working in this repo

- Always read `SCHEMA.md` + `docs/multi-agent.md` before changing skill schema or install behavior. Read `docs/project-memory.md` before changing how skills write persistent memory.
- Match existing patterns; don't introduce new conventions without strong justification + a written reason in the commit body.
- Don't add features, fallbacks, or abstractions beyond what the current ship cycle requires (see `ROADMAP.md`).
- No emojis in code or commits unless the maintainer asks.
- Commits should be small, focused, and explain *why* in the body. Use [Conventional Commits](https://www.conventionalcommits.org/) prefixes (`feat:`, `fix:`, `docs:`, `refactor:`, `spec:`, etc.).
- Never bypass commit hooks (`--no-verify`).
- The maintainer reviews every PR personally; CI must be green first. There is no review SLA.
- Skill / spec / ADR work follows the dogfood loop: spec → `jutsu swarm doc-review` → fix → ship → polish loop → adversarial `jutsu swarm pr-review` → tag.

## Don't do

- **Don't break the agent-first lens** (see Design principle above). New commands consumed by agents must support JSON output via AutoFormat. Auto-flip on TTY/pipe. Stable schema (`schema_version` field) on JSON outputs.
- **Don't ship interactive TUIs** for primary UX. Charm/bubbletea/lipgloss is rejected on principle — see the dream session on the gum-UX question (2026-05-07). Optional opt-in fancy mode may land in v0.9+ behind a flag, never default.
- **Don't add Go dependencies casually.** Each new dep gets a justification in the commit body. Prefer stdlib / existing deps / shell-out to canonical tools (`cosign`, `gh`, `goreleaser`).
- **Don't extend the kaijutsu.json schema without a SCHEMA.md update + migration doc.** The lockfile + manifest are the contract; agents read them directly.
- **Don't sign skills outside CI.** Sigstore signing happens via `.github/workflows/sign-core.yml` on tag push. Manual signing breaks the trust chain.
- **Don't accept third-party PRs to `skills/community/` without lint + maintainer review.** Lint is necessary, not sufficient — the maintainer eyeballs each.
- **Don't add `kind: hook` artifacts to the install path.** Hooks are agent-specific (Claude Code's PreToolUse, Codex's etc.). Users wire them themselves; jutsu doesn't auto-install hook configs. dcg ships in `skills/community/` as documentation + script the user manually wires.
- **Don't reach for MCP server work yet.** v0.8 dream session on the MCP idea (2026-05-07) found the premise ("self-discovery via MCP") was partially false (users still hand-configure MCP per client). Status quo (CLI + AGENTS.md fragment + JSON outputs) reaches more clients with less risk. Revisit in 6-12 months when MCP protocol stabilizes.
- **Don't conflate `kaijutsu.json` with `.kaijutsu/agents.yaml`.** Different files, different scopes:
  - `kaijutsu.json` (root, JSON) = which agent platforms (claude/codex/gemini) the project ships skills for + skill dependencies + lockfile.
  - `.kaijutsu/agents.yaml` (subdir, YAML) = swarm provider catalog (deepseek/glm/kimi/etc.) + persona declarations. Loaded by `jutsu swarm`.

## Recent design decisions worth knowing

- **agent-first, human-friendly** (this AGENTS.md, 2026-05-07): every output format auto-flips JSON-on-pipe / pretty-on-TTY. `jutsu describe` exists for fresh-agent self-discovery. `jutsu init` writes the AGENTS.md fragment.
- **dream skill + swarm dream preset** (v0.8.0, see `docs/specs/2026-05-07-v0.8.0-dream-skill-and-preset.md`): pre-implementation interrogation through 8 lenses. First dream-of-self caught a real bug in the dream prompt template (recursive correctness check works).
- **Quality fingerprinting** (v0.7.0, see `docs/specs/2026-05-06-v0.7.0-quality-fingerprinting.md`): per-(provider, persona, preset, codebase) precision tracking via local SQLite. Synthesizer downweights noisy tuples on next swarm.
- **Multi-provider swarm** (v0.6.0, see `docs/specs/2026-05-05-v0.6.0-multi-provider-agents.md`): driver abstraction + agents.yaml + personas. Not locked to claude/codex/gemini anymore.
- **Core skill curation** (v0.8.1, 2026-05-07): 28 → 17 core. Moved code-simplification / security-and-hardening / journal / session-retro / readme-update / dcg to `skills/community/`. Deleted decide / agent-doctor / multi-model-synth / lie-to-them / project-memory (the schema doc lives at `docs/project-memory.md`).
