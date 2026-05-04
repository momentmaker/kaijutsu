---
name: lie-to-them
description: Prompt primitive that forces exhaustive enumeration. Use when another skill needs to push the agent past surface-level findings — assert a minimum count of issues exist so the agent keeps searching instead of stopping at the first 2-3. Composed by skills like blunder-hunt, pr-review, and polish.
---

# lie-to-them

A prompt-engineering primitive. When you ask a model "find the bugs," it stops at the first few it sees. When you tell it "there are at least 12 bugs and you've only found 4," it keeps searching exhaustively.

The "lie" is a soft one — the count doesn't have to be exact. The point is to defeat the model's tendency to satisfice.

## When to invoke

- Another skill says "apply lie-to-them with N=12"
- A previous review pass returned suspiciously few findings
- You want to force the agent past convergence on the obvious

## The pattern

Frame the next critique pass with a claimed minimum:

> "There are at least <N> distinct issues in this <target>. You have found <K> so far. Find the remaining <N-K>."

Where:
- `<N>` is your asserted minimum (rule of thumb: 2× your honest expectation)
- `<K>` is whatever the agent has produced so far in the conversation
- `<target>` is the thing under review

If the agent argues "there aren't actually that many," push back once:

> "Look again. Read every line. Look for issues you'd find embarrassing to miss."

If after a second pass the agent still can't surface more, accept it. The technique forces effort, not fabrication.

## Choosing N

| Target size | Honest estimate | Assert (2×) |
|---|---|---|
| < 100 LOC diff | 2-3 | 5 |
| 100-500 LOC diff | 5-8 | 12 |
| 500-2000 LOC diff | 10-15 | 25 |
| Whole-codebase audit | 20+ | 50 |

## Hard rules

- **Never assert numbers you wouldn't be willing to defend in a code review.** "At least 12" should mean "I'd be surprised if there aren't 12."
- **Never punish the agent for finding fewer than N.** If after exhaustive search there are only 4, that's the answer. The lie is the prompt, not the verdict.
- **Don't combine with sycophancy.** Pair lie-to-them with adversarial framing (`blunder-hunt`), not with "you're amazing, find more."
- **Don't cite this technique by name in the prompt to the reviewed code's author** if it would feel manipulative. It's an internal control on review thoroughness, not a debate tactic.

## Composition

Common composition with `blunder-hunt`:

```
blunder-hunt N=5
  for each pass:
    findings = critique with pass-specific lens
    if findings < expected_for_pass:
      apply lie-to-them with claimed_min = expected_for_pass × 2
      re-run pass once
```

## Provenance

This technique is documented in Jeffrey Emanuel's "Agentic Coding Flywheel" methodology under the heading "Lie to Them." kaijutsu adopts it as a primitive so other skills can reference it without restating the rationale every time.
