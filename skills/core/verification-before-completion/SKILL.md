---
name: verification-before-completion
description: Hard gate before claiming work is complete. Use when about to say "done", "fixed", "passing", "ready to commit", "ready to merge" — requires running the verification command FRESH and reading actual output before any success claim. Composed by polish, incremental-implementation, and pr-review.
---

# verification-before-completion

> Adapted from [`obra/superpowers/skills/verification-before-completion`](https://github.com/obra/superpowers/blob/main/skills/verification-before-completion/SKILL.md) under the MIT License. Copyright (c) Jesse Vincent. Modifications by kaijutsu maintainers — pared down for kaijutsu's "trust the swarm" stance and reframed as a primitive other skills compose.

A discipline gate. Stops you from claiming work is done based on vibes. Forces evidence.

This skill is composed by other skills via `deps.skills`. Also directly invocable when you catch yourself about to say "should pass".

## What this is NOT

- Not a finding-triage gate. v0.7 quality fingerprinting + multi-agent swarm consensus already handles that.
- Not human ceremony layered on agent output. The agents are trusted; what we're verifying is YOUR CODE, not the agents' findings.
- Not a process for everything. A README typo fix doesn't need a verification gate. Use judgment — see "When to skip" below.

## What this IS

A short checklist between "I think this is done" and "I'm telling the user it's done". The cost of a stale claim is high: lost trust, wasted review cycles, debugging time tomorrow on bugs you said were fixed today.

## When to invoke

- Before saying "tests pass" / "linter clean" / "build succeeds" / "bug fixed"
- Before committing
- Before opening / merging a PR
- After auto-fixers (eslint --fix, prettier, etc.) — tools fail silently more than you think
- When `polish` or `incremental-implementation` says "clean" — convergence ≠ correctness
- After ANY agent (subagent, swarm, etc.) reports success — verify independently

## When to skip

- Pure prose / comment / doc edits with no executable impact
- Research / explore / read-only work
- The verification command itself is what's being changed (chicken-and-egg — note this and let user verify)

## The gate

```
BEFORE claiming any status:
  1. IDENTIFY: What command proves this claim?
  2. RUN: Execute the FULL command, fresh, this turn (not "I ran it earlier")
  3. READ: Full output. Check exit code. Count failures.
  4. VERIFY: Does output confirm the claim?
       NO  → state actual status with evidence
       YES → state claim WITH evidence
  5. ONLY THEN: emit the claim
```

## Claim → command mapping

| Claim | Verifying command | Not sufficient |
|-------|-------------------|----------------|
| Tests pass | full test run (`go test ./...`, `pytest`, etc.) → 0 failures | "I ran it earlier", "should pass" |
| Linter clean | full linter run → 0 errors | partial check, single-file run |
| Build succeeds | full build → exit 0 | linter passing, types check |
| Bug fixed | regression test for the original symptom passes | code changed, "looks right" |
| Regression test works | red-then-green cycle verified | green on first run only |
| Subagent succeeded | git diff shows expected changes | subagent's "success" report |
| Spec satisfied | line-by-line against spec acceptance criteria | tests passing |

## Forbidden phrasings (you're rationalizing)

If you catch yourself typing any of these, STOP and re-verify:

- "should work now"
- "probably passes"
- "seems to work"
- "I'm confident"
- "linter is happy" (linter ≠ tests ≠ build)
- "the agent said it succeeded"
- any victory phrase ("Great!", "Perfect!", "Done!") before the verify command ran

## Output format

When emitting the claim, attach the evidence:

```
✅ ran: go test ./internal/findings/...
   result: ok 6/6 tests · 0.4s
   "verification: tests pass"
```

vs the broken pattern:

```
❌ "All tests pass!" (no command shown, no output, vibes only)
```

If verification fails, state truth + evidence:

```
❌ ran: go test ./internal/findings/...
   result: 1 failure (TestRecordRun_RequiresMeta/missing_RunID)
   "verification failed: 1 test broken; debugging next"
```

## Composition

`polish` invokes this AFTER the convergence-detect "clean" signal, BEFORE emitting "polish complete". Catches the silent-fail tail where the loop converges but tests are actually broken.

`incremental-implementation` invokes per slice, before marking the slice shippable.

`pr-review` invokes on the user's local diff before running the swarm — "claim: my changes compile and tests pass". Saves swarm budget on broken builds.

## The kaijutsu-specific edge

Pair with `polish`'s convergence loop:
- Convergence-detect: "no new signal" — agents stopped finding issues
- Verification gate: "but the code still has to actually run"

Both can be true OR false independently. Verification catches the case where polish converged on a nice-looking diff that doesn't compile. Cheap, high-leverage.

## Anti-pattern: overusing this

Don't perform this gate on tasks that have no executable artifact. Don't add fake verification commands ("ran: cat README.md"). The gate's value is preventing real claim-vs-reality drift; ceremonial application dilutes the signal.
