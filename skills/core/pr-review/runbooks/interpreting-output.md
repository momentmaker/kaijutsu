# Runbook: interpreting the swarm output

What each section of the rendered comment means + how to act on it.

## Top-line summary

```
**Findings:** 7 total · 2 consensus · 3 contested
```

- **Total** = clustered findings (after cross-agent merge).
- **Consensus** = findings that ALL participating agents flagged.
- **Contested** = 1/N findings (one agent flagged, the rest didn't).

If `total == consensus` and `contested == 0`, the agents agreed on everything — multi-agent value was low this run. If `contested > consensus`, the disagreement signal is high; read the Disagreements section first.

## Disagreement table

See `references/disagreement-rubric.md` for the full rubric. Quick decoder:

- ✓ (severity) means "this agent flagged this finding at this severity"
- — means "this agent did not flag"
- The Severity column shows the MAX across reporters; the per-agent column shows what THAT agent said

## Synthesis

The synthesizer's prose review. Cluster-by-cluster reasoning, organized by severity. The synthesizer is allowed (and expected) to:
- Combine reasoning across agents into one coherent paragraph
- De-duplicate verbose phrasing
- Surface disagreements explicitly

The synthesizer should NOT:
- Drop findings silently
- Re-rank findings counter to severity
- Add findings the source agents didn't flag

If the synthesis does any of those, file the bug with a `--replay <sha>` reproduction — the synthesizer prompt needs tuning.

## Disagreements section

Lists 1/N findings with one-line summaries. THESE ARE THE HIGH-VALUE ROWS. The lone-flagger either saw something real the others missed, or hallucinated. Either way it deserves a read.

Common patterns:
- **codex catches a race the others missed** — codex is trained heavily on bug-fix data; trust this more often than not.
- **claude flags an architectural smell the others didn't** — claude's lens is high-level; verify by reading the cited file in full.
- **antigravity flags a cross-file consistency issue the others didn't** — likely correct; antigravity's strength is breadth.

## Per-agent stats footer

```
- claude — 4 finding(s) · 14s · est $0.082
- codex  — 5 finding(s) · 18s · est $0.094
- antigravity — 3 finding(s) · 9s  · est $0.041
```

Use this to:
- Diagnose noise (one agent producing 5x the findings of others = suspicious)
- Track cost trends per repo over time
- Notice latency outliers (one agent timing out)

## Marker line

```
<!-- kaijutsu-pr-review:run-id=20260505T153022Z sha=abc1234 -->
```

This is the idempotency key. Re-runs find the prior comment via this marker and edit-in-place. New SHAs append a "Previous reviews" footer to preserve the timeline.

## When to act

| Situation | Action |
|---|---|
| 1+ blocker, 3/3 consensus | Block merge. Fix before approval. |
| 1+ blocker, 1/3 consensus | INVESTIGATE — read the lone agent's reasoning, decide if it's real. |
| Issues only, all consensus | Address before merge or accept-with-justification. |
| Minors only | Trivial — fix or skip per team norm. |
| All info | Acknowledge in the PR thread; usually no action required. |

## When the result is wrong

- **Hallucinated finding** — file:line doesn't exist in the diff. Vote with --replay + tuned prompt.
- **Missed obvious bug** — likely a per-agent prompt mis-calibration. Check `tuning-prompts.md`.
- **Cost too high** — drop to `--quick`, lower `--max-cost`, or exclude noisy agents via `agents:` in `.kaijutsu/pr-review.yaml`.
