# Abort + resume — runbook

## State file

`.kaijutsu/autopilot-state.md` — YAML frontmatter persists pipeline state across phases.

```yaml
---
intent: "<original intent>"
phase: brainstorm | spec | plan | build | ship | learn | done
slug: <feature-slug>
approved_spec_path: docs/specs/<date>-<slug>.md
plan_path: IMPLEMENTATION_PLAN.md
branch: feat/<slug>
pr: <url-or-empty>
started_at: <ISO timestamp>
gate_responses: { brainstorm: approve | revise | reject }
cost_so_far_usd: <running total>
---
```

State file is updated at each phase transition. Used by `resume`, `abort`, and `status` commands.

## Resume

Triggered when:
- User runs `/autopilot resume` inside an agent CLI session
- User runs `jutsu autopilot resume` (prints state, doesn't continue — invoke /autopilot to continue)
- Session was interrupted mid-run (state file exists but no active pipeline)

Resolution algorithm:
1. Read `.kaijutsu/autopilot-state.md`.
2. Verify artifacts named in state still exist (spec path, plan path, branch).
3. If any artifact is missing, fall back to the last phase whose artifacts are intact.
4. Resume from `phase + 1` with the same `intent` and config.

Edge cases:
- **Missing state file**: print "no autopilot run in progress; use /autopilot to start one".
- **Branch deleted externally**: warn + ask user to reset or abort.
- **PR already merged**: jump to Phase 6 (LEARN).
- **Cost from prior run**: `cost_so_far_usd` carries through; new burns add to the same cap.

## Abort

Triggered by `/autopilot abort` or `jutsu autopilot abort`.

1. Read state file for artifact locations.
2. Ask user (unless `--yes`) for the abort reason — informs the LEARN phase even on abort.
3. Write a brief learning memory entry: `autopilot-run-{date}-{slug}: aborted at {phase} — {reason}`.
4. Optionally clean up:
   - git worktree (if created)
   - feature branch (if no commits or user confirms)
   - draft PR (if opened)
5. Delete `.kaijutsu/autopilot-state.md`.
6. Spec + plan paths are PRESERVED so user can resume manually later.

## Status

Read-only. `jutsu autopilot status` (or `/autopilot status`) prints the YAML frontmatter from the state file. No state file → "no autopilot run in progress".
