---
name: session-retro
description: End-of-session retrospective that mines the current conversation for learnings. Use when the user says "retro", "session retro", "what did we learn", "extract learnings", "session review", "conversation review", or wants to capture insights before ending a session. Also trigger when the user asks to turn repeated patterns into skills or memory, or says "before we wrap up".
---

# Session Retrospective

Review the current conversation to surface actionable learnings, then help the user decide what to persist.

## Step 1: Scan the Conversation

Read through the full conversation and extract items in these categories:

### Errors & Fixes
- What broke and how it was fixed
- Root causes that were non-obvious
- Workarounds that succeeded after initial approaches failed

### Repeated Patterns
- Sequences of tool calls or steps that appeared 3+ times
- Copy-pasted prompts or instructions given to subagents
- Manual processes that could be automated

### User Corrections
- Times the user redirected your approach
- Explicit preferences stated ("always do X", "never do Y", "I prefer Z")
- Implicit preferences revealed by approvals/rejections

### Knowledge Gained
- Codebase facts discovered (file locations, conventions, gotchas)
- External system behaviors learned (API quirks, tool limitations)
- Configuration or environment details that affected the work

### Workflow Friction
- Steps that took multiple attempts
- Places where context was lost or repeated
- Tasks that would benefit from a hook, skill, or MCP server

## Step 2: Present Findings

Group findings into a concise table:

```
## Session Retrospective

### Potential Memory Entries (persist across sessions)
| # | Finding | Type | Source |
|---|---------|------|--------|
| 1 | R2 bucket uploads need --content-type flag | codebase fact | error at turn 12 |
| 2 | User prefers max 10 concurrent agents | preference | explicit instruction |

### Potential Skill Opportunities (automate repeated work)
| # | Pattern | Frequency | Effort |
|---|---------|-----------|--------|
| 1 | Batch sentence generation with 10-agent concurrency | 5 times | medium |

### Potential Skill Improvements (existing skills that fell short)
| # | Skill | Issue | Suggestion |
|---|-------|-------|------------|
| 1 | generate-japanese | No furigana validation step | Add post-gen validation |

### Potential Hooks / Automation
| # | Trigger | Action | Why |
|---|---------|--------|-----|
| 1 | PostToolUse:Write on *.json | Validate JSON schema | Caught 3 malformed files |
```

Omit any category that has zero findings — don't show empty tables.

After the table, give a one-line recommendation for the single highest-impact item.

## Step 3: Ask What to Implement

Prompt: "Which items should I act on? Enter numbers by category (e.g., 'memory 1,2' or 'skill 1') — or 'all' to do everything."

## Step 4: Execute

For each selected item:

- **Memory entries** → Write or update files in your agent's project-memory directory. Check for existing entries first to avoid duplicates. Update the always-loaded memory index if the finding is important enough; otherwise use a topic file.
- **Skill opportunities** → Draft a new SKILL.md following the [Anthropic Agent Skills](https://agentskills.io) format. Keep it lean — the user can iterate later.
- **Skill improvements** → Read the existing skill, apply the suggested change, show a diff.
- **Hooks / automation** → Create the hook script and add the entry to the agent's settings file, following the agent-specific schema.

After applying changes, summarize what was written and where.
