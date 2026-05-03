---
name: scope-check
description: Mid-task scope mirror. Use when the user says "scope check", "am I scope creeping", "how big has this gotten", "check the scope", or asks about the size of current work. Also use proactively when you notice the work has grown significantly beyond the original ask.
---

# Scope Check

Hold up a mirror — compare the original task intent to the actual work done. Present data, not judgment.

## Step 1: Find the Original Intent

Check these sources in order. Use the first one found:

1. **Active plan file** — search your agent's plan/scratch directory for a plan referencing this project. Read the Goal and Context sections.
2. **Task list** — if your agent has a tracked task list, the earliest task often captures the original intent.
3. **Ask the user** — if neither exists, ask: "What was the original task or goal? I want to compare it to what's actually happened."

## Step 2: Measure the Actual Work

Detect default branch first: `git symbolic-ref refs/remotes/origin/HEAD 2>/dev/null | sed 's|refs/remotes/origin/||'` — fall back to `main` if unset.

Run in parallel:

- `git diff --stat` — unstaged changes
- `git diff --cached --stat` — staged changes
- `git diff {default-branch}...HEAD --stat` — all changes on this branch vs default (skip if current branch IS the default branch)
- `git ls-files --others --exclude-standard | wc -l` — new untracked files
- Count new vs modified files
- Sum lines added/removed

## Step 3: Categorize by Concern

Group changed files by their top-level directory or domain:
- Read the list of changed files
- Group by the first 2 path segments (e.g., `Models/Proximity/`, `Scenes/ActiveWalk/`, `src/handlers/`)
- Count files and lines per group

## Step 4: Check Time Spent

If your agent keeps a session log, count sessions and estimate total time spent on this project today and yesterday.

## Step 5: Present the Mirror

```
## Scope Check

**Original task:** "{intent from Step 1}"

### What's been touched
| Domain | New | Modified | Lines |
|--------|-----|----------|-------|
| {domain} | {n} | {n} | {n} |
| **Total** | **{n}** | **{n}** | **{n}** |

**Sessions:** {count} | **Estimated time:** {hours}

**Observation:** {factual description of how the scope grew — what domains were added beyond the original intent}

**Questions to consider:**
- Could any of these domains be a separate PR?
- Is every new subsystem necessary for v1, or could it be simpler?
- What's the smallest shippable slice of this work?
```

Do NOT tell the user to stop or reduce scope. Present the facts and let them decide. The value is in making the invisible visible.
