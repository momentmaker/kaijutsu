---
name: incremental-implementation
description: Implement features as thin vertical slices behind feature flags, each shippable on green CI. Use when the user says "implement this", "ship it in stages", "vertical slice", "incremental", or invokes /implement / /build. Reject big-bang refactors; demand a slice plan first.
---

# Incremental Implementation

> Adapted from [`addyosmani/agent-skills`](https://github.com/addyosmani/agent-skills/blob/main/skills/incremental-implementation) under the MIT License. Copyright (c) Addy Osmani. Per-slice two-stage review pattern + continuous-execution rule adapted from [`obra/superpowers/skills/subagent-driven-development`](https://github.com/obra/superpowers/blob/main/skills/subagent-driven-development/SKILL.md) under MIT.

The principle: every PR is mergeable on its own. No "wait for the next one" merges. No multi-week branches. Vertical = touches every layer (UI / API / DB / tests) for one narrow capability rather than completing one layer fully before moving to the next.

## When to invoke

- A spec or plan is ready and the user is about to start coding
- The user says "this should be a multi-PR change"
- A change exceeds 500 LOC or touches >5 files

## Process

### Step 1: Confirm a slice plan exists

If `planning-and-task-breakdown` produced a list of tasks, use that. Otherwise produce a slice plan now: 3-7 vertical slices, each ≤500 LOC, each with its own acceptance criteria.

### Step 2: Pick the first slice

Hardest invariant first if possible (proves the design); easiest first if morale is needed (early win).

### Step 3: Implement behind a feature flag

```
if (featureFlags.kaijutsuV2) { /* new path */ } else { /* old path */ }
```

The flag defaults OFF in main. Old behavior is preserved. New behavior ships when the flag flips.

### Step 4: Tests + green CI

Every slice ends with: passing unit tests for the new code, no regression in existing tests, lint clean, type-check clean. If CI is red, the slice isn't done.

### Step 5: Per-slice two-stage review chain

A slice is NOT complete after the implementer says "done". Run two reviews in order; only after BOTH pass does the slice get marked shippable.

**Stage A — Spec compliance review.**
Dispatch a fresh subagent (clean context, no implementation history) with:
- The slice's declared acceptance criteria from the plan
- The slice's diff (`git diff <base>..<head>`)
- One question: "Does this diff satisfy every acceptance criterion? Pass / fail per criterion + reasons."

If FAIL on any criterion → implementer fixes the gap. Re-dispatch the spec-reviewer (fresh subagent, fresh context) on the new diff. Repeat until PASS.

**Stage B — Code quality review.**
ONLY after Stage A passes, run `polish` on the slice's diff. Polish handles the multi-pass critique + verification-before-completion gate (tests, lint, build all green via fresh runs). Polish surfaces quality issues; implementer fixes; re-run polish until clean.

**Why two stages, not one:** spec-compliance and code-quality are different failure modes. A high-quality slice that misses a criterion is a regression hidden behind nice code; a spec-correct slice with broken tests is a half-shipped feature. Single-stage review conflates them and lets one slip behind the other.

Each stage uses a FRESH subagent with isolated context — no inheritance of the implementer's session history. The reviewer evaluates work product, not narrative.

### Step 6: Scope-check between slices (compose scope-check)

After every 2 slices, invoke `scope-check`. Are we still on the original spec? Did slice 3 add scope that slice 1 didn't? Catch creep early.

### Step 7: Merge + repeat

Slice merged → flag still off → next slice begins from a clean main.

### Step 8: Flip the flag

Last slice merged. Run integration tests with the flag ON. Roll forward in stages (1% → 10% → 100%) if the project supports it.

## Anti-rationalization table

| Excuse | Rebuttal |
|---|---|
| "It's all interconnected; I can't slice it" | If you genuinely can't, the design is wrong. The first slice is "introduce the abstraction that lets the rest slice." |
| "Feature flags are overhead" | Less overhead than a bisected revert. Flags are the cheapest insurance for a multi-week project. |
| "Tests for slice 1 are hard before slice 3 lands" | Mock or stub slice 3's behavior; the test exists to catch regressions in slice 1, not to prove slice 3. |
| "Reviewers can't tell if it's done in slices" | They review per-slice; the spec sets the destination. Tag each PR with the slice number so the through-line is visible. |
| "Big bang is faster" | Big bang is faster to write, slower to review, slower to debug, slower to ship. Net wall-time loss. |

## Hard rules

- **No slice ships without a feature flag** unless the change is a pure no-op refactor with green tests.
- **No slice ships without per-slice tests.**
- **No slice ships on red CI.** Period.
- **No slice ships without BOTH spec-compliance + code-quality review.** (Step 5.)
- **Scope-check between slices.** Not after — between.
- **Each slice fits in <500 LOC.** If you're over, split.
- **Continuous execution between independent slices.** Once the user has approved the slice plan, do NOT pause to ask "should I continue?" between independent slices. Slices were declared independent on purpose; mid-loop check-ins waste their time. Stop only on: a slice fails review and the failure mode genuinely requires user input, an ambiguity blocks progress, or all slices complete.

## Composes

- `scope-check` — cost-tier classification + creep detection between slices
- `polish` — per-slice review-and-fix before merge

## When NOT to use

- True one-line bug fix
- Pure dependency bump where the diff is auto-generated
- Documentation-only changes
