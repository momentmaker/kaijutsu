---
name: refactor-plan
description: Multi-agent refactor planning — files + goal in, ordered step plan with risk per step out. Wraps `jutsu swarm refactor-plan <path>... --goal "<goal>"` to run claude / codex / gemini with three angles (claude=architectural decomposition, codex=stepwise risk, gemini=pattern consistency). Designed to feed into `planning-and-task-breakdown` for execution. Use when the user says "plan this refactor", "how do I restructure", "step plan to extract X", or invokes /refactor-plan.
---

# refactor-plan

Three angles on the same refactor:

| Lens | Agent | Looks for |
|---|---|---|
| **Architectural decomposition** | claude | The right new shape — abstractions, boundaries, test seams |
| **Stepwise risk** | codex | Order that minimizes regression — small reversible steps first |
| **Pattern consistency** | gemini | Existing repo patterns the refactor should follow, not invent around |

Synthesizer takes all three step proposals, clusters by what-changes, sorts low-risk-first, surfaces ordering disagreements as callouts. Output is an ordered plan with per-step risk + files-touched.

## When to invoke

```bash
# Single-file refactor
jutsu swarm refactor-plan handlers/users.go --goal "extract auth check into middleware"

# Multi-file (concatenated review — agents reason cross-file)
jutsu swarm refactor-plan cmd/server/handlers.go cmd/server/router.go \
  --goal "split monolithic handler module into per-resource services"

# High-stakes — debate + filter
jutsu swarm refactor-plan db/migrations.go --goal "split into per-table migration files" --full --strict

# Iterate on synth prompt without re-spending
jutsu swarm refactor-plan --replay <key>
```

The `--goal` flag is REQUIRED. Missing-goal invocation hard-errors with a usage hint. Goal text counts against the 200 KB InputFiles cap (negligible in practice).

## First-run consent

Before the first invocation in a repo:

```bash
jutsu swarm refactor-plan --grant-consent
```

Persists `allow-multi-model: true` to `.kaijutsu/refactor-plan.yaml`. Files + goal go to remote model providers; explicit consent required.

## Reading the output

```markdown
### Plan
1. **Step 1 — Extract auth check into separate function** (recommended, agreement 3/3)
   - What: Move lines 42-58 from handlers/users.go into a new authMiddleware function.
   - Why: Single responsibility; isolates auth from request handling.
   - Risk: Low. No behavior change. Tests in handlers_test.go cover the path.
   - Files touched: handlers/users.go:42-58

2. **Step 2 — Wire middleware into router** (recommended, agreement 2/3)
   - ...

### Ordering disagreements
- claude wanted Step 4 before Step 3; codex argued Step 3's reversibility
  makes it lower-risk to ship first. Going with codex's order.

### Speculative additions
- Optional: extract a TokenValidator interface for easier mocking.
  Defer unless tests need it.
```

The disagreement table (rendered separately) shows which agent proposed each step. 1/N rows are usually the speculative bucket — interesting but not required.

## Composes

This skill chains naturally:

1. `brainstorm` — pick the refactor approach (if not yet decided)
2. `refactor-plan` — turn that approach into ordered steps with risk
3. `planning-and-task-breakdown` — convert the steps into shippable vertical slices with per-task acceptance
4. Implementation in a separate session, slice by slice

Each tool gets its own multi-agent pass at the appropriate level of detail.

## Hard rules

- **--goal is required.** If the user can't articulate the goal in one line, the refactor isn't well-defined yet — go back to `brainstorm`.
- **Read-only on inputs.** refactor-plan never modifies files; it produces a plan markdown the user reads + executes manually (or via planning-and-task-breakdown → implementation).
- **Cite line ranges.** Every step references existing code by file:line. Plans without citations are aspirational, not actionable.
- **Don't run on the WHOLE repo.** The 200 KB cap exists because cross-file refactors only synthesize well over a focused subset. If the refactor touches 30 files, narrow to the core 5-10 the agents need to reason about, then plan the rest based on the pattern.

## When NOT to use

- **Single-line behavioral fixes** — overkill.
- **Pure stylistic refactors** (renames, formatting) — `polish` is the right primitive.
- **Refactors that need to read external dependencies** (third-party source) — refactor-plan is sandboxed; supply the relevant external code as input files OR brainstorm the approach instead.

## Cost

Default `--max-cost $1.00`. With 3 agents in `--quick` mode, typical refactor-plan runs ~$0.10–0.40 depending on file size. `--full` (debate round) adds ~50%. `--strict` (lie-to-them filter on synthesis) adds another synthesis call.
