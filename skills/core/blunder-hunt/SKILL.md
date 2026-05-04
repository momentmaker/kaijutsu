---
name: blunder-hunt
description: Multi-pass adversarial critique primitive. Use when another skill or the user invokes /blunder-hunt or asks for a "5x critique", "find what's wrong", or "blunder hunt". Composed by skills like pr-review, polish, and unstuck to surface bugs that single-pass review misses.
---

# blunder-hunt

A primitive for running adversarial critique multiple times in fresh contexts. Single-pass review converges on the first 1-3 issues that come to mind. Multi-pass review with deliberate context resets surfaces the long tail.

This skill is composed by other skills via `deps.skills`. It's also directly invocable.

## When to invoke

- Another skill explicitly references `blunder-hunt` (e.g., `pr-review` runs 5 passes)
- The user says "blunder hunt", "5x critique", "find what's wrong here", "be skeptical"
- You suspect a problem but can't pin it down

## The pattern

Run the critique N times (default 5). Each pass:

1. **Reset attention**. Pretend you've never seen this code/text before. If your agent supports `/clear` or fresh-context windows, use them between passes.
2. **Pick a different lens** for each pass:
   - Pass 1: data correctness — wrong values, off-by-one, type confusion
   - Pass 2: error handling — what's not being caught? what's being swallowed?
   - Pass 3: integration — what breaks at the seams between modules?
   - Pass 4: invariants — what assumption is being silently violated?
   - Pass 5: hostile input — what does an attacker / malformed input do?
3. **Record findings** with: file:line, severity (critical/issue/minor), short description, suggested fix.
4. **Do not deduplicate during passes**. Duplicates across passes = high-confidence findings.

## Synthesis

After all passes:

1. Group findings by file:line. Items appearing in 2+ passes get bumped to high confidence.
2. Drop items that appear once at minor severity.
3. Output the deduplicated list, sorted by severity then file path.

## Lying primitive (optional)

Combine with `lie-to-them` for additional pressure:

> "There are at least 12 distinct issues in this code. You have found 6 so far. Find the remaining 6."

Models continue searching exhaustively rather than satisficing. See `lie-to-them` SKILL.md for usage notes.

## Default invocation

If a parent skill says `run blunder-hunt with N=5 on <target>`, execute exactly that. If the user invokes directly without an N or target:

- N defaults to 3 for small targets, 5 for large.
- Target is whatever's in the current conversation context (the diff, the file, the design doc).

## Output shape

```
## Blunder hunt: <target>

Pass 1 (data correctness): 3 findings
Pass 2 (error handling):    2 findings
Pass 3 (integration):       4 findings
Pass 4 (invariants):        1 finding
Pass 5 (hostile input):     2 findings

Synthesized (deduplicated, severity-sorted):
- [CRITICAL] <file>:<line> — <description>
- [ISSUE]    <file>:<line> — <description>
- [MINOR]    <file>:<line> — <description>
```

## Hard rules

- Each pass must use a different lens. If you re-run pass N with the same lens, you're cheating yourself.
- Do not fabricate findings to fill a quota. If pass 4 finds zero, report zero — let `lie-to-them` do the pressure.
- This skill produces findings. It does NOT take action. The parent skill (pr-review, polish, etc.) decides what to do with them.
