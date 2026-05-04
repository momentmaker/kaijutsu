---
name: journal
description: Monthly development journal. Use when the user says "journal", "monthly journal", "what did we do this month", "monthly recap", "development diary", or invokes /journal. Optionally /journal {month} for a specific month (e.g., /journal march, /journal 2026-03).
---

# Monthly Development Journal

Generate a structured narrative of what happened in this project over a month. Reads git log, merged PRs, decisions, and any project memory the agent has access to.

## Cadences

Default is monthly. Other cadences accept the same data but produce different narratives:

- `/journal weekly` — last 7 days. Lighter narrative; focus on what shipped + what's next.
- `/journal sprint` — last sprint length (default 14 days; configurable via `.claude/journal-cadence.yaml`).
- `/journal release` — since the last git tag. Releases-as-units narrative.
- `/journal monthly` (default) — calendar month, full retrospective.

The data-gathering steps (git log, PRs, decisions, memory) are identical; only the time window and narrative emphasis differ.

## Step 1: Determine the Window

Resolve the time window by cadence:

| Cadence | Window resolution |
|---|---|
| `monthly` (default) | Previous calendar month if today is day 1-7, otherwise current month. Argument may be a month name (`march`) or `YYYY-MM`. |
| `weekly` | Previous ISO week if today is Mon/Tue, otherwise current ISO week. Argument may be `YYYY-WW`. |
| `sprint` | Last sprint length ending today (default 14 days; configurable via `.claude/journal-cadence.yaml`). |
| `release` | Since the most recent git tag (use `git describe --tags --abbrev=0`). End at HEAD. |

Compute `start_date` and `end_date` accordingly. For monthly, `start_date` = first day of month, `end_date` = first day of next month. For weekly, ISO week boundaries. For sprint, today minus sprint length. For release, the tag's commit date and HEAD's commit date.

Check if the would-be filename (per Step 6's table) already exists. If so, ask: "A journal for {window-label} already exists. Regenerate it?"

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

## Step 4.5: Sentiment + velocity tracking

While scanning, surface signals beyond the raw what-happened:

- **Velocity**: commits per day, ratio vs prior period. Flag a 50%+ drop or rise.
- **Crunch indicators**: late-night commits (after 22:00 local), weekend commits in projects that don't normally see them, commit-message tone shift toward "fix"/"hotfix"/"revert" prefixes.
- **Demoralization signals**: long branches that never merge, repeated reverts, decision tripwires firing without acknowledgment.

Present these in a separate "Signals" section of the journal — factual, no judgment. The user reads and acts.

## Step 4.75: Multi-model synthesis (optional)

For high-effort journals (monthly, release), optionally compose `multi-model-synth`: give the same gathered data to 2-3 models, compare narratives, synthesize the strongest. Different models emphasize different things — one might catch the velocity dip, another the architectural drift. Synthesis preserves both.

Skip for weekly/sprint (cost > value at small windows).

## Step 5: Present for Review

Show the full journal in the conversation. Ask:

"Here's the journal for {window-label}. Anything you'd like to add, change, or remove before I save it?"

## Step 6: Save

Create the journal directory if needed:
```bash
mkdir -p "$(git rev-parse --show-toplevel)/.claude/journal"
```

Filename by cadence:

| Cadence | Filename |
|---|---|
| `monthly` (default) | `.claude/journal/YYYY-MM.md` |
| `weekly` | `.claude/journal/YYYY-WW.md` (ISO week) |
| `sprint` | `.claude/journal/sprint-<start-YYYY-MM-DD>.md` |
| `release` | `.claude/journal/release-<tag>.md` |

Confirm: "Journal saved to <path>."
