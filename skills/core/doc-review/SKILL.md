---
name: doc-review
description: Universal multi-agent QA gate for markdown artifacts (specs, plans, decision records, RFCs, design docs). Wraps `jutsu swarm doc-review <path>` to run claude / codex / gemini in parallel with tailored lenses (claude=completeness, codex=implementability, gemini=consistency), synthesizes into a single review with a disagreement table. Designed to be the final-review pass for any artifact-producing skill (spec-driven-development, planning-and-task-breakdown, decide). Use when the user says "review this spec", "review this plan", "check this RFC", or invokes /doc-review. Prints to stdout; cache + replay supported.
---

# doc-review

Multi-agent review for markdown artifacts. Specs, plans, decision records, RFCs, design docs — anything written that another human (or another agent) is supposed to act on.

This skill wraps `jutsu swarm doc-review`. Three agents read the same artifact through three lenses, the orchestrator clusters their findings into a disagreement table, and a synthesizer agent writes the prose. Iterate via `--replay <key>` while tuning prompts.

## Why this exists

Before `doc-review`, kaijutsu's artifact-producing skills (`spec-driven-development`, `planning-and-task-breakdown`, `decide`) each invented their own "fresh-eyes pass" mechanism. Each was different, none were systematic. This skill is the universal QA gate they all delegate to.

## Layout (rich)

```
doc-review/
├── SKILL.md                     (this file)
├── skill.yaml
├── scripts/
│   └── run.sh                   thin wrapper around jutsu swarm doc-review
├── prompts/
│   ├── claude.md                completeness lens
│   ├── codex.md                 implementability lens
│   ├── gemini.md                consistency lens
│   ├── synthesizer.md           cluster + render markdown
│   └── debate.md                Pass-2 critique template
├── references/
│   ├── lens-design.md           why each lens is what it is
│   └── interpreting-output.md   how to read the disagreement table
└── runbooks/
    └── tuning-prompts.md        per-repo override workflow
```

The CLI loads prompts from `prompts/` at runtime and falls back to built-in defaults when a file is missing. Per-repo overrides go in `<project>/.claude/skills/doc-review/prompts/<lens>.md`.

## When to invoke

Common entry points:

```bash
# After writing a spec
jutsu swarm doc-review SPEC.md

# Plan + decision review
jutsu swarm doc-review IMPLEMENTATION_PLAN.md
jutsu swarm doc-review .claude/decisions/2026-05-05-auth.md

# Multiple files (single concatenated review)
jutsu swarm doc-review SPEC.md PLAN.md

# --full for a high-stakes doc
jutsu swarm doc-review RFC.md --full --strict

# Iterate on the synthesis prompt
jutsu swarm doc-review --replay <key>
```

## What each lens looks for

**claude — completeness**
- Missing edge cases the artifact should address but doesn't
- Undefined terms or ambiguous phrasing
- Scope creep (sections claiming work outside the stated goal)
- Internal contradictions across sections
- Acceptance criteria that aren't testable as written

**codex — implementability**
- "This section says X but never specifies HOW" — vague directives
- Acceptance criteria that don't say what passes vs fails
- Risks named without mitigations, or mitigations cited without the risk they address
- API/CLI/data-shape claims that contradict the rest of the document
- Numerical thresholds, timeouts, or limits left unquantified

**gemini — consistency**
- Phrases or terms used differently in different sections
- References to other documents/sections/issues that don't resolve
- Contradictions between locked-decisions tables and stage-detail sections
- Drift from the document's own stated patterns
- Heading hierarchy gaps

## Reading the output

Same structure as `pr-review`:

1. **Disagreement table** — rows are clustered findings, columns are agents. The 1/N rows (one agent flags, others didn't) are the conversation-starters. Read those first.
2. **Synthesis** — synthesizer's prose review.
3. **Disagreements section** — explicit callout of 1/N findings.
4. **Per-agent stats footer** — finding counts + costs.

See `references/interpreting-output.md` for the full rubric.

## Cache + replay

Cache key is SHA256(preset + canonical body) → first 12 hex chars. Same artifact bytes → same cache → free `--replay` for tuning the synthesis prompt without re-spending tokens.

```bash
# First run: 3 agents review + synthesize, cache written
jutsu swarm doc-review SPEC.md
# stderr: ... cache key abc123def456 ...

# Edit synthesizer prompt, then replay (no model calls)
$EDITOR .claude/skills/doc-review/prompts/synthesizer.md
jutsu swarm doc-review --replay abc123def456
```

## Workflow when invoked from another skill

`spec-driven-development` ends with: "Run `jutsu swarm doc-review <spec.md>`. Iterate findings until the disagreement table is mostly 2/N consensus or 0 findings."

`planning-and-task-breakdown` ends with: "Run `jutsu swarm doc-review <plan.md>`."

`decide` ends with: "Run `jutsu swarm doc-review <decision-record.md>` for high-stakes decisions."

The artifact-producing skill is responsible for surfacing the findings to the user and iterating. doc-review just runs the swarm.

## Hard rules

- **Never** modify the artifact under review. doc-review is read-only by design — output goes to stdout, the user decides what to change.
- **Never** post comments. doc-review's input isn't tied to a PR; `--post-comment` is silently ignored even if pulled in by accident.
- **Always** cite section names in finding reasoning when possible. "Acceptance Criteria, paragraph 3" is more useful than "line 47".
- **Don't** double-review by running pr-review on the same markdown — pr-review's lenses are code-tuned, doc-review's are prose-tuned. Mixing them produces noise.

## When NOT to use this

- **Code reviews** — use `pr-review` instead.
- **Unstructured chat / brainstorm dumps** — too noisy for the lenses. Tighten the artifact first.
- **Live drafts** — let the author finish a draft before reviewing. doc-review on incomplete writing produces lots of false-positive "missing" findings.
