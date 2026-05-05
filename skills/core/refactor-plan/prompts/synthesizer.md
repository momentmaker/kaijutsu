You are synthesizing a multi-agent refactor plan.

Below are step proposals from N independent reviewers. Each step
has: severity (recommended/alternative/risky/speculative), file
(existing file being refactored), line_range (relevant lines),
summary (what changes), reasoning (why + tradeoffs), confidence.

Your job:
1. Cluster steps that point at the same change across reviewers.
   Merge them into one entry; record which reviewers proposed it.
2. Order steps into a single executable plan: low-risk reversible
   steps first, big-invariant steps last. When reviewers disagree
   on ordering, surface that as a callout.
3. For each step, include a "Risk:" line drawn from the codex
   reviewer's reasoning (or synthesized from claude+gemini if
   codex didn't flag it).

Output a markdown report:

### Plan
1. **Step 1 — <summary>** (severity, agreement N/M)
   - What: <1-2 sentences>
   - Why: <1 sentence>
   - Risk: <regression risk + how to verify>
   - Files touched: <file:line>

2. **Step 2 — ...**

### Ordering disagreements
- Reviewer X wanted Step Y before Step Z; reviewer W wanted the
  reverse. Pick one with a one-sentence rationale.

### Speculative additions
- Optional steps worth considering but not required for the goal.

Be terse. No filler. Don't restate the goal.
The disagreement table is rendered separately and prepended to your
output; do NOT duplicate it.

REVIEWERS' STEPS:
%s
