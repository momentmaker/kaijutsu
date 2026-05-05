<p align="center">
  <img src="./docs/assets/logo-256.png" alt="kaijutsu mascot" width="200" height="200">
</p>

<h1 align="center">kaijutsu</h1>

<p align="center">
  <strong>Open skills for AI coding agents.</strong><br>
  One source of truth — Claude, Codex, Gemini, all from the same registry.
</p>

<p align="center">
  <a href="https://kaijutsu.dev">kaijutsu.dev</a> ·
  <a href="./ROADMAP.md">Roadmap</a> ·
  <a href="./CONTRIBUTING.md">Contributing</a>
</p>

<p align="center">
  <a href="https://github.com/momentmaker/kaijutsu/releases/latest"><img src="https://img.shields.io/github/v/release/momentmaker/kaijutsu?display_name=tag&color=3DDC97" alt="Latest release"></a>
  <a href="./LICENSE"><img src="https://img.shields.io/badge/license-MIT-3DDC97.svg" alt="License: MIT"></a>
  <a href="https://github.com/momentmaker/kaijutsu/actions/workflows/lint-skills.yml"><img src="https://github.com/momentmaker/kaijutsu/actions/workflows/lint-skills.yml/badge.svg" alt="Lint skills"></a>
</p>

---

> **Status: alpha.** The `jutsu` CLI works end-to-end (init / install / list / remove / upgrade / lint / search / info / publish). 16 core skills currently ship: 9 task-oriented (decide, journal, polish, unstuck, scope-check, session-retro, agent-doctor, pr-review, readme-update) and 7 composable primitives (blunder-hunt, lie-to-them, convergence-detect, deslop, dispatch-parallel, multi-model-synth, project-memory). Install paths, skill schema, and command surface are stable for v0.x. Signature-verification enforcement, hooks-as-first-class, and the public skill catalog at [kaijutsu.dev](https://kaijutsu.dev) are still maturing — see [`ROADMAP.md`](./ROADMAP.md). Security model: [`SECURITY.md`](./SECURITY.md).

## Why

Skills (markdown + scripts that extend AI coding agents) are exploding across Claude Code, Codex, and Gemini CLI. The good news: all three agents converge on the [Anthropic Agent Skills](https://agentskills.io) open standard, and Codex + Gemini both read from `.agents/skills/`. The bad news: nobody is curating a shared registry, and there is no easy way to keep the same toolkit in sync across all three agents.

**kaijutsu** is an MIT-licensed, agent-agnostic registry and CLI for AI agent skills. The CLI is named `jutsu`. One author manifest, two install paths (`.claude/skills/` for Claude, `.agents/skills/` for Codex + Gemini), three agents covered.

## Quickstart

```bash
# Install jutsu
brew install momentmaker/tap/jutsu
# or
curl -fsSL https://kaijutsu.dev/install.sh | sh

# Uninstall jutsu
brew uninstall jutsu
# or
curl -fsSL https://kaijutsu.dev/uninstall.sh | sh

# In any project
jutsu init                  # detects which agents you have installed
jutsu install pr-review     # writes to .claude/skills/ and/or .agents/skills/
jutsu list
jutsu upgrade

# Globally
jutsu install -g unstuck
```

A skill lives in this repo (or any third-party repo registered in `registry/index.json`) as a directory:

```
skills/core/pr-review/
  skill.yaml          # kaijutsu metadata (name, version, license, perms, agents[])
  SKILL.md            # Anthropic Agent Skills standard — works for all three agents
  scripts/            # optional shared executables
  README.md
```

Most skills only need a single `SKILL.md`. For the rare cases where Claude and Codex need different content, drop a per-agent override under `overrides/<agent>/SKILL.md`. See [`SCHEMA.md`](./SCHEMA.md) and [`docs/multi-agent.md`](./docs/multi-agent.md) for the full reference.

## Core skills

**Task-oriented** — `pr-review` · `readme-update` · `decide` · `journal` · `polish` · `unstuck` · `scope-check` · `session-retro` · `agent-doctor` · `doc-review`

**Primitives (composed by other skills via `deps.skills`)** — `blunder-hunt` · `lie-to-them` · `convergence-detect` · `deslop` · `dispatch-parallel` · `multi-model-synth` · `project-memory`

Run `jutsu search <query>` to find skills by name / description / tag, or `jutsu info <skill>` for full metadata.

## Multi-agent flywheel (`jutsu swarm`)

Two skills (`pr-review` v1.x, `doc-review` v0.1+) ship as multi-agent presets that orchestrate claude / codex / gemini in parallel through `jutsu swarm <preset>`. Each agent reviews the same input through a different lens, then a synthesizer produces a single markdown report with a disagreement table. Used as the universal QA gate for kaijutsu artifacts:

- `jutsu swarm pr-review --pr 42` — adversarial multi-agent code review on a PR
- `jutsu swarm doc-review SPEC.md` — multi-agent review on a spec / plan / decision record / design doc

Three artifact-producing skills (`spec-driven-development`, `planning-and-task-breakdown`, `decide`) call `jutsu swarm doc-review` as their final review pass instead of inventing their own — a single shared QA gate replaces three ad-hoc mechanisms. See [`IMPLEMENTATION_PLAN_PHASE2.md`](./IMPLEMENTATION_PLAN_PHASE2.md) for the design.

## Trust model

- **Core skills** (this monorepo, `skills/core/`) are signed at release time using [Sigstore](https://www.sigstore.dev/) keyless signing. The CLI verifies the signature and the GitHub identity of the signer before installing.
- **Community skills** (third-party repos registered in `registry/index.json`) are unsigned by default. The CLI prompts the user when a skill declares sensitive permissions (shell, network, filesystem write).
- Every skill ships a permission manifest in `skill.yaml`.

## Troubleshooting

### `GitHub API rate limit exceeded`

`jutsu install` and `jutsu upgrade` hit the GitHub API to resolve tags and download tarballs. Anonymous requests are capped at **60/hr per IP**; authenticated requests get **5000/hr**.

Set `GITHUB_TOKEN` to a personal access token to authenticate:

```sh
# Use the token gh CLI is already holding
export GITHUB_TOKEN=$(gh auth token)

# Or set explicitly in your shell rc (~/.zshrc, ~/.bashrc, etc.)
export GITHUB_TOKEN="ghp_..."
```

A fine-grained PAT with **public-repository read access** is sufficient. No write or admin scopes needed.

The same env var works for `install.sh`:

```sh
curl -fsSL https://kaijutsu.dev/install.sh | GITHUB_TOKEN=$(gh auth token) sh
```

### `dep <X> of <Y>: skill.yaml not found at skills/core/<X> in momentmaker/kaijutsu@<sha>`

You're on `v0.2.0`, which shipped broken transitive dep constraints. Upgrade to `v0.2.1` or later (`brew upgrade jutsu` or re-run `install.sh`).

## Recommended collections

kaijutsu is intentionally not the *only* skill source. We bundle a tight set under `skills/core/` (meta-process, primitives, safety) and adapt a handful of high-leverage skills from peer collections under MIT attribution:

- **[`addyosmani/agent-skills`](https://github.com/addyosmani/agent-skills)** — Addy Osmani's 20-skill SDLC framework (Define / Plan / Build / Verify / Review / Ship / Simplify). 5 of these are ported into our `skills/core/` with attribution: `spec-driven-development`, `planning-and-task-breakdown`, `security-and-hardening`, `code-simplification`, `incremental-implementation`. The remaining 15 are referenced in `registry/index.json` and installable via `jutsu install addyosmani/<skill>` (v0.3 vanilla-SKILL.md compat mode).
- **[Anthropic Agent Skills](https://agentskills.io)** — the open standard our schema extends. Any skill that follows the SKILL.md frontmatter convention is installable via kaijutsu, agent-agnostic.

Full attribution + license accounting in [`NOTICE.md`](./NOTICE.md).

## Contributing

See [`CONTRIBUTING.md`](./CONTRIBUTING.md). Skill authors can submit a PR to `skills/community/` or register a third-party repo in `registry/index.json`. All contributions must be MIT-licensed (or compatible: BSD-2/3, ISC, Apache-2.0).

## What "kaijutsu" means

Three readings, layered:

1. **Open skills** *(primary)* — `kai` (開, "open") + `jutsu` (術, "skill / technique"). An open registry of agentic skills, free for any AI agent to consume.
2. **Beast of techniques** — `kaiju` (怪獣, "strange beast") + `jutsu`. A nod to the chibi monster mascot. Open-source agents are wild creatures of capability waiting to be tamed and shared.
3. **Kaizen-jutsu** — short for *kaizen-no-jutsu*, "techniques of continuous improvement." A system of techniques varying in intensity and effort to make oneself better and stronger toward the state of **optimum improvement**, lived by a regret-minimization framework.

The primary meaning is *open skills*. The other readings are happy resonances.

## License

MIT. See [`LICENSE`](./LICENSE).
