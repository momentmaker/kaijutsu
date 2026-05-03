---
name: agent-doctor
description: Diagnostic + cleanup tool for AI agent state directories. Audits disk usage across ~/.claude, ~/.codex, ~/.gemini, ~/.agents; surfaces stale sessions, orphaned cache, broken hook refs; reclaims space safely (trash-not-delete, 30-day recovery). Use when the user says "agent doctor", "clean up claude", "audit codex", "agents disk full", "old sessions", or invokes /agent-doctor. Subcommands — /agent-doctor (read-only report, default), /agent-doctor cleanup (--dry-run by default; pass --apply to commit; --tier safe|aggressive), /agent-doctor recover (restore from trash), /agent-doctor purge-trash. Read-only by default. Reversible for 30 days via trash. Never touches MEMORY.md, CLAUDE.md, AGENTS.md, GEMINI.md, agents/, skills/, hooks/, rules/, identities/, plans/, settings*.json.
---

# agent-doctor

Audit + clean the per-agent state directories (`~/.claude`, `~/.codex`, `~/.gemini`, `~/.agents`). Like `brew doctor` for your agent state.

This is a multi-agent generalization of the original Claude-Code-only `claude-doctor`. Run with no args to get a read-only health report; everything else is opt-in.

## Default behavior

Running `/agent-doctor` with no args = **read-only health report**. Never modifies anything. Safe to run anytime.

## Detect installed agents

Check which agent dirs exist and report each one separately:

```bash
for dir in ~/.claude ~/.codex ~/.gemini ~/.agents; do
  [ -d "$dir" ] && echo "$dir present"
done
```

The cleanup tiers below apply per-directory; rules differ slightly per agent because each agent uses a different layout.

## What it looks at

Common subdirectories the agent runtimes commonly create:

```
~/.<agent>/
├── projects/                          per-repo session state (often the fattest dir)
├── telemetry/                         failed-event uploads (auto-purgeable)
├── file-history/                      editor undo state
├── paste-cache/                       clipboard captures
├── image-cache/                       image inputs to past sessions
├── plugins/                           installed plugins + asset cache
└── .trash/                            quarantine (created by this skill on apply)
```

Not all agents create all of these. The skill should `du -sh` whatever exists and skip the rest silently.

## Cleanup tiers

| Tier | What it touches |
|---|---|
| **safe** (default) | telemetry >7d, paste-cache >30d, image-cache >30d, file-history >90d, stale ephemeral state files >14d, **orphaned project dirs** (repos no longer on disk) |
| **aggressive** | safe + session jsonls >180d compressed with `zstd` + trash >30d hard-deleted |

## Protected paths (never touched)

Across every agent: `CLAUDE.md`, `AGENTS.md`, `GEMINI.md`, `MEMORY.md` (anywhere), `agents/`, `skills/`, `rules/`, `hooks/`, `identities/`, `plans/`, `settings.json`, `settings.local.json`, `keybindings.json`.

If a user requests removing something protected, refuse and explain why. Suggest they edit the file directly if they want a finer-grained change.

## Trash, not delete

Everything goes to `~/.<agent>/.trash/YYYY-MM-DD-HHMMSS/<original-relative-path>` first. Hard-delete only happens when `--tier aggressive --apply` runs and a trash batch is older than 30 days.

## Workflow

When the user invokes this skill:

1. **First time / no args** — run the audit (`du -sh` per subdir of every detected agent dir, plus age-based candidate counts). Report findings. Highlight the safe-tier recoverable total. Suggest next step (`cleanup --dry-run`).
2. **User says "clean it up"** — run the cleanup logic with `--dry-run` (always dry-run first). Show what would move. Ask for confirmation before `--apply`.
3. **User confirms** — run with `--apply`. Report what got trashed + total reclaimed.
4. **User says "I deleted X by accident"** — walk through recovery: list trash batches by date, locate the file, copy it back to its original path, optionally remove from trash.
5. **User asks for aggressive cleanup** — explain what aggressive does (jsonl compression + 30d trash purge), confirm explicitly, then run with `--tier aggressive`.

## Detecting orphaned project dirs

Each agent encodes the project path differently. Where the encoding is reversible (replace `-` with `/` doesn't always work — repo names containing `-` are ambiguous), best-effort resolve:
- Try to recover the original path
- `[ -d "$resolved" ]` — if the directory still exists, it's not orphaned
- Show the user the resolved path so they can sanity-check before confirming

## Hard rules

- **Never** run `--apply` without showing dry-run output first OR without explicit user confirmation in the same turn.
- **Never** touch any protected path. If the user requests it, refuse and explain why.
- **Always** stage to `.trash/` before any irreversible action.
- For orphaned project dirs: heuristic resolution is fragile. Always show the resolved path before confirming.
- Errors in the audit ≠ silent failures. If `find` returns nothing, that's "0 items," not "broken." Report 0 honestly.

## What's missing in this v0 port

The original `claude-doctor` shipped a rich layout with `scripts/`, `references/`, and `runbooks/`. This v0 skill is the flat equivalent: the SKILL.md describes the workflow, and the agent runs ad-hoc shell commands to execute it. A future v0.1 release should re-introduce:

- `scripts/lib.sh` — shared helpers (color, sizing, protected-path check, trash, path resolver)
- `scripts/doctor.sh` — read-only report
- `scripts/cleanup.sh` — staged cleanup
- `references/directory-map.md` — per-agent directory inventory
- `references/cleanup-tiers.md` — rule table + rationale per cutoff
- `references/protected-paths.md` — never-touch list + how to extend
- `runbooks/recover-from-trash.md` — restoring trashed items

Until then, the skill works by leaning on the agent's own ability to run shell commands inline and follow this SKILL.md as the spec.
