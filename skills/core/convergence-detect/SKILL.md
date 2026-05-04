---
name: convergence-detect
description: Detect when an iterative loop has stopped producing new signal. Use when another skill (polish, session-retro, plan refinement) needs a stop condition that's smarter than "ran N rounds". Three signals — output shrinking, change rate slowing, content similarity rising — indicate it's time to stop.
---

# convergence-detect

Iterative loops (review-fix loops, plan-polish loops, retrospective extraction) have a natural endpoint: when each new pass mostly restates the previous one, you're done. This skill is a primitive for detecting that endpoint quantitatively, so loops don't stop too early (missed signal) or too late (wasted tokens).

## When to invoke

- Another skill says "use convergence-detect to decide when to stop"
- You're running a multi-round refinement and want a principled stop
- The user asks "are we done iterating?"

## The three signals

Track these across the last 3 rounds (minimum). All three must point the same way before declaring convergence.

### 1. Output size shrinking

Token count of the output decreases round-over-round. New rounds add fewer findings, suggestions, or words than the previous.

| Round | Output tokens |
|---|---|
| N-2 | 1500 |
| N-1 | 800 |
| N   | 350 |

Decreasing → signal to stop.

### 2. Change rate slowing

Of the items in round N, how many are *new* vs. *restated from a previous round*?

| Round | Items | New | Restated |
|---|---|---|---|
| N-2 | 12 | 12 | 0 |
| N-1 | 8 | 5 | 3 |
| N   | 6 | 1 | 5 |

New / total approaching 0 → signal to stop.

### 3. Content similarity rising

Compare round N output to round N-1 output. If they're saying mostly the same things in different words, you've converged.

Practical heuristic without an embedding model:
- Take each finding/item from round N
- Check if a substantively-equivalent item appears in round N-1
- If 80%+ of round N items have a match in round N-1, they're saying the same thing

## Quantitative scoring

Don't eyeball — measure.

- **Output tokens**: count tokens (or characters as a proxy) per round. Track the ratio `tokens(N) / tokens(N-1)`. Below 0.6 = strongly shrinking; below 0.8 = mildly shrinking.
- **New-item ratio**: of the items in round N, count how many are new vs. restated. Items whose normalized text matches an item from N-1 (case-insensitive, whitespace-collapsed, identifier-equivalent) count as restated. Below 0.2 = strongly slowing.
- **Jaccard similarity to previous round**: |intersect(N, N-1)| / |union(N, N-1)|. Above 0.7 = strongly similar.

Optional: when an embedding endpoint is available, replace string-jaccard with cosine similarity over embeddings. More robust to paraphrase.

## Persistence

Optionally log round-by-round signals to `.claude/convergence-log/<loop-id>.jsonl` (one JSON record per round). Schema:

```json
{
  "round": 3,
  "tokens": 350,
  "new_items": 1,
  "total_items": 6,
  "similarity_to_prev": 0.83,
  "verdict": "converged",
  "verdict_confidence": "high"
}
```

Useful for retroactive tuning ("we declared converged at round 3 but pass 4 still found 2 issues — calibrate harder next time").

## Stop condition

Declare convergence when ALL three signals fire simultaneously:

- Output size is shrinking (this round < previous round)
- New-item ratio is below 20%
- 80%+ of items have an equivalent in the previous round

If only 1-2 signals fire, run another round.

## Hard rules

- **Need at least 3 rounds before checking convergence.** Two rounds isn't enough data to spot a trend.
- **Don't conflate "agreement" with "convergence".** If two rounds both produce 0 findings, that might be convergence — or it might be that the agent gave up. Verify by checking: did the agent actually look this round, or did it short-circuit?
- **Convergence is per-loop, not per-skill.** A polish loop on a small change might converge in 2 rounds; a plan-polish loop on a large doc might need 6.

## Output shape

```
## Convergence check (round N)

| Signal             | Round N-2 | Round N-1 | Round N | Direction |
|--------------------|-----------|-----------|---------|-----------|
| Output tokens      | 1500      | 800       | 350     | shrinking |
| New-item ratio     | 100%      | 63%       | 17%     | dropping  |
| Similarity to prev | n/a       | 50%       | 83%     | rising    |

Verdict: CONVERGED (high confidence — all three signals strong)
        | CONVERGED (low — only 1 of 3 signals strong, see notes)
        | NOT CONVERGED — output shrinking but similarity still 60%
```

Or:

```
Verdict: NOT CONVERGED — output is shrinking but similarity is still 60%. Run another round.
```

## Provenance

The three-signal model is from Jeffrey Emanuel's "Agentic Coding Flywheel" under "convergence detection". kaijutsu adopts it so other skills can call it as a primitive instead of redefining the rules.
