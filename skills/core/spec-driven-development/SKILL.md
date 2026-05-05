---
name: spec-driven-development
description: Author a PRD-style spec before writing implementation. Use when the user says "write a spec", "PRD", "design doc", "spec out", "let's plan this", or invokes /spec. Produce a written, reviewable artifact (problem, scope, non-goals, acceptance criteria, open questions). End with `jutsu swarm doc-review <spec.md>` to surface gaps before declaring done.
---

# Spec-Driven Development

> Adapted from [`addyosmani/agent-skills`](https://github.com/addyosmani/agent-skills/blob/main/skills/spec-driven-development) under the MIT License. Copyright (c) Addy Osmani. Modifications by kaijutsu maintainers — wrapped in the kaijutsu schema, composed with `decide` for ADR capture, and the final review pass migrated to `jutsu swarm doc-review` (the Phase-2 universal QA gate).

A spec is a workflow artifact, not an essay. The deliverable is a markdown file that the user (and future agent sessions) can read in 5 minutes and act on.

## When to invoke

- User describes a problem fuzzy ("we should make it faster", "users keep getting confused")
- User asks for "a plan" before any implementation begins
- A change spans more than one file or one obvious step
- The agent is about to start coding without first writing what they're going to build

## Process

### Step 1: Capture the problem

Ask the user — or write down — what problem this solves. Not the solution, the problem. Format:

```
Problem: <one sentence>
Who feels it: <user persona, role, or system>
When: <conditions / triggers / frequency>
Today's workaround: <what people do now>
Cost of doing nothing: <what breaks / stays broken>
```

If you can't fill this in cleanly, the spec is premature — go back to the user with clarifying questions.

### Step 2: Scope + non-goals

Two equally important sections:

```
In scope: <bullet list — what this change WILL deliver>
Non-goals: <bullet list — what this change explicitly does NOT cover>
```

Non-goals are the discipline. Without them every spec becomes "fix everything." Aim for at least 3 explicit non-goals.

### Step 3: Acceptance criteria

Concrete, observable, falsifiable:

```
- [ ] <user-facing behavior or system property>
- [ ] <test that passes>
- [ ] <metric that moves>
```

If a criterion can't be checked by reading the diff, running a command, or observing behavior, it's prose — drop it.

### Step 4: Open questions + risks

```
Open questions:
- Q1: <decision the user / team needs to make>
- Q2: ...

Risks:
- <thing that could derail this; mitigation if known>
```

Open questions block implementation until resolved. Risks are documented but don't necessarily block.

### Step 5: Write to disk

Save the spec to:

```
docs/specs/YYYY-MM-DD-<short-slug>.md
```

(or wherever the project's spec convention dictates — read CLAUDE.md / AGENTS.md / contributing docs).

### Step 6: Multi-agent review (DO NOT skip)

Run the spec through `doc-review` — kaijutsu's universal QA gate that orchestrates claude / codex / gemini in parallel with prose-tuned lenses (completeness, implementability, consistency):

```bash
jutsu swarm doc-review docs/specs/<filename>.md
```

(or the equivalent skill wrapper: `~/.claude/skills/doc-review/scripts/run.sh docs/specs/<filename>.md`)

The output is a synthesized markdown review with a disagreement table. Iterate findings:

1. Read the disagreement table FIRST. 1/N findings (lone-wolf) are highest-leverage — the one agent that flagged it either saw a real gap others missed OR hallucinated. Investigate before dismissing.
2. Fix issue-level findings (consensus or not). Fix consensus minor findings.
3. Re-save the spec. Re-run `jutsu swarm doc-review` if the changes were substantive (note: NEW content = new cache key = re-spend; same content = `--replay <key>` is free).
4. Stop when the disagreement table shows zero issue-level findings or only contested-minor / info-level rows.

If `jutsu swarm doc-review` fails with "unknown preset", the doc-review skill isn't installed — `jutsu install doc-review` and retry. (doc-review is in this skill's `deps.skills` so a full `jutsu install spec-driven-development` should pull it transitively.)

### Step 7: ADR capture (if architectural)

If the spec touched architectural decisions, also invoke `decide` to record them as ADRs (decision journal entries). Decisions made during spec authoring deserve the same record-keeping as decisions made during implementation.

## Anti-rationalization table

| Excuse | Rebuttal |
|---|---|
| "I'll just code it; the spec will write itself" | Specs surface disagreement BEFORE expensive implementation. A 30-min spec saves a day of rework. |
| "The user already explained what they want" | The user explained ONE solution. The spec separates problem from solution and reveals other valid approaches. |
| "We'll add acceptance criteria during implementation" | Implicit criteria become political fights at PR review. Write them upfront, fight about them once. |
| "Open questions can be resolved later" | Unresolved questions usually compile into bugs or shipping the wrong thing. Resolve them now. |
| "It's a small change, I don't need a spec" | If it's actually small (one file, one obvious step), agreed — skip. If you're rationalizing, it's not small. |
| "doc-review costs money / I'll skip it" | The cost is bounded by `--max-cost` (defaults to $1.00) and you'll usually spend 1–3× less than the iteration time you'd waste on a spec gap caught at PR review. Skip only on genuinely tiny specs. |

## Hard rules

- **The spec is the deliverable of this skill.** Implementation happens in a separate session, after the user signs off on the spec.
- **Never skip the doc-review pass.** A spec that hasn't been multi-agent-reviewed is a draft, period.
- **Save to disk before declaring done.** Conversation specs vanish; file specs survive.
- **Acceptance criteria must be checkable.** Drop anything that depends on subjective judgment.
- **Non-goals are not optional.** Every spec has at least 3 explicit non-goals or it's not finished.

## Output format

```
## Spec: <short title>

**Saved to:** `docs/specs/<filename>.md`
**Reviewed by:** doc-review (`jutsu swarm doc-review`)
**Open questions:** <count> blocking, <count> non-blocking
**Status:** ready for review / needs user input / implementation green-light
```

## Composes

- `doc-review` — universal multi-agent QA gate; ships the prose-tuned lenses (completeness / implementability / consistency) the final review uses.
- `decide` — record any architectural decisions surfaced during spec authoring as ADRs.

## When NOT to use

- True one-line bug fix (`s/foo/bar/`)
- Pure refactor with no semantic change (and a passing test suite)
- Time-bounded experiment that the user explicitly framed as throwaway

For everything else, write the spec.
