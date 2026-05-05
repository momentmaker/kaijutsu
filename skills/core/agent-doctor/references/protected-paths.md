# Protected Paths

These paths are **never** touched by agent-doctor cleanup, regardless of tier or `--apply` flag. The protection applies inside every scanned agent dir (`~/.claude`, `~/.codex`, `~/.gemini`, `~/.agents`).

## Always protected (per agent dir, where `<d>` is the agent's root)

| Path | Why |
|---|---|
| `<d>/CLAUDE.md` / `<d>/AGENTS.md` / `<d>/GEMINI.md` | Per-agent global instructions |
| `**/MEMORY.md` (any depth) | Per-project memory index — irreplaceable |
| `**/memory/` (any depth) | Per-project memory directory |
| `<d>/agents/` | Subagent definitions |
| `<d>/skills/` | Skills (this includes agent-doctor itself) |
| `<d>/rules/` | Custom rules |
| `<d>/hooks/` | Hook scripts |
| `<d>/identities/` | Identity configs |
| `<d>/plans/` | Saved implementation plans |
| `<d>/settings.json` | Global settings |
| `<d>/settings.local.json` | Local overrides (machine-specific) |
| `<d>/keybindings.json` | Custom keybindings |
| `<d>/ecosystem.yaml` | Ecosystem config |
| `<d>/config.toml` | Codex runtime config |
| `<d>/outreach/`, `outreach-log.md` | Outreach skill state (Claude) |

## Why these specifically

**Memory** (`MEMORY.md`, `memory/`) lives **inside** `projects/`. The cleanup logic for orphaned project dirs MUST exclude memory before suggesting removal. The check in `is_protected()` matches `*/memory/*` and `*/MEMORY.md` regardless of parent directory — so even if a project dir is orphaned, the memory inside is still protected.

**plans/** holds work-in-progress implementation plans. They look like cache (.md files) but they're not — losing one mid-task is destructive.

**hooks/** contains user-authored scripts. Small directory, but losing custom hooks is not recoverable from `git`.

**skills/** contains user-authored skills. Same logic.

**settings.json / config.toml / keybindings.json** are config — never cache.

## How `is_protected()` matches

`scripts/lib.sh` does two things:

1. **Filename rules** (anywhere in tree): `MEMORY.md`, `CLAUDE.md`, `AGENTS.md`, `GEMINI.md`, anything matching `*/memory` or `*/memory/*` is protected unconditionally.
2. **Prefix match** against the per-agent `PROTECTED_PATHS` array (computed by `compute_protected "$AGENT_DIR"` before each scan): any path equal to or under one of those entries is protected.

There are no globs beyond the two filename rules. Add explicit paths to `PROTECTED_PATHS` (in `compute_protected()`) if you need broader coverage.

## Adding to the list

Edit the `compute_protected()` function in `scripts/lib.sh`:

```bash
compute_protected() {
  local d="$1"
  PROTECTED_PATHS=(
    # ...existing...
    "$d/my-custom-dir"
    "$d/notes.md"
  )
}
```

Re-run `doctor.sh` to confirm the path now reports as protected if it shows up in any cleanup target.

## Verifying

Quick check that a path is protected:

```bash
source ~/.claude/skills/agent-doctor/scripts/lib.sh
compute_protected ~/.claude
is_protected ~/.claude/MEMORY.md && echo PROTECTED || echo unprotected
```
