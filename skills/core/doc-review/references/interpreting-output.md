# How to read doc-review output

Same structure as `pr-review`. Brief recap with doc-specific notes.

## Disagreement table

```
| Finding | Severity | Consensus | claude | codex | gemini |
|---|---|---|---|---|---|
| `SPEC.md:42-58` — Acceptance Criteria untestable as written | issue | 3/3 | ✓ | ✓ | ✓ |
| `SPEC.md:130` — "scope" used inconsistently | minor | 2/3 | ✓ | — | ✓ |
| `SPEC.md:90` — section claims X but no HOW | issue | 1/3 | — | ✓ | — |
```

- **3/3** = consensus. Treat as ground truth, fix it.
- **2/3** = strong signal. Read the dissenter's full review — sometimes they had a good reason to disagree.
- **1/3** = lone wolf. Highest-value row. Either the one agent saw a real gap others missed, OR they hallucinated. Investigate.

## Synthesis prose

The synthesizer's narrative review. Findings explained, sometimes with section names called out ("In Acceptance Criteria at line 42..."). Reasoning is consolidated across reviewers.

## Disagreements section

Explicit callout of 1/N findings. Read these FIRST after the table. They're the conversation-starters that justify multi-agent review existing at all.

## Per-agent stats footer

```
- claude — 5 finding(s) · 1m20s · est $0.04
- codex  — 3 finding(s) · 45s · est $0.02
- gemini — 7 finding(s) · 22s · est $0.01
```

Use to detect:
- One agent producing 5x more findings than others (likely noise — tune that lens)
- Cost outliers
- Latency outliers

## Acting on findings

| Pattern | Action |
|---|---|
| 1+ blocker, 3/3 consensus | Stop. Fix before declaring artifact ready. |
| 1+ blocker, 1/3 consensus | INVESTIGATE — read the lone agent's reasoning, decide if it's real. |
| Issues only, all consensus | Address before declaring ready or accept-with-justification. |
| Minors only | Polish-pass material. Fix or skip per project norm. |
| All info | Acknowledge, usually no action. |

## When the artifact is "ready"

Heuristic: the disagreement table shows mostly 0 findings or 2/3+ consensus on minors. Lots of 1/N findings = artifact still ambiguous in some areas. Iterate.

## Replay loop

```bash
# First run, finds X issues
jutsu swarm doc-review SPEC.md
# stderr shows cache key e.g. abc123def456

# Edit the spec to address findings
$EDITOR SPEC.md

# Re-run (NEW cache key — content changed)
jutsu swarm doc-review SPEC.md

# OR — replay against the OLD spec to compare against same baseline
jutsu swarm doc-review --replay abc123def456
```

The cache key changes when content changes, so each iteration creates a fresh cache. `--replay` is for tuning the synthesis prompt against a fixed input.
