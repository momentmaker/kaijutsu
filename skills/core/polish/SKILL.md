---
name: polish
description: Automated post-implementation review-and-fix loop. Use after completing a feature or implementation plan to find and fix all issues across multiple passes until the code is clean. Invoke with /polish.
---

# Polish Loop

Autonomous review-fix loop that runs until clean. This replaces the manual cycle of "review → fix them all → review again → fix them all" with a single command.

## How It Works

You will perform up to 4 review-fix passes. Each pass:
1. Review all changes thoroughly
2. Identify concrete issues
3. Fix every issue found
4. Verify fixes (run tests/linters)

Stop early if a pass finds zero issues.

## Execution

### Step 0: Understand Scope

Determine what was implemented:
- Read any active implementation plan if it exists
- Run `git diff master...HEAD --stat` (or `git diff --cached --stat` / `git diff --stat` if on master)
- Identify all files changed as part of this feature

### Step 1-4: Review-Fix Passes

For each pass (max 4), do ALL of the following:

**Review Phase** — Examine every changed file for:
- Potential bugs and logic errors
- Missing error handling at system boundaries
- Security issues (injection, XSS, auth gaps, missing RLS)
- Inconsistency with existing codebase patterns
- Dead code, unused imports, leftover debug statements
- Missing or broken tests for new functionality
- Type safety issues
- Race conditions or state management problems
- Missing validation at external boundaries
- Performance concerns (N+1 queries, missing indexes, unnecessary re-renders)

**Report Phase** — List findings with severity:
```
Pass N findings:
- [CRITICAL] file:line — description
- [ISSUE] file:line — description
- [MINOR] file:line — description
```

If zero findings: announce "Pass N: Clean" and stop the loop.

**Fix Phase** — Fix ALL findings from this pass:
- Fix every issue, not just critical ones
- Run the project's test suite after fixes
- Run linters/formatters after fixes
- If a fix introduces a new issue, catch it in the next pass

**Verify Phase** — After fixes:
- Run relevant test commands (check the project's contributor docs for test commands)
- Run linters/analyzers
- If tests fail, fix them before moving to next pass

### Step 5: Summary

After the loop completes (clean pass or 4 passes done), output:

```
## Polish Summary
- Passes completed: N
- Issues found: X (Y critical, Z issues, W minor)
- Issues fixed: X
- Tests: passing/failing
- Status: Clean / N remaining items
```

If items remain after 4 passes, list them for the user to decide on.

## Rules

- DO fix issues immediately — don't just report them
- DO run tests after each fix pass
- DO stop early when a pass is clean
- DO commit after polish is complete (ask user)
- DON'T gold-plate — fix real issues, don't refactor for style
- DON'T add features — this is about correctness, not enhancement
- DON'T count cosmetic preferences as issues unless they violate project conventions
