---
name: journal
description: Monthly development journal. Use when the user says "journal", "monthly journal", "what did we do this month", "monthly recap", "development diary", or invokes /journal. Optionally /journal {month} for a specific month (e.g., /journal march, /journal 2026-03).
---

# Monthly Development Journal

Generate a structured narrative of what happened in this project over a month. Reads git log, merged PRs, decisions, and any project memory the agent has access to.

## Step 1: Determine the Month

- If no argument: use the previous calendar month if today is day 1-7, otherwise use the current month
- If argument is a month name (e.g., "march"): resolve to YYYY-MM using the current year
- If argument is YYYY-MM: use directly

Compute:
- `start_date`: first day of the month (YYYY-MM-01)
- `end_date`: first day of the next month

Check if `.claude/journal/YYYY-MM.md` already exists. If so, ask: "A journal for {month} already exists. Regenerate it?"

## Step 2: Gather Data

Run ALL of the following in parallel:

### Git History
- `git log --oneline --since="{start_date}" --until="{end_date}"` — all commits
- `git log --since="{start_date}" --until="{end_date}" --format="%H %s" --reverse` — commits with hashes for grouping by branch
- `git shortlog -sn --since="{start_date}" --until="{end_date}"` — contributor summary
- Detect default branch: `git symbolic-ref refs/remotes/origin/HEAD 2>/dev/null | sed 's|refs/remotes/origin/||'` (fallback to `main`)

### Merged PRs (if `gh` is available)
- `gh pr list --state merged --limit 30 --json number,title,headRefName,mergedAt --jq '.[] | select(.mergedAt >= "{start_date}" and .mergedAt < "{end_date}")'`

### Releases
- `git tag --sort=-creatordate --format="%(creatordate:short) %(refname:short)"` — filter to tags within the month

### Decisions
- Glob `.claude/decisions/*.md` (or your agent's equivalent) — read each, filter by `date:` frontmatter within the month

### Memory
- If your agent has a project-memory directory, read its index for high-level context, and check filesystem mtime on individual entries to find anything created within the month.

### Open Work
- `gh pr list --state open --json number,title,headRefName,createdAt` — open PRs
- `git branch --no-merged {default-branch} --format="%(refname:short)"` — unmerged branches

## Step 3: Analyze and Group

Before writing the journal, process the raw data:

### Group commits by branch/PR
- Match commits to merged PRs by branch name
- Commits not matching any PR get grouped under "Other work"
- For each group: count commits, count files changed, estimate lines

### Spot patterns
Look for:
- Commit message prefixes (feat/fix/refactor/chore) — compute ratio
- Files modified in 3+ commits (hotspots)
- Same error pattern or bug type appearing multiple times
- Domains touched (group by first path segment)

## Step 4: Generate the Journal

Write the following structure:

    # {Month Name Year} — {project-name}

    ## What We Built

    ### {PR title or branch name}
    {One-sentence summary derived from commit messages}
    - {N} commits, {M} files changed
    - Key changes: {2-3 most significant commits}

    ### {next group}
    ...

    ## What We Shipped
    - {version} ({date}) — {tag or release title}

    ## What We Learned
    - {From memory entries created this month — bullet per entry}
    - {From decision reasoning — what was non-obvious}

    ## Decisions
    - **{decision title}** ({date}) — {one-line why}
      Tripwire: {tripwire}

    ## Activity
    - {N} sessions across {M} days
    - Most active branches: {top 3}
    - Commit breakdown: {X} feat, {Y} fix, {Z} refactor, {W} chore

    ## Patterns
    - {Hotspot files — modified most often}
    - {Recurring themes in commit messages or bug fixes}
    - {Any observation worth noting}

    ## Looking Ahead
    - {Open PRs with age}
    - {Unmerged branches}
    - {Anything flagged in commit messages as TODO or pending}

Omit any section that has zero content. Don't generate empty sections.

## Step 5: Present for Review

Show the full journal in the conversation. Ask:

"Here's the journal for {month}. Anything you'd like to add, change, or remove before I save it?"

## Step 6: Save

Create the journal directory if needed:
```bash
mkdir -p "$(git rev-parse --show-toplevel)/.claude/journal"
```

Write to `.claude/journal/YYYY-MM.md` at the repo root.

Confirm: "Journal saved to `.claude/journal/YYYY-MM.md`."
