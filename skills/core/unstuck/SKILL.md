---
name: unstuck
description: Guided problem articulation when you're stuck. Use when the user says "unstuck", "I'm stuck", "this isn't working", "help me think through this", "I keep going in circles", or invokes /unstuck. Helps break out of unproductive loops by forcing clarity before the next attempt.
---

# Unstuck

Break out of unproductive loops. Articulate the problem, then either solve it through clarity or escalate systematically.

> Step 0.5's iron-law framing + multi-component instrumentation pattern adapted from [`obra/superpowers/skills/systematic-debugging`](https://github.com/obra/superpowers/blob/main/skills/systematic-debugging/SKILL.md) under MIT.

## Step 0: Memory-first

Before asking the user anything, check project memory for prior similar problems:

1. Read `MEMORY.md` index, scan descriptions for matches against the current symptom (error message, stack trace, behavior)
2. For any matches, read the full memory entry
3. If a strong match exists, present it: "We've hit something like this before: <prior root cause>. Is this the same problem? If yes, the prior fix was: <fix>."

If the user confirms it's the same problem, jump straight to Step 5 (Solution Capture) — the entry confirms the user already had the answer.

If no match or user says it's different, proceed to Step 0.5.

Composes `project-memory`.

## Step 0.5: Gather evidence

Iron law: **no fix proposals without root-cause investigation first.** Symptom-fixes mask underlying issues and create new bugs. This step is for the AGENT to do legwork BEFORE asking the human articulation questions — so Step 1's discussion is grounded in facts, not memory of facts.

1. **Read the actual error message in full.**
   - Don't paraphrase. Copy the literal stack trace, exit code, log line.
   - Note line numbers, file paths, error codes — the message often contains the exact answer.
   - If the error was N invocations ago and you don't have it, ask the user to re-run with the failure reproduced.

2. **Check recent changes.**
   - `git log --oneline -10` on suspect files
   - `git diff` against last known good
   - New dependencies, config edits, env-var changes
   - The cause is usually in the most recent change, but the SYMPTOM may surface later.

3. **Verify reproducibility.**
   - Does the failure happen every time, or intermittently?
   - Exact steps to reproduce. If intermittent → gather more data, do not guess.
   - If "I keep going in circles" was the trigger, the problem may be that the symptom is non-deterministic.

4. **Multi-component systems: instrument boundaries.**
   - When the system has multiple components (CI → build → deploy, API → service → DB, swarm → cache → render):
     - Log what data ENTERS each component
     - Log what data EXITS each component
   - The bug is at the boundary where input ≠ expected. Find the boundary BEFORE proposing a fix.

5. **Surface the evidence to the user before Step 1.**
   - Present what you found: "Error says X. Last change touched Y. Reproducer is Z."
   - Then ask the Step 1 articulation questions — they're now augmenting your evidence, not replacing it.

Skip this step ONLY when:
- Step 0 found a memory match and the user confirmed it (jump straight to Step 5)
- The user has already provided full evidence in their initial message

If you find yourself wanting to skip because "I'm pretty sure I know what it is" → that's the rationalization the iron law exists to block. Run the evidence-gathering anyway.

## Walk-away timer

If `unstuck` has been invoked 2+ times in the same session on the same goal, escalate Step 4's Walk-Away level immediately. Repeated invocation = fresh-eyes problem, not articulation problem.

## Step 1: Articulate

Ask these 4 questions one at a time. Do not skip any.

1. **"What are you trying to do?"**
   - The goal — the outcome, not the implementation detail.

2. **"What did you expect to happen?"**
   - The assumption being violated.

3. **"What actually happened?"**
   - The concrete symptom: error message, wrong behavior, unexpected state. Be specific.

4. **"What have you tried so far?"**
   - Each approach, briefly. This is the attempt log. Knowing what failed is as valuable as knowing what worked.

## Step 2: Reflect

Read back the articulated problem as a structured summary:

    ## Problem Statement

    **Goal:** {answer 1}
    **Expected:** {answer 2}
    **Actual:** {answer 3}

    ### Attempts
    1. {attempt 1}
    2. {attempt 2}
    ...

Then ask: "Does seeing this written out change anything? Often the answer becomes visible once the problem is articulated."

If the user sees the answer → skip to Step 5 (Solution Capture).

## Step 3: Pivot

If still stuck, suggest 3 fundamentally different angles. Tailor to the actual problem, but the three categories are:

1. **Reverse direction** — "You're debugging from {current direction}. Try from the other end."
   - Top-down → bottom-up (or vice versa)
   - Reading code → writing a test
   - Tracing forward → tracing backward from the symptom

2. **Isolate** — "Can you reproduce this in the smallest possible case? Strip away everything except the broken behavior."
   - Create a minimal test case
   - Comment out everything unrelated
   - Hardcode inputs to eliminate variables

3. **Simplify** — "Remove code until it works, then add back line by line until it breaks."
   - Binary search for the offending change
   - git stash and verify the problem exists on a clean state
   - Revert to last known good and re-apply changes one at a time

Ask which angle to try. Help execute that approach.

## Step 4: Escalate

If pivoting didn't help, work through this ladder:

**Level 1: Search**
- Search for the exact error message
- Grep the codebase for similar patterns that work
- Check if this is a known issue in the library/framework

**Level 2: Read Source**
- Don't read docs — read the actual source code of the library or framework involved
- Find the function that throws the error or produces the unexpected behavior
- Understand what it actually does vs what you assumed

**Level 2.5: Blunder hunt**
- Invoke `blunder-hunt` on the file or function suspected to be broken
- Five passes with different lenses (data, error handling, integration, invariants, hostile input) often surface the cause that single-lens debugging missed
- If `blunder-hunt` is not installed, run the equivalent manually: 5 passes of skeptic critique with deliberately different attention each pass

**Level 3: Walk Away**
- Record the full problem state in your agent's persistent memory:
  - Goal, symptoms, all attempts, what was learned
  - File paths and line numbers involved
  - Any theories not yet tested
- Tell the user: "Problem state saved. Come back with fresh eyes — it'll be here when you return."

## Step 5: Solution Capture

After the problem is resolved (at any step), ask:

"What was the actual root cause?"

Then write a project-memory entry (per `project-memory` SKILL.md):

```yaml
---
name: <symptom-slug>
description: <symptom> -> <root cause>
type: project
source: unstuck
---

**Symptom:** <what happened>
**Root cause:** <what was actually wrong>
**Fix:** <what solved it>
**How to apply:** <when this knowledge is useful in the future>
```

Update `MEMORY.md` if the root cause is likely to recur. Future `unstuck` invocations will find this entry in Step 0 and short-circuit.

Update the project-level memory index if the root cause is likely to recur.

If the user declines to capture, that's fine — don't push.
