# Contributing to kaijutsu

Thanks for considering a contribution. This project is in pre-alpha; the CLI and core skills are still being scaffolded. Most useful contributions right now are:

- Authoring a new skill (community tier)
- Reporting bugs in the design or schema
- Improving documentation
- Reviewing the [`ROADMAP.md`](./ROADMAP.md) and weighing in on open design questions

## Authoring a skill

Every skill follows the [Anthropic Agent Skills](https://agentskills.io) open standard — a directory containing `SKILL.md` with YAML frontmatter — plus a kaijutsu-specific `skill.yaml` that the `jutsu` CLI uses for registry metadata, permissions, and trust.

### 1. Pick a layout

**Flat** (default — recommended for most skills):

```
skills/community/<your-skill>/
  skill.yaml
  SKILL.md
  scripts/                # optional, shared executables
  README.md
```

**Rich** (Anthropic progressive-disclosure pattern, opt-in for complex domain skills):

```
skills/community/<your-skill>/
  skill.yaml              # layout: rich
  SKILL.md
  references/             # shared deep-dive markdown, loaded on demand
  runbooks/               # shared procedures
  scripts/                # shared executables
  assets/                 # shared templates / fixtures
  subagents/              # delegated specialists
  README.md
```

**Per-agent overrides** (rare — only when behavior must genuinely differ):

```
skills/community/<your-skill>/
  skill.yaml
  SKILL.md                # default for all agents
  overrides/
    claude/SKILL.md       # used only when installing for Claude
    codex/SKILL.md        # used only when installing for Codex
  scripts/
```

See [`SCHEMA.md`](./SCHEMA.md) for the full `skill.yaml` reference and [`docs/multi-agent.md`](./docs/multi-agent.md) for how each agent loads skills.

### 2. Write `SKILL.md`

`SKILL.md` follows the Anthropic Agent Skills standard. The frontmatter `name` must match `skill.yaml:name`.

```markdown
---
name: my-skill
description: One sentence describing when this skill triggers — used by the agent to decide whether to activate.
---

# My Skill

Body of the skill — instructions, examples, references.
```

### 3. Write `skill.yaml`

Required fields:

```yaml
name: my-skill
version: 0.1.0
license: MIT
layout: flat
description: "One sentence describing what this skill does."
agents: [claude, codex, gemini]   # which agents you've tested it with
permissions:
  bash: false
  network: false
  fs-write: false
```

### 4. Lint locally

```bash
jutsu lint skills/community/my-skill
```

This checks schema validity, license compatibility, broken links, and `SKILL.md` frontmatter.

### 5. Submit

- Open a PR with your skill under `skills/community/`.
- The lint workflow will run automatically.
- A maintainer will review for fit, quality, and license compatibility.

Alternative: keep your skill in your own repo and add a one-line entry to `registry/index.json` pointing at it. Same review, less merge churn.

## License compatibility

All contributions must be MIT-licensed. Skills using compatible licenses (BSD-2-Clause, BSD-3-Clause, ISC, Apache-2.0 with attribution preserved) will be accepted on a case-by-case basis. GPL-family licenses are not compatible.

`jutsu lint` flags non-compatible licenses. Each source file should carry an SPDX header where applicable:

```
# SPDX-License-Identifier: MIT
```

## Code of conduct

See [`CODE_OF_CONDUCT.md`](./CODE_OF_CONDUCT.md). Be kind. Assume good faith.

## Maintainer turnaround

There is no SLA on PR review. kaijutsu is solo-maintained. CI must be green before review. Please do not bump or ping unless something has changed.
