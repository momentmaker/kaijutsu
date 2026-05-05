---
name: planning-and-task-breakdown
description: Decompose a spec into a verifiable task list — vertical slices, explicit dependencies, per-task acceptance criteria. Use when the user says "break this down", "task list", "make a plan", "what are the steps", or invokes /plan. End with `jutsu swarm doc-review <plan.md>` to surface gaps before tasks are considered ready for implementation.
---

# Planning + Task Breakdown

> Adapted from [`addyosmani/agent-skills`](https://github.com/addyosmani/agent-skills/blob/main/skills/planning-and-task-breakdown) under the MIT License. Copyright (c) Addy Osmani. Modifications by kaijutsu maintainers — composed with `scope-check` for cost-tier classification, and the final review pass migrated to `jutsu swarm doc-review` (the Phase-2 universal QA gate).

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

### Step 6: Multi-agent review (DO NOT skip)

Run the plan through `doc-review` — kaijutsu's universal QA gate that orchestrates claude / codex / gemini in parallel with prose-tuned lenses:

```bash
jutsu swarm doc-review docs/plans/<filename>.md
```

(or via the skill wrapper: `~/.claude/skills/doc-review/scripts/run.sh docs/plans/<filename>.md`)

Three lenses each surface different planning failure modes:
- **claude (completeness)** — missing dependencies between slices, hidden state assumptions, unfalsifiable acceptance criteria
- **codex (implementability)** — slices that are "actually two", vague success conditions, untestable steps
- **gemini (consistency)** — drift from the parent spec, contradictions between slice acceptance and DAG order, missing references

Iterate findings:
1. Read the disagreement table FIRST. 1/N findings are conversation-starters worth investigating.
2. Fix issue-level findings + consensus minors.
3. Re-save the plan. Re-run `jutsu swarm doc-review` if substantive (or `--replay <key>` for free synthesis re-render after prompt-tuning).
4. Stop when the table shows zero issue-level findings or only contested-minor / info rows.

### Step 7: Cost-tier classification

After doc-review converges, compose `scope-check` to classify each slice by reasoning space (plan / bead / code) per the cost-tier model. This is per-slice metadata for the implementer, not a review pass.

If `jutsu swarm doc-review` fails with "unknown preset", the doc-review skill isn't installed — `jutsu install doc-review` and retry. (doc-review is in this skill's `deps.skills` so a full `jutsu install planning-and-task-breakdown` should pull it transitively.)

**Version constraint note**: this skill's `deps.skills` pins `doc-review@^0.1`. When doc-review v0.2.0 ships, transitive installs of this skill won't pick it up automatically. If you need the newer version, install it explicitly with `jutsu install doc-review@^0.2`.

## Anti-rationalization table

| Excuse | Rebuttal |
|---|---|
| "I'll figure out the slices as I go" | Slices that emerge mid-implementation become accidentally non-shippable. Plan upfront. |
| "Acceptance criteria slow things down" | Acceptance criteria PREVENT slow things — they prevent the "done? not done?" debate at PR time. |
| "Dependencies are obvious" | They're obvious to the agent that wrote the plan. Future-you / collaborators read the DAG to parallelize. |
| "We'll skip the review" | Plans without review consistently produce slices that block on each other or balloon past 500 LOC. The review IS the deliverable. |
| "doc-review costs money / I'll skip it" | Bounded by `--max-cost` (default $1.00). The cost of catching a bad slice at plan time is 5–20× cheaper than catching it at PR time. |

## Hard rules

- **The plan is the deliverable.** Implementation happens in a separate session, after the user approves the plan.
- **doc-review is non-negotiable.** A plan that hasn't been multi-agent-reviewed is a draft.
- **Save to disk.** Conversation plans evaporate. File plans survive.
- **No slice exceeds 500 LOC estimate.** If it does, split.
- **No "TBD" acceptance criteria.** Every slice has concrete, observable success conditions before implementation begins.

## Composes

- `doc-review` — universal multi-agent QA gate; prose-tuned lenses for the final review.
- `scope-check` — cost-tier classification per slice; flags creep risks. Runs AFTER doc-review converges, not as a review pass itself.

## When NOT to use

- Single-file change where the diff IS the plan
- True experiment / prototype the user explicitly framed as throwaway
