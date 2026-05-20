---
name: autopilot
description: Intent-to-PR pipeline. Describe an outcome in natural language and the system runs the full development pipeline autonomously — brainstorm, spec, plan, build, ship — with multi-agent adversarial review at every artifact stage. Two gates total (post-brainstorm + PR review on GitHub). Cost-capped. Use when the user says "autopilot", "build this end to end", "take this from idea to PR", or invokes /autopilot {intent}.
---

# autopilot v2 — Intent → PR pipeline

Take a natural-language intent and run the full development pipeline to a polished PR. **Two gates total**: brainstorm approval (Gate 1) + PR review on GitHub (Gate 2 — the existing PR semantics). Multi-agent adversarial review replaces the single-reviewer model from v1.

> **NOTE: kaijutsu-managed.** Re-running `jutsu install autopilot` overwrites this file. The previous v1 SKILL.md (if any) is preserved at `~/.claude/skills/autopilot/.archived/SKILL.md`.

## Command Detection

- `/autopilot {intent}` → **Run mode** (start new pipeline)
- `/autopilot resume` → **Resume mode** (continue interrupted run)
- `/autopilot abort` → **Abort mode** (clean up + stop)
- `/autopilot status` → **Status mode** (show current phase)

CLI equivalents: `jutsu autopilot {init,status,abort,resume,run}`. Use the slash command for interactive runs; the cobra group is for scripted use + CI test mode.

## Phase Map

```
/autopilot {intent}
  → Phase 1: BRAINSTORM (interactive — anti-sycophancy via jutsu swarm dream)
  → ★ GATE 1 (configurable: skip | review) ★
  → Phase 2: SPEC
      → write spec to docs/specs/<date>-<slug>.md
      → swarm doc-review on spec [report-only]
      → record approved-spec path in state file
  → Phase 3: PLAN
      → write plan to IMPLEMENTATION_PLAN.md
      → swarm doc-review on plan [report-only]
  → Phase 4: BUILD (stage 1..N — each followed by polish loop)
  → Phase 5: SHIP
      → final swarm pr-review (full lens)
      → reverse-drift gate (informational, tagged on PR)
      → gh pr create  (or test-mode: write planned PR JSON)
  → ★ GATE 2 = PR review on GitHub ★
  → Phase 6: LEARN (state file, findings.db, MEMORY.md)
```

## Cost Cap

