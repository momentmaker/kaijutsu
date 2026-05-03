# Schema Reference

Three canonical files define a kaijutsu skill ecosystem:

- **`skill.yaml`** — per-skill manifest (lives inside each skill directory)
- **`kaijutsu.json`** — project manifest (declares which skills a project depends on)
- **`kaijutsu.lock.json`** — pinned resolution of `kaijutsu.json` (committed for reproducibility)

JSON Schema files are in [`schemas/`](./schemas/). The descriptions below are the human reference; the schemas are authoritative.

> **Why this is simpler than you'd expect**
> Claude Code, OpenAI Codex CLI, and Google Gemini CLI all converge on the [Anthropic Agent Skills](https://agentskills.io) open standard: a directory `<name>/SKILL.md` with a YAML frontmatter block (`name`, `description`). Codex and Gemini both read `.agents/skills/<name>/`; Claude reads `.claude/skills/<name>/`. So a kaijutsu skill is just a single `SKILL.md` plus the metadata wrapper described here. See [`docs/multi-agent.md`](./docs/multi-agent.md) for source citations.

---

## Skill directory layout

### Flat layout (default — recommended for ~80% of skills)

```
skills/<area>/<name>/
  skill.yaml          # kaijutsu metadata (this document)
  SKILL.md            # Anthropic Agent Skills standard — works for all three agents
  scripts/            # optional shared executables
  README.md
```

### Rich layout (Anthropic progressive-disclosure pattern, opt-in)

```
skills/<area>/<name>/
  skill.yaml          # layout: rich
  SKILL.md            # entry, loaded always
  references/         # markdown deep-dives, loaded on demand by the agent
  runbooks/           # procedural docs
  scripts/            # executables
  assets/             # templates / fixtures
  subagents/          # agent-delegated specialists
  README.md
```

### Per-agent overrides (rare — opt-in)

When a skill genuinely needs different behavior on a specific agent, override the entry file (or any other file) under `overrides/`:

```
skills/<area>/<name>/
  skill.yaml
  SKILL.md            # default for all agents
  overrides/
    claude/SKILL.md   # used only when installing for Claude
    codex/SKILL.md    # used only when installing for Codex
  scripts/
```

The CLI prefers `overrides/<agent>/<file>` when present; otherwise it uses the top-level file.

---

## `skill.yaml`

Lives at `skills/<area>/<name>/skill.yaml`.

```yaml
name: pr-review                          # required, lowercase-kebab-case
version: 1.2.0                           # required, semver
license: MIT                             # required, SPDX identifier
layout: flat                             # required, "flat" | "rich"
description: "Reviews a PR with confidence-filtered feedback."
author: "Jane Smith <jane@example.com>"  # optional
homepage: https://kaijutsu.dev/skills/pr-review     # optional
repository: https://github.com/momentmaker/kaijutsu # optional
tags: [git, review, pull-request]                   # optional, used by `jutsu search`

agents: [claude, codex, gemini]          # required, list of supported agents

permissions:                             # required
  bash: true                             # skill executes shell commands
  network: false                         # skill makes network requests
  fs-write: scoped                       # "scoped" | "full" | false

deps:                                    # optional
  skills:
    - decide@^1.0
    - scope-check@^1.0

trust:                                   # optional
  expected-signer: kaijutsu-core@github  # author declares who SHOULD have signed;
                                         # the CLI verifies a Sigstore bundle at <skill>/skill.sig.
                                         # Authors do NOT self-assert "signed: true".
```

### Field notes

- **`name`** must be unique within the registry. Names use `lowercase-kebab-case`. The same `name` must appear in the YAML frontmatter at the top of `SKILL.md`.
- **`version`** must be valid semver. The lockfile pins to a specific git ref; if upstream has no semver tags, the CLI pins to a commit SHA.
- **`license`** must be MIT or in the configured allowlist (BSD-2/3, ISC, Apache-2.0).
- **`layout`**: `flat` (default) or `rich` (with `references/`, `runbooks/`, `scripts/`, `assets/`, `subagents/`).
- **`agents`**: only agents listed here will be installed when a user runs `jutsu install`. The intersection of `kaijutsu.json:agents` and `skill.yaml:agents` determines actual install targets.
- **`permissions`**: declared upfront so the CLI can prompt before install. `fs-write: scoped` means the skill writes only inside its own skill directory; `full` means anywhere.

There is no `entry:` field. The CLI installs the entire skill directory and the agent picks up `SKILL.md` automatically. Use `overrides/<agent>/` for rare per-agent splits.

---

## `kaijutsu.json` (project manifest)

Lives at the root of any project that uses kaijutsu. Created by `jutsu init`.

```json
{
  "version": 1,
  "agents": ["claude", "codex"],
  "dependencies": {
    "pr-review": "^1.0",
    "decide": "^1.0"
  },
  "registry": {
    "default": "https://github.com/momentmaker/kaijutsu",
    "extra": []
  }
}
```

- **`agents`** is auto-detected by `jutsu init` based on which agent config dirs exist (`~/.claude`, `~/.codex`, `~/.gemini`). User can edit.
- **`dependencies`** uses semver ranges. `jutsu install <skill>` adds an entry here.
- **`registry.default`** is the canonical kaijutsu monorepo. **`registry.extra`** allows additional registries (community forks, private mirrors).

---

## `kaijutsu.lock.json`

Pinned, reproducible resolution. Committed to source control. Updated by `jutsu install` and `jutsu upgrade`.

```json
{
  "version": 1,
  "agents": ["claude", "codex"],
  "skills": {
    "pr-review": {
      "version": "1.2.0",
      "source": "momentmaker/kaijutsu",
      "ref": "abc123def456...",
      "integrity": "sha256-..."
    },
    "weird-thirdparty-skill": {
      "version": null,
      "source": "github.com/someone/their-skill",
      "ref": "f00ba12...",
      "integrity": "sha256-..."
    }
  }
}
```

- **`version: null`** means the upstream source has no semver tag; the lockfile pins to a commit SHA.
- **`integrity`** is a SHA-256 of the fetched tarball. The CLI re-fetches and re-hashes on every install for verification.

---

## Install paths

`jutsu install` writes to two places (or one, if you target only one family of agents):

| Family | User scope | Project scope |
| ------ | ---------- | ------------- |
| Claude Code | `~/.claude/skills/<name>/` | `<project>/.claude/skills/<name>/` |
| Codex + Gemini | `~/.agents/skills/<name>/` | `<project>/.agents/skills/<name>/` |

A single write to `~/.agents/skills/` covers both Codex and Gemini, per the [Agent Skills standard](https://agentskills.io). Claude Code is the only agent that needs its own directory.

## Global lockfile

`jutsu install -g <skill>` writes to `~/.kaijutsu/global.lock.json`, same schema as the project lockfile.
