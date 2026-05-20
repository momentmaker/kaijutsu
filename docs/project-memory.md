# project-memory — cross-skill memory schema

> **Status:** convention doc (not an installable skill).
> **Audience:** authors writing kaijutsu skills that read or write persistent project memory.

This doc was previously the `project-memory` skill (deleted in v0.8.1's curation pass — it was a schema spec wrapped in skill clothing, not a runnable skill). The schema itself is still the canonical contract for cross-skill memory.

Several kaijutsu skills accumulate knowledge about a project over time and follow this schema:

- `dream` (graveyard recall)
- `unstuck` (resolved-problem patterns)
- `session-retro` (community tier — extracts learnings from conversations)
- `journal` (community tier — reads accumulated entries to summarize)

This doc defines where memory lives, what shape entries take, and how skills should read / write / dedupe.

## Storage location

Each agent platform has its own per-project memory directory. Skills should abstract over them:

| Agent | Path |
| --- | --- |
| Claude Code | `~/.claude/projects/<encoded-project-path>/memory/` |
| Codex CLI | `~/.codex/projects/<encoded-project-path>/memory/` (or platform equivalent) |
| Antigravity CLI | `~/.antigravity/projects/<encoded-project-path>/memory/` (or platform equivalent) |

`<encoded-project-path>` is the URL-safe encoding of the project's absolute filesystem path. Each agent runtime auto-resolves this differently; kaijutsu skills should call into the agent's native primitive when available rather than computing the path manually.

**Per-agent primitive availability:**

| Agent | Native auto-memory? | Fallback |
|---|---|---|
| Claude Code | Yes — auto-memory writes to `~/.claude/projects/<encoded>/memory/` | n/a |
| OpenAI Codex CLI | Partial — stateful via `AGENTS.md` but no per-project key/value store | Write to `<project>/.kaijutsu/memory/` and document path in AGENTS.md |
| Google Antigravity CLI | Partial — extensions can ship state but no first-class memory primitive | Same fallback as Codex |

When writing skills that touch memory, prefer the native primitive when present, and fall back to a project-local `.kaijutsu/memory/` directory otherwise. Document the chosen path in the project's `AGENTS.md` so subsequent sessions can find it.

## Directory layout

```
<memory-dir>/
  MEMORY.md              # always-loaded index, one line per entry
  <topic-slug>.md        # individual entries
  <topic-slug>.md
  ...
```

`MEMORY.md` is loaded into the agent's context at session start. Keep it tight — one line per entry, ≤200 chars per line. Detail lives in topic files, loaded on demand.

## Entry schema

Every topic file has YAML frontmatter:

```yaml
---
name: <one-line entry name>
description: <one-line — used to decide relevance in future conversations>
type: <user | feedback | project | reference>
---
```

Plus a body. For `project` and `feedback` types, the body should be structured:

```markdown
<the rule or fact>

**Why:** <reason — often a past incident or strong preference>
**How to apply:** <when/where this guidance kicks in>
```

For `user` and `reference` types, free-form prose is fine.

## Type semantics

| Type | What it captures |
| --- | --- |
| `user` | Who the user is, their role, preferences, knowledge |
| `feedback` | Guidance the user has given about how to work — corrections, validations |
| `project` | Project-specific facts, decisions, deadlines, motivations |
| `reference` | Pointers to external systems where current state lives |

## Reading

Before doing relevant work, a skill should:
1. List all topic files in the memory directory
2. Read frontmatter only (description) for each — cheap scan
3. Identify entries relevant to the current task by description match
4. Read full body only for relevant entries

Always prefer reading existing memory over asking the user a question they've answered before.

## Writing

Two-step:

### Step 1 — write the topic file

Use a stable slug. Topic slugs are kebab-case derived from the entry's primary noun phrase. Example: a feedback memory about preferring SQLite goes to `prefers-sqlite-over-postgres.md`.

Check for an existing entry with the same name first. If present, update in place rather than creating a duplicate.

### Step 2 — update MEMORY.md

Add a one-line pointer:

```markdown
- [Entry name](file-slug.md) — one-line hook
```

Order matters less than findability. Group by type if the index gets long.

## Deduplication rules

Skills writing to project memory MUST:

- Search by description before creating a new entry. If 70%+ semantic overlap with an existing entry, update the existing entry instead.
- Never create two entries with the same `name` field.
- When updating, preserve the original `name` and `description` if the user authored them; only edit if the new content fundamentally changes the meaning.

## Removal

Outdated entries:
1. Verify staleness (the underlying fact has changed)
2. Either delete the topic file + remove from MEMORY.md, or mark `type: archived` and remove from MEMORY.md

## Cross-skill integration

When a skill writes to project memory, it should:
- Use the agent's auto-memory primitive (don't compute paths manually)
- Tag the entry's frontmatter with `source: <skill-name>` so audits can identify which skill produced which entry (optional but recommended)

When a skill reads project memory, it should:
- Treat MEMORY.md as the index of record
- Verify any cited fact against current observable state before relying on it (memory can decay)

## Hard rules

- **Don't compute memory paths from raw filesystem operations.** Use the agent's auto-memory primitive. Path encoding differs between agent versions.
- **Don't write to memory without checking for existing entries.** Duplicates degrade signal-to-noise.
- **Don't write user-private info.** Memory may be shared across sessions and could be reviewed by the user later. No secrets, no tokens, no third-party private data.
- **Don't paraphrase user-provided memory.** If the user said "X is the rule," store "X is the rule," not "the user prefers X-like rules."
