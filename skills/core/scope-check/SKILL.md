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

## Step 4.5: Cost-tier classification

For each changed file, classify by **reasoning space** per the Agentic Coding Flywheel cost model:

| Tier | What it touches | Rework cost |
|---|---|---|
| **plan-space** | design docs, plans, architecture markdown, ADRs | 1× |
| **bead-space** | task lists, scope-of-work specs, project tracking | 5× |
| **code-space** | actual implementation files, tests, configs | 25× |

Tally by tier. If 80%+ of the change is **code-space** and the original intent was a small fix, that's a signal: the work drifted into expensive territory. The mirror should surface:

> "Currently 12 of 15 changed files are in code-space (25× rework cost). If a piece of the original plan is still being debated, consider returning to plan-space briefly before committing more to code."

## Step 4.75: Identify split-points

Scan the file groups from Step 3. Look for natural seams:

- Two domains that don't share imports or types → could be two PRs
- One domain has tests + implementation; the other has only implementation → split: ship the tested one first
- Refactor changes mixed with feature changes → split: refactor PR, then feature PR
- Independent bug fixes bundled with the main feature → split: ship the bug fixes immediately

For each candidate split-point, propose:

> "Possible split: extract `<domain>` (N files, ~M LOC) as a separate PR. It has no test or runtime dependency on the rest."

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

### Cost tier breakdown
| Tier | Files | Lines | Rework cost |
|---|---|---|---|
| plan-space | {n} | {n} | 1× |
| bead-space | {n} | {n} | 5× |
| code-space | {n} | {n} | 25× |

### Possible split-points
- {domain} ({n} files, ~{m} LOC) — no shared deps with the rest

**Observation:** {factual description of how the scope grew — what domains were added beyond the original intent, plus the tier breakdown's implications}

**Questions to consider:**
- Could any of these domains be a separate PR?
- Is every new subsystem necessary for v1, or could it be simpler?
- What's the smallest shippable slice of this work?
```

Do NOT tell the user to stop or reduce scope. Present the facts and let them decide. The value is in making the invisible visible.
