---
name: decide
description: Decision journal — record and query architectural decisions. Use when the user says "decide", "record decision", "why did we", "let's decide", invokes "/decide {statement}" to record, or invokes "/decide" without arguments to list active decisions. High-stakes decisions get a multi-agent review pass via `jutsu swarm doc-review` before commit. Also check .claude/decisions/ proactively before suggesting approaches that might contradict recorded decisions.
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

### Step 3.5: Auto-tag from content

Instead of asking the user for tags, infer them from the decision content + repo state:

- Read package.json / go.mod / Cargo.toml etc. for stack signals → `frontend`, `backend`, `infra`, `dependencies`
- Detect domain from path patterns in the conversation context → `auth`, `payments`, `data-model`, `api`
- Pick 2-4 tags. Show the user, let them edit.

### Step 4: Write the Decision Record (Draft)

Create the decisions directory at the repo root if it doesn't exist:
```bash
mkdir -p "$(git rev-parse --show-toplevel)/.claude/decisions"
```

Write to `.claude/decisions/YYYY-MM-DD-{slug}.md`:

```markdown
---
date: {YYYY-MM-DD}
status: draft
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

Generate the slug from the decision statement: lowercase, hyphens, max 50 chars. Status is `draft` until Step 5 finalizes.

### Step 5: Multi-agent review (high-stakes decisions only)

For high-stakes decisions (dep changes, architecture pivots, schema changes, security-relevant choices), run the draft through `doc-review` — kaijutsu's universal QA gate that orchestrates claude / codex / gemini in parallel:

```bash
jutsu swarm doc-review .claude/decisions/YYYY-MM-DD-{slug}.md
```

(or via the skill wrapper: `~/.claude/skills/doc-review/scripts/run.sh .claude/decisions/<filename>.md`)

Three lenses each surface different decision-record failure modes:
- **claude (completeness)** — is the tripwire concrete and observable? are alternatives genuinely exhaustive? is the "why" specific enough to disambiguate from the alternatives?
- **codex (implementability)** — can the tripwire be objectively measured (vs vague "if it gets slow")? is the chosen approach actionable in this codebase?
- **gemini (consistency)** — does this decision contradict prior records in `.claude/decisions/`? does the reasoning align with the project's stated patterns?

Iterate findings:
1. Read the disagreement table FIRST. 1/N findings are the "argue against" voice — investigate the lone-flagger's reasoning before dismissing.
2. Fix issue-level findings + consensus minors inline (edit the draft).
3. Re-run via `--replay <key>` (free) for synthesis-prompt tuning, or fresh run if content changed.
4. Stop when zero issue-level findings remain or only contested-minor / info rows.

If `jutsu swarm doc-review` fails with "unknown preset", the doc-review skill isn't installed — `jutsu install doc-review` and retry. (doc-review is in this skill's `deps.skills` so a full `jutsu install decide` should pull it transitively.)

For LOW-stakes decisions (file naming conventions, internal-only refactors, throwaway-experiment choices), skip Step 5 — go directly to Step 6.

### Step 6: Finalize

Update the draft's frontmatter from `status: draft` to `status: active`. Also write a project-memory entry (per `project-memory` SKILL.md):

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

### Step 7: Confirm

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

## Composes

- `doc-review` — universal multi-agent QA gate; replaces the previous ad-hoc "fan out to second model with argue-against prompt" pattern. Three prose-tuned lenses give a richer signal than a single counter-argument call.
- `project-memory` — decisions are written to project-memory so other skills surface them.

## When NOT to use Step 5 (multi-agent review)

- Throwaway experiments
- File-naming / formatting / style conventions  
- Decisions reversible in <1 hour
- Anything where the cost of being wrong is bounded by a single function-level rewrite
