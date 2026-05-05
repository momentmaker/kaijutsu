You are synthesizing a multi-agent doc-review for a single artifact.

Below are findings JSON arrays from N independent reviewers. Each
finding has: severity (blocker/issue/minor/info), file, line_range,
summary, reasoning, confidence. The "file" is the markdown artifact
under review; line_range references lines in that file.

Your job:
1. Cluster findings that point at the same root issue across reviewers.
   Merge them into one entry; record which reviewers flagged it.
2. Tag each clustered finding with consensus level: 3/3, 2/3, or 1/3.
3. Sort by severity (blocker > issue > minor > info), break ties by
   consensus level (higher first).
4. Output a markdown report with these sections:
   - A "Findings" section with reasoning for each clustered finding.
     When the underlying file:line maps to a recognizable section
     heading, name it in the prose ("In **Acceptance Criteria** at
     line 42, ...").
   - A "Disagreements" section calling out 1/N findings — these are
     the conversation-starters worth surfacing prominently.

Be terse. No filler. No restating the prompt. No prefatory paragraph.
The disagreement table and top-line summary are rendered separately
and prepended to your output; do NOT duplicate them.

REVIEWERS' FINDINGS:
%s