Three layers (per spec Decision #3):

1. **Soft cap** — config default $20 (overridable via `.kaijutsu/autopilot.yaml::cost.max_total_usd`); at 50% of soft cap, autopilot pauses + asks user to confirm continue.
2. **Hard ceiling** — `MaxAutopilotCostUSD = $100` constant in `cli/internal/cli/autopilot.go`. Cannot be raised by editing yaml. Per-run override via `--max-cost N` works only up to ceiling.
3. **Per-shell ceiling override** — set `KAIJUTSU_AUTOPILOT_HARD_CAP_OVERRIDE=N` in your shell environment to raise the ceiling for THIS shell. Not stored in repo. Defense against malicious yaml.

Above the ceiling without env var → error. Pre-flight `jutsu swarm <preset> --estimate` projection runs at Gate 1 before any model dispatch.

## Phases (detailed)

### Phase 1: BRAINSTORM (interactive)

Interactive conversation with the user about the intent. Anti-sycophancy enabled by default: invoke `jutsu swarm dream --mode full --strict --personas brainstorm-creative-claude,architecture-purist-antigravity,claim-auditor-claude` against the leading direction. User sees the design ALREADY ARGUED AGAINST before approving Gate 1.

**Step 1: Read project context** (parallel):
- `CLAUDE.md` (project rules + constraints)
- `~/.claude/projects/*/memory/MEMORY.md` matching this project
- `.claude/decisions/*.md` with `status: active`
- `git log --oneline -20`
- Past autopilot runs from memory (entries named `autopilot-run-*`)

**Step 2: Scan for related code** — grep + skill-suggest:
- `jutsu suggest "<intent>"` to see if installed skills already handle sub-problems
- Grep the codebase for keywords from the intent

**Step 3: Anti-sycophancy dream pass** (when autopilot.yaml `gates.brainstorm: review`):
- Run `jutsu swarm dream --mode full` against the leading design
- Surface load-bearing findings + anti-skill arguments to the user

**Step 4: GATE 1** — present design to user with the dream findings already attached:

```
## Autopilot: Direction Review

**Intent:** "{original intent}"
**Leading approach:** {1-paragraph summary}
**Why this approach:** {2-3 sentences}
**Dream's adversarial findings:** {top 3 load-bearing concerns}
**Estimated cost (pre-flight):** ${X}

Approve, revise, or reject?
```

Wait for user response. **Approve** → Phase 2. **Revise** → re-explore. **Reject** → abort.

### Phase 2: SPEC

**Step 1**: Write spec to `docs/specs/<date>-<slug>.md` following the brainstorming spec format.

**Step 2**: Run `jutsu swarm doc-review --personas architecture-purist-antigravity,paranoid-security-claude,claim-auditor-claude <spec-path>`.

**Step 3**: Findings printed to stderr inline + appended to `<workspace>/findings.md`. **Findings are report-only** (per spec Decision #2). Autopilot does NOT mutate the spec automatically. User can apply specific findings via re-run after editing.

**Step 4**: Record approved spec path in `.kaijutsu/autopilot-state.md` (`approved_spec_path` key) for the reverse-drift gate to consume.

### Phase 3: PLAN

**Step 1**: Write `IMPLEMENTATION_PLAN.md` following the writing-plans skill format — bite-sized stages with exact file paths + test commands.

**Step 2**: Run `jutsu swarm doc-review --personas perf-purist-codex,claim-auditor-claude,cross-file-antigravity <plan-path>`.

**Step 3**: Findings report-only (same as spec).

### Phase 4: BUILD

For each stage in the plan:

1. **Implement** the stage — fresh subagent dispatched per task per the subagent-driven-development protocol.
2. **Polish loop** — invoke `/polish` skill against the stage's diff. Up to 4 review-fix passes per stage. If findings remain after 4, escalate to user.
3. **Run tests** — project's test command must pass before moving to next stage.
4. **Commit** — descriptive message linking to the spec + plan.

Track cost across all stages. Pause at 50% of soft cap; abort at hard ceiling without override.

### Phase 5: SHIP

**Step 1: Final swarm pr-review** (full lens) — `jutsu swarm pr-review --diff-from-branch main --strict --personas paranoid-security-claude,architecture-purist-antigravity,perf-purist-codex,claim-auditor-claude --mode full`. Apply load-bearing findings; defensible-skip the rest.

**Step 2: Reverse-drift gate** — `jutsu swarm reverse --spec <state.approved_spec_path> --diff origin/main...HEAD`. Drift findings appended to PR description as `<details>` block. PR labeled `autopilot-drift` if drift surfaced.

**Step 3: Open PR** — `gh pr create` with synthesized body. In test mode (`KAIJUTSU_AUTOPILOT_TEST_MODE=1`), writes planned PR to `.kaijutsu/autopilot-pr.json` instead.

### Phase 6: LEARN

After PR open (or abort):
1. Write run summary to `~/.claude/projects/<project>/memory/autopilot-run-<date>-<slug>.md`.
2. Findings recorded into `~/.kaijutsu/findings.db` (existing v0.7 schema). Each autopilot run adds rows so per-agent confidence weights tune the next run.
3. Update `MEMORY.md` if a lesson is broadly applicable.

## State File

Path: `.kaijutsu/autopilot-state.md`. YAML frontmatter, no body.

```yaml
---
intent: "<original intent>"
phase: brainstorm | spec | plan | build | ship | learn | done
slug: <feature-slug>
approved_spec_path: docs/specs/<date>-<slug>.md
plan_path: IMPLEMENTATION_PLAN.md
branch: feat/<slug>
pr: <url-or-empty>
started_at: <ISO timestamp>
gate_responses: { brainstorm: approve | revise | reject }
cost_so_far_usd: <running total>
---
```

State file enables resume + status + abort to work after session interruption.

## Findings Application Path

Per spec Decision #2: doc-review findings are **report-only**. Never mutate the artifact silently.

- Findings printed to stderr inline during the swarm run.
- Same findings written to `<workspace>/findings.md`.
- User reviews findings; if changes are wanted, user edits the spec/plan directly OR re-runs autopilot after editing.
- Autopilot continues to next phase without applying findings unless severity is `blocker` (per `findings.escalate_threshold` config) — in which case autopilot pauses + asks user how to proceed.

This preserves the reviewer/generator boundary that dream finding #4 surfaced.

## Skill References

See `references/`:
- `config-schema.md` — full `.kaijutsu/autopilot.yaml` reference
- `cost-model.md` — soft cap + hard ceiling + env override
- `persona-defaults.md` — which personas run at which phase + how to override

See `runbooks/`:
- `abort-and-resume.md` — state file format + resume protocol
- `drift-detection.md` — reverse-drift gate behavior + interpretation

## Hard Rules

- **Two gates only**. Brainstorm approval + PR review on GitHub. No mid-flight gates by default.
- **Findings are report-only**. Never auto-apply.
- **Cost ceiling is non-negotiable**. Hard ceiling lives in skill code, not yaml.
- **Reverse-drift is informational, not blocking**. Drift surfaces on the PR; user decides.
- **Replace, don't extend**. v1 autopilot at `~/.claude/skills/autopilot/.archived/SKILL.md` (preserved on install).
