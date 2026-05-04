---
name: polish
description: Automated post-implementation review-and-fix loop. Use after completing a feature or implementation plan to find and fix issues across multiple passes until convergence. Invoke with /polish. Composes blunder-hunt for the review pass, convergence-detect for the stop condition, and deslop for any user-facing prose at the end.
---

# polish

Loops until clean. Each pass: review the diff, fix everything found, re-run tests/linters. Stop when `convergence-detect` says no new signal is appearing.

This skill composes:
- `blunder-hunt` for the review (multi-pass adversarial critique)
- `convergence-detect` for the stop condition (smarter than "ran N rounds")
- `deslop` for any user-facing prose touched in the diff

## Step 0: Scope

Determine what was implemented:
- Read any active implementation plan if it exists
- `git diff master...HEAD --stat` (or `--cached --stat` / `--stat` if on master)
- Identify all files in the change set

## Step 1-N: Review-Fix Passes

Cap at 4 passes. Stop early when `convergence-detect` says converged.

For each pass, alternate two lenses:

### Lens A — Random Code Exploration (odd passes: 1, 3)

Traverse the codebase semi-randomly:
1. Pick 5 files in the diff at random
2. Read each in full, not just the diff hunks
3. Look for: dead code from refactoring, leftover debug statements, missing error handling at system boundaries, security issues, race conditions, missing tests for new paths
4. For each finding: severity + file:line + description

### Lens B — Cross-Agent Integration Review (even passes: 2, 4)

Look at how pieces fit together:
1. List the public-API surface added or changed in the diff
2. Find every call site of those APIs (in the diff and outside)
3. Verify: contracts honored, error paths handled by callers, no silent type narrowing, no swallowed errors
4. For each finding: severity + file:line + description

### Apply blunder-hunt to each pass

Run `blunder-hunt` with the pass's lens on the in-scope files. The primitive handles the lie-to-them pressure and the dedup. Polish just consumes the output.

### Fix Phase

After each pass, fix every finding (not just critical). Run the project's test/lint commands. If a fix breaks something, fix that too — count as part of the same pass.

### Convergence check

After pass 3 — once you have at least three rounds of findings — invoke `convergence-detect`. If it returns CONVERGED, stop. Otherwise continue to pass 4 (the cap).

## Step F: Final deslop

Before declaring done, scan the diff for user-facing prose: README sections, commit messages, doc strings, error messages shown to humans, CLI help text. Run `deslop` on each.

Skip code comments unless the diff specifically reworked them.

## Step Final: Summary

```
## Polish Summary
- Passes completed: <N>
- Stop reason: <converged | hit cap of 4 | clean from pass 1>
- Issues found: <X> (<Y> critical, <Z> issue, <W> minor)
- Issues fixed: <X>
- Tests: <passing | failing — listed below>
- Deslop edits: <count>
- Status: <Clean | <N> remaining items>
```

If items remain after the cap, list them for the user.

## Hard rules

- DO fix issues immediately — don't just report them
- DO run tests after each fix pass
- DO stop when convergence-detect says converged, even before the cap
- DO commit after polish (ask user first)
- DON'T gold-plate. Real issues only, no style refactors
- DON'T add features. Correctness only
- DON'T count cosmetic preferences as issues unless they violate project conventions
- DON'T silently revert decisions the author made — if you disagree, raise it as a finding for the user to weigh in
