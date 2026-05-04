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
- **`permissions`**: declared upfront so the CLI can prompt before install.
  - `fs-write: false` — read-only.
  - `fs-write: scoped` — writes are limited to the user's project tree and the agent state directories (`~/.claude/`, `~/.codex/`, `~/.gemini/`, `~/.agents/`), but not arbitrary system paths.
  - `fs-write: full` — unrestricted writes anywhere on disk.

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
      "path": "skills/core/pr-review",
      "integrity": "sha256-...",
      "installedAs": "direct"
    },
    "blunder-hunt": {
      "version": "0.1.0",
      "source": "momentmaker/kaijutsu",
      "ref": "abc123def456...",
      "path": "skills/core/blunder-hunt",
      "integrity": "sha256-...",
      "installedAs": "dep:pr-review"
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
- **`path`** is the directory inside the source repo where the skill lives (omitted for repo-root layouts).
- **`integrity`** is a base64-encoded SHA-256 of the fetched tarball, prefixed `sha256-`. The CLI re-fetches and re-hashes on every sync to verify reproducibility.
- **`installedAs`** records *why* the skill is in the lockfile and is informational. `"direct"` means the user explicitly ran `jutsu install <name>`. `"dep:<parent>"` means the skill was first pulled in to satisfy `<parent>`'s `deps.skills`. The value is written once (on first install) and preserved across subsequent re-encounters; if multiple skills depend on the same dep, only the first parent is recorded. Authoritative orphan detection at remove time should compute the dep graph dynamically by walking every installed skill's `deps.skills` rather than trusting this field as the source of truth.

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

---

## JSON sidecar pattern

Skills produce human-readable markdown by default. When invoked under structured-output mode (a `--json` flag, an environment variable, or a parent skill that needs to consume the output), they MAY also emit a JSON twin alongside the markdown.

The JSON twin is for downstream tools — composition by other skills, eval harnesses, dashboards — and follows a per-skill schema documented in the skill's SKILL.md "JSON sidecar" section.

Conventions:

- Every JSON output object includes `skill` (name) and `version` fields.
- Field names are `lower_snake_case` for machine consumers.
- Free-text fields appear in both the markdown and the JSON; structured fields appear only in the JSON.
- Skills that produce findings should include a stable identifier per finding (e.g., `<file>:<line>:<short-hash>`) so downstream tools can dedupe across runs.

Example skills with declared JSON sidecars:

| Skill | Top-level shape |
|---|---|
| `blunder-hunt` | `{ skill, version, target, passes: [...], synthesized: [...] }` |
| `dispatch-parallel` | `{ skill, version, task, synthesis, subagents: [...], synthesized: [...] }` |
| `multi-model-synth` | `{ skill, version, models, agreements, disagreements, synthesized }` |
| `convergence-detect` | `{ round, tokens, similarity_to_prev, verdict, verdict_confidence }` |

Adopt the pattern in new skills when their output is mechanical enough to be useful to a machine. Skip it for skills whose output is fundamentally prose (README updates, journal entries) — JSON adds noise without adding value.

---

## Cross-skill project memory

Multiple skills accumulate persistent knowledge about a project: `decide` records architectural decisions, `session-retro` extracts learnings, `unstuck` saves resolved-problem patterns, `journal` reads accumulated entries, `agent-doctor` audits for staleness. The `project-memory` skill is the **contract** they share.

The schema:

```
<agent-memory-dir>/
  MEMORY.md              # always-loaded index, one line per entry
  <topic-slug>.md        # individual entries
```

Each topic file:

```yaml
---
name: <one-line entry name>
description: <one-line — used to decide relevance in future conversations>
type: <user | feedback | project | reference>
source: <skill-name that produced this entry>   # optional, recommended
---
<body>
```

Type taxonomy:

- `user` — who the user is, role, preferences, knowledge
- `feedback` — guidance the user has given about how to work — corrections, validations
- `project` — project-specific facts, decisions, deadlines, motivations
- `reference` — pointers to external systems where current state lives

Any skill that reads or writes project memory MUST follow the conventions in the `project-memory` SKILL.md (deduplication rules, agent-platform path resolution, `MEMORY.md` index updates).
