---
name: unstuck
description: Guided problem articulation when you're stuck. Use when the user says "unstuck", "I'm stuck", "this isn't working", "help me think through this", "I keep going in circles", or invokes /unstuck. Helps break out of unproductive loops by forcing clarity before the next attempt.
---

# Unstuck

Break out of unproductive loops. Articulate the problem, then either solve it through clarity or escalate systematically.

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

Then write a memory entry to your agent's project-memory directory.

Suggested entry shape:

    **Symptom:** {what happened}
    **Root cause:** {what was actually wrong}
    **Fix:** {what solved it}
    **How to apply:** {when this knowledge is useful in the future}

Update the project-level memory index if the root cause is likely to recur.

If the user declines to capture, that's fine — don't push.
