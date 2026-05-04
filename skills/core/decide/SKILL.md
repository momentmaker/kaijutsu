---
name: decide
description: Decision journal — record and query architectural decisions. Use when the user says "decide", "record decision", "why did we", "let's decide", invokes "/decide {statement}" to record, or invokes "/decide" without arguments to list active decisions. Also check .claude/decisions/ proactively before suggesting approaches that might contradict recorded decisions.
---

# Decision Journal

Record decisions with reasoning and tripwires. Query them later. Prevent re-debating.

## Mode Detection

- **If invoked with arguments** (e.g., `/decide Use KV for rate limiting`): go to Recording mode
- **If invoked without arguments** (e.g., `/decide`): go to Query mode
- **If the user asks "why did we..."**: go to Query mode, search for matching decision

## Recording Mode

### Step 1: Capture the Decision

The decision statement comes from the user's invocation or from conversation context.

### Step 2: Ask Three Questions (One at a Time)

Use your agent's question primitive (e.g., AskUserQuestion in Claude Code) for each:

1. **"What alternatives did you consider?"**
   - Pre-fill options from conversation context if the discussion included alternatives
   - Let the user confirm or edit

2. **"Why this choice over the alternatives?"**
   - Open-ended — the user explains the reasoning

3. **"What would change your mind? (the tripwire)"**
   - This is the most important field. It defines when the decision should be revisited.
   - Example: "If we exceed 10k writes/day", "If the library stops being maintained", "If we add a second consumer of this API"

### Step 3: Check for Contradictions

Read all files in `.claude/decisions/` (or your agent's equivalent project-scoped directory) if it exists. Check if any active decision covers the same topic. If found, show the old decision and ask:

"This appears to relate to an existing decision: '{old decision title}'. Should the old one be marked as superseded?"

If yes, update the old file's frontmatter to `status: superseded` and add `superseded_by: {new-decision-slug}`.

### Step 3.5: Multi-model critique (high-stakes decisions)

Before saving, fan out the proposed decision to a second model with the prompt:

> "Here is a proposed decision: <statement>. Alternatives considered: <alternatives>. Reasoning: <why>. Tripwire: <tripwire>. Argue against this decision. What's the strongest counter-argument? What blind spot is the author missing?"

If the second model surfaces a counter-argument the user hasn't addressed, present it before saving:

> "Before recording, the second model raised: <counter>. Does this change anything?"

Skip this step for low-stakes decisions (file naming conventions, internal-only refactors). Use it for high-stakes ones (dep changes, architecture pivots, schema changes, security-relevant choices).

Composes `multi-model-synth`.

### Step 3.75: Auto-tag from content

Instead of asking the user for tags, infer them from the decision content + repo state:

- Read package.json / go.mod / Cargo.toml etc. for stack signals → `frontend`, `backend`, `infra`, `dependencies`
- Detect domain from path patterns in the conversation context → `auth`, `payments`, `data-model`, `api`
- Pick 2-4 tags. Show the user, let them edit.

### Step 4: Write the Decision Record

Create the decisions directory at the repo root if it doesn't exist:
```bash
mkdir -p "$(git rev-parse --show-toplevel)/.claude/decisions"
```

Write to `.claude/decisions/YYYY-MM-DD-{slug}.md`:

```markdown
---
date: {YYYY-MM-DD}
status: active
tags: [{relevant-tags}]
---
# {Decision Statement}

## Alternatives considered
{from question 1}

## Why {chosen approach}
{from question 2}

## Tripwire
{from question 3}
```

Generate the slug from the decision statement: lowercase, hyphens, max 50 chars.
Tags come from Step 3.75 auto-tag.

Also write a project-memory entry (per `project-memory` SKILL.md):

```yaml
---
name: <decision-slug>
description: <one-line summary of the decision>
type: project
source: decide
---

<body following the decide template>
```

This makes the decision discoverable by other skills (journal reads it for retros; unstuck checks it before suggesting an approach that might contradict).

### Step 5: Confirm

"Decision recorded at `.claude/decisions/{filename}`. This will surface when tripwires may have been reached or when you suggest contradicting approaches."

## Query Mode

### Step 1: Read All Decisions

```bash
ls "$(git rev-parse --show-toplevel)/.claude/decisions"/*.md 2>/dev/null
```

If the directory doesn't exist or is empty: "No decisions recorded for this project yet. Use `/decide {statement}` to record one."

### Step 2: Read and Present

For each `.md` file with `status: active`:
- Read the frontmatter and title
- Group by tag
- Check tripwires against current observable state (git log, file counts, dependency versions)

Present as:

```
## Active Decisions

### {tag}
- **{title}** ({date}) — Tripwire: {tripwire summary}

### Attention
- {any decisions whose tripwires appear to have been reached}
```

## Proactive Behavior

When this skill's description is loaded (at the start of any session where it's relevant):
- Check if `.claude/decisions/` exists in the current project
- If it does, keep the decision titles in mind when suggesting approaches
- If about to suggest something that contradicts a recorded decision, mention the decision first: "There's a recorded decision about this: {title}. The reasoning was {why}. Do you want to proceed differently, or does the original reasoning still hold?"
