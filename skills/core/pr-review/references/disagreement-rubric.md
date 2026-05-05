# How to read the disagreement table

The disagreement table is the headline output. Read it before reading the prose synthesis.

## Anatomy

```
| Finding | Severity | Consensus | claude | codex | gemini |
|---|---|---|---|---|---|
| `auth.go:88` — race in token refresh | blocker | 3/3 | ✓ (blocker) | ✓ (issue) | ✓ (blocker) |
| `cache.go:42` — TTL not honored | issue | 1/3 | — | ✓ (issue) | — |
| `parser.go:117` — recursion limit | minor | 2/3 | ✓ (minor) | — | ✓ (minor) |
```

- **Finding** column = clustered finding (cross-agent merge by file+line).
- **Severity** = the maximum severity any agent assigned. The merge prefers more-alarmed votes over less-alarmed ones.
- **Consensus** = N/M, where N = agents who flagged it and M = agents that participated in this run.
- **Per-agent columns** = severity that specific agent assigned, or `—` if they didn't flag this finding at all.

## Reading the rubric

| Pattern | Meaning | Action |
|---|---|---|
| 3/3, all flag the same severity | Strong consensus blocker/issue. | Treat as ground truth. Fix it. |
| 3/3, severity disagreement | Consensus on existence, not impact. Take the highest severity as the cap. | Read the per-agent reasoning to decide if the diff REQUIRES blocker-level treatment. |
| 2/3 | One agent didn't flag it. Either they missed it or they considered it acceptable. | Read the missing agent's full review — sometimes they explicitly said "this is fine and here's why". |
| 1/3 | Lone wolf. Either the one agent saw a real bug the others missed, OR they're hallucinating. | THIS IS THE HIGHEST-VALUE ROW. Investigate the reasoning before accepting or rejecting. |

## Why 1/3 findings matter most

- If 3/3 agents agree, you barely needed three agents — one would have sufficed.
- If 1/3 dissents (high), the value of the multi-agent run is concentrated in that row.
- The synthesizer surfaces 1/N findings in a "Disagreements" section. Read those FIRST.

## When the table looks empty

Three rows max, all minor. Either:
- The diff is genuinely clean (good).
- The agents had nothing to grab onto (review-fatigue or token-budget exhaustion).

Re-run with `--full` to add a Pass-2 critique. If still empty, trust it.

## When the table is a mess

Lots of 1/3 rows, no consensus, contradictory severities. Indicates:
- Diff is complex enough that no two reviewers prioritize the same things.
- Or each reviewer's prompt is pushing them in incompatible directions.

Inspect the per-agent stats. If one agent generated 20 findings and the others generated 3 each, that one is probably noise. Adjust the prompt for that lens (per-repo override in `.claude/skills/pr-review/prompts/<agent>.md`).
