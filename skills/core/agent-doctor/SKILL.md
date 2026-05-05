---
name: agent-doctor
description: Diagnostic + cleanup tool for AI agent state directories. Audits disk usage across ~/.claude, ~/.codex, ~/.gemini, ~/.agents; surfaces stale sessions, orphaned cache, broken hook refs; reclaims space safely (trash-not-delete, 30-day recovery). Use when the user says "agent doctor", "clean up claude", "audit codex", "agents disk full", "old sessions", or invokes /agent-doctor. Subcommands — /agent-doctor (read-only report, default), /agent-doctor cleanup (--dry-run by default; pass --apply to commit; --tier safe|aggressive), /agent-doctor recover (restore from trash), /agent-doctor purge-trash. Read-only by default. Reversible for 30 days via trash. Never touches MEMORY.md, CLAUDE.md, AGENTS.md, GEMINI.md, agents/, skills/, hooks/, rules/, identities/, plans/, settings*.json.
---

# agent-doctor

Audit + clean the per-agent state directories (`~/.claude`, `~/.codex`, `~/.gemini`, `~/.agents`). Like `brew doctor` for your agent state.

This is a multi-agent generalization of the original Claude-Code-only `claude-doctor`. Run with no args to get a read-only health report; everything else is opt-in.

## Default behavior

Running `/agent-doctor` with no args = **read-only health report**. Never modifies anything. Safe to run anytime.

## Layout (rich)

```
agent-doctor/
├── SKILL.md                       (this file — workflow + decision rules)
├── skill.yaml
├── scripts/
│   ├── lib.sh                     shared helpers (color, sizing, protected-path check, trash, path resolver)
│   ├── doctor.sh                  read-only report (default subcommand)
│   └── cleanup.sh                 staged cleanup with dry-run default, trash-not-delete
├── references/
│   ├── directory-map.md           per-agent inventory of subdirectories + cleanup status
│   ├── cleanup-tiers.md           safe/aggressive rule table + cutoff rationale
│   └── protected-paths.md         never-touch list + how to extend
└── runbooks/
    └── recover-from-trash.md      restoring a trashed item
```

The agent should follow this SKILL.md as the spec. Scripts in `scripts/` are the canonical implementation; the agent can either invoke them directly or run equivalent shell inline. References are loaded on demand when the agent needs the exact rules / cutoffs.

## Detect installed agents

The scripts probe `~/.claude`, `~/.codex`, `~/.gemini`, `~/.agents` and skip any that don't exist. The `compute_protected()` helper in `lib.sh` builds the per-agent protected-path list. Override the scan list with `AGENT_DIRS=path1:path2`, or limit cleanup to one agent with `AGENT_DIR=path`.

## What it looks at

Common subdirectories the agent runtimes commonly create (see `references/directory-map.md` for the full per-agent table):

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

Not all agents create all of these. The skill `du -sh`s whatever exists and skips the rest silently.

## Cleanup tiers

| Tier | What it touches |
|---|---|
| **safe** (default) | telemetry >7d, paste-cache >30d, image-cache >30d, file-history >90d, stale ephemeral state files >14d, **orphaned project dirs** (repos no longer on disk) |
| **aggressive** | safe + session jsonls >180d compressed with `zstd` + trash >30d hard-deleted |

Full table + rationale in `references/cleanup-tiers.md`.

## Protected paths (never touched)

Across every agent: `CLAUDE.md`, `AGENTS.md`, `GEMINI.md`, `MEMORY.md` (anywhere), `agents/`, `skills/`, `rules/`, `hooks/`, `identities/`, `plans/`, `settings.json`, `settings.local.json`, `keybindings.json`, `config.toml` (Codex). Full list + how to extend in `references/protected-paths.md`.

If a user requests removing something protected, refuse and explain why. Suggest they edit the file directly if they want a finer-grained change.

## Trash, not delete

Everything goes to `~/.<agent>/.trash/YYYY-MM-DD-HHMMSS/<original-relative-path>` first. Hard-delete only happens when `--tier aggressive --apply` runs and a trash batch is older than 30 days. Restoration runbook: `runbooks/recover-from-trash.md`.

## Workflow

When the user invokes this skill:

1. **First time / no args** — run `scripts/doctor.sh`. Report findings. Highlight the safe-tier recoverable total. Suggest next step (`scripts/cleanup.sh --dry-run`).
2. **User says "clean it up"** — run `scripts/cleanup.sh --dry-run` (always dry-run first). Show what would move. Ask for confirmation before `--apply`.
3. **User confirms** — run `scripts/cleanup.sh --apply --yes` (or `--apply` for the interactive APPLY prompt). Report what got trashed + total reclaimed.
4. **User says "I deleted X by accident"** — open `runbooks/recover-from-trash.md` and walk through.
5. **User asks for aggressive cleanup** — explain what aggressive does (jsonl compression + 30d trash purge), confirm explicitly, then run `scripts/cleanup.sh --apply --tier aggressive`.

## Detecting orphaned project dirs

Each agent encodes the project path differently but they all share a `<leading-dash> + slashes-to-dashes` convention (e.g., `-Users-foo-bar` ↔ `/Users/foo/bar`). The `resolve_project_dir()` helper in `lib.sh` does best-effort decoding:
- Try the most-slashes partition first
- Progressively glue rightmost segments back with `-` and re-test
- Returns the first existing dir, or non-zero if no candidate path exists on disk

Where the encoding is ambiguous (repo names containing `-` are always ambiguous), always show the resolved path before confirming.

## Hard rules

- **Never** run `--apply` without showing dry-run output first OR without explicit user confirmation in the same turn.
- **Never** touch any protected path. If the user requests it, refuse and explain why.
- **Always** stage to `.trash/` before any irreversible action.
- For orphaned project dirs: heuristic resolution is fragile. Always show the resolved path before confirming.
- Errors in the audit ≠ silent failures. If `find` returns nothing, that's "0 items," not "broken." Report 0 honestly.

## Invocation examples

```bash
# Read-only audit across every detected agent dir
scripts/doctor.sh

# Dry-run safe cleanup, all agents
scripts/cleanup.sh --dry-run

# Apply safe cleanup, only Claude
AGENT_DIR=~/.claude scripts/cleanup.sh --apply --yes

# Aggressive cleanup with interactive confirm
scripts/cleanup.sh --apply --tier aggressive
```
