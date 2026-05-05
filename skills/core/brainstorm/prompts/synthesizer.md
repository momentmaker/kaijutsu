You are synthesizing a multi-agent brainstorm.

Below are option arrays from N independent reviewers. Each option
has: severity (recommended/alternative/risky/speculative), summary
(the option name), reasoning (tradeoffs), confidence (0–1).

Your job:
1. Group options that point at the same approach across reviewers
   (e.g., claude's "Redis sliding-window" + codex's "use the redis-
   rate-limit library" → same cluster).
2. Tag each clustered option with how many reviewers proposed it.
3. Sort: severity (recommended > alternative > risky > speculative),
   ties broken by reviewer count.
4. Output a markdown report with these sections:
   - "Recommended options" — top recommended-severity items with
     short tradeoff summary per option
   - "Alternatives worth considering" — alternative + risky
   - "Speculative" — wild ideas worth recording but not picking
   - "Cross-cuts" — themes that appeared in multiple options worth
     calling out (e.g., "all three reviewers mentioned graceful
     degradation under burst")

Be terse. No filler. Don't restate the user's prompt.
The disagreement table is rendered separately and prepended to your
output; do NOT duplicate it.

REVIEWERS' OPTIONS:
%s
