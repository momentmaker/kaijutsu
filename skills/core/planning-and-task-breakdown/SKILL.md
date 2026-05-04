---
name: planning-and-task-breakdown
description: Decompose a spec into a verifiable task list — vertical slices, explicit dependencies, per-task acceptance criteria. Use when the user says "break this down", "task list", "make a plan", "what are the steps", or invokes /plan. Run a fresh-eyes review (blunder-hunt + scope-check) before tasks are considered ready for implementation.
---

# Planning + Task Breakdown

> Adapted from [`addyosmani/agent-skills`](https://github.com/addyosmani/agent-skills/blob/main/skills/planning-and-task-breakdown) under the MIT License. Copyright (c) Addy Osmani. Modifications by kaijutsu maintainers — composed with `blunder-hunt` and `scope-check`.

A plan is the bridge between a spec and an implementation. The deliverable is a markdown file with vertical-slice tasks, each individually shippable.

## When to invoke

- A spec exists (see `spec-driven-development`) and the user wants to start work
- The user says "what are the steps" / "how do we approach this" / "task list"
- A change spans >5 files or >500 LOC

## Process

### Step 1: Read the spec

Locate the spec file (usually `docs/specs/<latest>.md`). If no spec exists, refuse — invoke `spec-driven-development` first.

### Step 2: Decompose into vertical slices

Each slice:
- Touches every layer it needs (UI / API / DB / tests / docs)
- Is independently shippable, possibly behind a flag
- Has its own acceptance criteria
- Estimates at <500 LOC, <1 day, <5 files

3-7 slices is the sweet spot. Fewer = you're not slicing enough; more = the spec is too big.

### Step 3: Map dependencies

Make the DAG explicit:

```
Slice 1: <describe> — depends on: nothing
Slice 2: <describe> — depends on: slice 1
Slice 3: <describe> — depends on: nothing  (parallel with 1+2)
...
```

Identify the critical path (longest chain). Any slices off the critical path can be assigned in parallel.

### Step 4: Per-task acceptance criteria

For each slice:
```
Slice N: <title>
- [ ] <observable behavior>
- [ ] <test that passes>
- [ ] <CI green>
```

### Step 5: Write to disk

Save to `docs/plans/YYYY-MM-DD-<short-slug>.md` (or wherever the project's plan convention dictates).

### Step 6: Final review (DO NOT skip)

Compose `blunder-hunt` on the plan with these lenses:
- **Missing dependencies** — slice 4 silently assumes slice 2 finished
- **Hidden state** — what setup does slice 1 require that's not listed?
- **Unfalsifiable acceptance criteria** — any "looks good" / "fast enough" lurking?
- **Slice that's actually two** — is "slice 3" really three sub-slices?
- **Scope leakage from the spec** — did the plan add features the spec didn't authorize?

Then compose `scope-check` to classify each slice by reasoning space (plan / bead / code) per the cost-tier model.

Apply findings inline. Re-save.

## Anti-rationalization table

| Excuse | Rebuttal |
|---|---|
| "I'll figure out the slices as I go" | Slices that emerge mid-implementation become accidentally non-shippable. Plan upfront. |
| "Acceptance criteria slow things down" | Acceptance criteria PREVENT slow things — they prevent the "done? not done?" debate at PR time. |
| "Dependencies are obvious" | They're obvious to the agent that wrote the plan. Future-you / collaborators read the DAG to parallelize. |
| "We'll skip the review" | Plans without review consistently produce slices that block on each other or balloon past 500 LOC. The review IS the deliverable. |

## Hard rules

- **The plan is the deliverable.** Implementation happens in a separate session, after the user approves the plan.
- **Final review is non-negotiable.** Compose `blunder-hunt` AND `scope-check` before declaring done.
- **Save to disk.** Conversation plans evaporate. File plans survive.
- **No slice exceeds 500 LOC estimate.** If it does, split.
- **No "TBD" acceptance criteria.** Every slice has concrete, observable success conditions before implementation begins.

## Composes

- `blunder-hunt` — final-review lens application before declaring the plan done
- `scope-check` — cost-tier classification per slice; flags creep risks

## When NOT to use

- Single-file change where the diff IS the plan
- True experiment / prototype the user explicitly framed as throwaway
