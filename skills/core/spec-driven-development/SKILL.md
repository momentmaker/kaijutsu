---
name: spec-driven-development
description: Author a PRD-style spec before writing implementation. Use when the user says "write a spec", "PRD", "design doc", "spec out", "let's plan this", or invokes /spec. Produce a written, reviewable artifact (problem, scope, non-goals, acceptance criteria, open questions). Run a fresh-eyes review before declaring done.
---

# Spec-Driven Development

> Adapted from [`addyosmani/agent-skills`](https://github.com/addyosmani/agent-skills/blob/main/skills/spec-driven-development) under the MIT License. Copyright (c) Addy Osmani. Modifications by kaijutsu maintainers — wrapped in the kaijutsu schema, composed with `blunder-hunt` and `decide`, and given a final review pass.

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

### Step 6: Final review (DO NOT skip)

The reason this skill ships with `deps.skills: blunder-hunt`. Before declaring the spec done:

1. Invoke `blunder-hunt` on the spec file with these lenses:
   - **Internal contradictions** — does Section A claim something Section B disallows?
   - **Hidden assumptions** — what's implicit that should be explicit?
   - **Unfalsifiable criteria** — any "improve UX" / "make it better" lurking?
   - **Scope leakage** — anything in scope that should be a separate spec?
   - **Missing rollback / kill-switch** — what's the un-do plan if this ships and goes wrong?
2. Apply findings inline. Re-save.
3. If the spec touched architectural decisions, also invoke `decide` to record them as ADRs (decision journal entries).

A spec without a fresh-eyes review pass is a draft, not a spec.

## Anti-rationalization table

| Excuse | Rebuttal |
|---|---|
| "I'll just code it; the spec will write itself" | Specs surface disagreement BEFORE expensive implementation. A 30-min spec saves a day of rework. |
| "The user already explained what they want" | The user explained ONE solution. The spec separates problem from solution and reveals other valid approaches. |
| "We'll add acceptance criteria during implementation" | Implicit criteria become political fights at PR review. Write them upfront, fight about them once. |
| "Open questions can be resolved later" | Unresolved questions usually compile into bugs or shipping the wrong thing. Resolve them now. |
| "It's a small change, I don't need a spec" | If it's actually small (one file, one obvious step), agreed — skip. If you're rationalizing, it's not small. |

## Hard rules

- **The spec is the deliverable of this skill.** Implementation happens in a separate session, after the user signs off on the spec.
- **Never skip the final review pass.** A spec that hasn't been blunder-hunted is a draft, period.
- **Save to disk before declaring done.** Conversation specs vanish; file specs survive.
- **Acceptance criteria must be checkable.** Drop anything that depends on subjective judgment.
- **Non-goals are not optional.** Every spec has at least 3 explicit non-goals or it's not finished.

## Output format

```
## Spec: <short title>

**Saved to:** `docs/specs/<filename>.md`
**Reviewed by:** blunder-hunt (5-lens), decide (if architectural)
**Open questions:** <count> blocking, <count> non-blocking
**Status:** ready for review / needs user input / implementation green-light
```

## Composes

- `blunder-hunt` — final-review lens application before declaring the spec done
- `decide` — record any architectural decisions surfaced during spec authoring

## When NOT to use

- True one-line bug fix (`s/foo/bar/`)
- Pure refactor with no semantic change (and a passing test suite)
- Time-bounded experiment that the user explicitly framed as throwaway

For everything else, write the spec.
