# IMPLEMENTATION_PLAN.md — v0.11.0

Spec: `docs/specs/2026-05-07-v0.11.0-autopilot.md`
Branch: `v0.11.0-autopilot`

## Stage 1: `swarm.RunPipeline` extraction + Stage 2 stub replacement

**Goal**: Extract the cobra-coupled `runSwarmPipeline` (470 LOC at `cli/internal/cli/swarm.go:549`) into a non-cobra public API at `cli/internal/swarm/pipeline.go`. `eval preset` + `eval swarm-skill` route through real `swarm.RunPipeline` per side. All v0.6/v0.7/v0.8/v0.9/v0.10 swarm behavior preserved via the 8-behavior pin table from the spec.

**Success Criteria**:
- New `cli/internal/swarm/pipeline.go` exposes `RunPipeline(ctx, opts) (*Result, error)` + `PipelineOpts` struct.
- `cli/internal/cli/swarm.go::runSwarmPipeline` becomes a thin cobra → `PipelineOpts` adapter (≤50 LOC).
- `presetModeResolver.Resolve` (Stage 2 stub from v0.10) deleted; `eval preset` resolver calls `swarm.RunPipeline` per `<preset>:<mode>` pair.
- Same for `eval swarm-skill` resolver: with-skill side = `RunPipeline` with SKILL.md injected; without-skill side = `RunPipeline` without.
- All existing swarm tests stay green: consent, debate, lie-to-them, lens rotation, cache, persona-dispatch, telemetry-warning, replay.
- `swarm/pipeline_test.go` covers `RunPipeline` end-to-end via stub agents.
- Stage 2 stub stderr warning text removed from `eval_swarm_shape.go`; grep for "Stage 2 stub" returns zero matches.
- `eval preset` + `eval swarm-skill` per-side artifacts have semantic divergence (`synthesis.json::findings_count` differs between sides AND from the v0.10 stub baseline which returned identical claude output).

**Tests**:
- Pre-refactor: capture `jutsu swarm pr-review --diff-from-branch main --strict --replay <key>` output to `/tmp/pre-extract.md`.
- Post-refactor: same command, capture to `/tmp/post-extract.md`.
- Diff: must be byte-identical (replay path means no model dispatch; deterministic).
- New: `cli/internal/swarm/pipeline_test.go` exercising every PipelineOpts field through stub agents.
- New: `cli/internal/eval/runner_swarm_test.go::TestPresetSemanticDivergence` asserts `findings_count` differs between baseline + challenger sides.

**Status**: Not Started

## Stage 2: Autopilot v2 skill core + cost cap + state file + jutsu autopilot CLI

**Goal**: Replace `~/.claude/skills/autopilot/SKILL.md` (362 lines, single-reviewer + 2 gates) with v2 (multi-agent review at every artifact gate, configurable, cost-capped). Ship as `skills/core/autopilot/` for distribution via `jutsu install autopilot`. Add `jutsu autopilot` cobra subcommand group at `cli/internal/cli/autopilot.go`.

**Success Criteria**:
- `skills/core/autopilot/skill.yaml` declares `layout: rich`, agents `[claude, codex, gemini]`, deps `[]` (no other skill dependencies).
- `skills/core/autopilot/SKILL.md` covers Phase 1-6 protocol, gate config, cost-cap layered model, abort/resume modes.
- `skills/core/autopilot/references/{config-schema,cost-model,persona-defaults}.md` written.
- `skills/core/autopilot/runbooks/{abort-and-resume,drift-detection}.md` written.
- `cli/internal/cli/autopilot.go` exposes `jutsu autopilot init|status|abort|resume|run` cobra subcommand group.
- `jutsu autopilot init` writes `.kaijutsu/autopilot.yaml` from baked-in defaults. Refuses to overwrite an existing file unless `--force` is passed (non-destructive default; not strictly idempotent because re-run prints an error rather than silently no-oping).
- `jutsu autopilot status` reads `.kaijutsu/autopilot-state.md`, prints YAML-frontmatter table.
- `jutsu autopilot abort` cleans worktree + branch + state file (with confirm prompt, `--yes` bypass).
- `jutsu autopilot resume` reads `.kaijutsu/autopilot-state.md` and continues from the last completed phase (matches existing v1 resume protocol behavior).
- `jutsu autopilot run "<intent>"` non-interactive entry point; with `KAIJUTSU_AUTOPILOT_TEST_MODE=1`, writes `.kaijutsu/autopilot-pr.json` instead of `gh pr create`.
- **Cost ceiling enforcement** lives in compiled Go code at `cli/internal/cli/autopilot.go` as `const MaxAutopilotCostUSD = 100.0`. Skill markdown DOCUMENTS the constant but does not contain it (no parse-back from markdown into the binary). CLI flag `--max-cost N` raises the soft cap up to (but never above) `MaxAutopilotCostUSD`. Env var `KAIJUTSU_AUTOPILOT_HARD_CAP_OVERRIDE=N` raises the ceiling per-shell — read once at autopilot start.
- **Old autopilot replacement**: when `jutsu install autopilot` runs and `~/.claude/skills/autopilot/SKILL.md` already exists, the install pipeline backs it up to `~/.claude/skills/autopilot/.archived/SKILL.md` BEFORE overwriting with v2. No v1 detection heuristic — file presence alone triggers the archive. CHANGELOG breaking-change section calls out the behavior + advises users with local edits to back up first.
- Default config (no `.kaijutsu/autopilot.yaml`) exercises Phase 1-6 using the spec's Decision #6 baked-in defaults (built-in personas only, no agents.yaml additions required).

**Tests**:
- `cli/internal/cli/autopilot_test.go::TestAutopilotInit_WritesDefaults` asserts yaml fields match the spec's Decision #6 baked-in defaults.
- `cli/internal/cli/autopilot_test.go::TestAutopilotInit_RefusesOverwriteWithoutForce` asserts re-run without `--force` exits non-zero with a clear error message; `--force` overwrites cleanly.
- `cli/internal/cli/autopilot_test.go::TestAutopilotStatus_ReadsState` writes a fixture state file, asserts CLI output.
- `cli/internal/cli/autopilot_test.go::TestAutopilotAbort_CleansArtifacts` writes fake worktree + branch + state, asserts cleanup with `--yes`.
- `cli/internal/cli/autopilot_test.go::TestAutopilotResume_ResumesFromLastPhase` writes a state file with `phase: plan` set, asserts `jutsu autopilot resume` picks up at the next phase (`build`).
- `cli/internal/cli/autopilot_test.go::TestAutopilotRun_TestModeWritesPRJSON` runs autopilot in `KAIJUTSU_AUTOPILOT_TEST_MODE=1`, asserts `.kaijutsu/autopilot-pr.json` exists with title/body/labels.
- `cli/internal/cli/autopilot_test.go::TestAutopilotCostCap_HardCeilingHonored` writes yaml with `max_total_usd: 10000`, asserts run aborts at $100 without env-var override.
- `cli/internal/cli/autopilot_test.go::TestAutopilotCostCap_EnvOverrideRaisesCeiling` sets `KAIJUTSU_AUTOPILOT_HARD_CAP_OVERRIDE=200`, asserts run proceeds past $100.
- `cli/internal/cli/autopilot_test.go::TestAutopilotCostCap_MaxCostFlagRaisesSoftCap` invokes `jutsu autopilot run "test" --max-cost 50`, asserts soft cap honored AND the flag cannot exceed `MaxAutopilotCostUSD` (passing `--max-cost 200` errors).
- `cli/internal/install/install_test.go::TestInstallAutopilot_ArchivesPreExistingSkillMd` writes a fake pre-existing SKILL.md at `~/.claude/skills/autopilot/`, runs `install.Install`, asserts `.archived/SKILL.md` contains the original bytes + the new SKILL.md is the v2 content.
- Skill-side: `jutsu lint skills/core/autopilot/` passes (no schema errors).

**Status**: Not Started

## Stage 3: Reverse-drift gate + new built-in personas + housekeeping

**Goal**: Add 3 new built-in personas (`claim-auditor-claude`, `cross-file-gemini`, `perf-purist-codex`); wire reverse-drift gate behavior; CHANGELOG / ROADMAP / README updates; marker bump.

**Success Criteria**:
- `cli/internal/agents/builtin_personas.go` updated: 3 new personas added with documented system_prompts (per spec Stage 3 block).
- `cli/internal/agents/resolve_test.go` covers the 3 new built-in personas (registration + system_prompt presence).
- `jutsu agent list --personas` shows 7 built-in personas (4 existing + 3 new).
- `jutsu swarm pr-review --personas claim-auditor-claude` works without agents.yaml entries.
- Reverse-drift gate wired in autopilot Phase 5: `swarm.RunPipeline` invocation with `reverse` preset, `--spec <state.approved_spec_path>` + `--diff origin/main...HEAD`. The `state.approved_spec_path` field is the YAML key in `.kaijutsu/autopilot-state.md` (defined in spec §"State file"); set during Phase 2 SPEC right after the user approves Gate 1.
- Drift summary appended to PR description as `<details>` block; PR labeled `autopilot-drift` if findings present.
- Test mode: `KAIJUTSU_AUTOPILOT_TEST_MODE=1` writes drift findings to `.kaijutsu/autopilot-pr.json::drift_findings`.
- `init_agents_fragment` marker `0.10.1` → `0.11.0`; existing `init_agents_fragment_test.go` updated to assert `version=0.11.0`.
- `CHANGELOG.md` `[0.11.0]` entry covers all 3 stages, breaking change for old autopilot users, persona pack additions, cost ceiling design.
- `ROADMAP.md` flips Stage 1 carryover to [x]; adds autopilot v2 row.
- `README.md` v0.11 section: highlights autopilot v2, persona pack, cost cap.
- `docs/autopilot.md` written: full user-facing skill documentation (install, configure, common workflows, threat model).

**Tests**:
- `cli/internal/agents/resolve_test.go::TestBuiltinPersonas_NewV011` asserts the 3 new personas are registered with non-empty system_prompts.
- `cli/internal/cli/init_agents_fragment_test.go` already covers `version=0.10.1`; update to `0.11.0`.
- `cli/internal/cli/autopilot_test.go::TestAutopilotPhase5_ReverseDriftFiresWhenSpecDrifts` simulates drift in test mode, asserts `autopilot-pr.json::labels` contains `autopilot-drift`.
- `cli/internal/cli/autopilot_test.go::TestAutopilotPhase5_NoDriftLabelWhenSpecMatches` asserts the negative case.

**Status**: Not Started

---

## End-state verification (after all 3 stages)

```bash
# Build + smoke
brew upgrade momentmaker/tap/jutsu
jutsu --version                                       # 0.11.0
jutsu agent list --personas | grep -c "^"             # >= 7

# Stub replacement (Stage 1)
jutsu eval preset --evals skills/core/dream/evals/evals.json    # no Stage 2 stub stderr
# inspect .kaijutsu/eval-runs/iteration-1/*/synthesis.json — findings_count differs per side

# Autopilot CI-friendly (Stage 2)
jutsu autopilot init                                  # writes .kaijutsu/autopilot.yaml
jutsu autopilot status                                # "no autopilot run in progress"
KAIJUTSU_AUTOPILOT_TEST_MODE=1 jutsu autopilot run "add a test endpoint"
ls .kaijutsu/autopilot-pr.json                        # planned PR JSON exists
jq -r .labels[] .kaijutsu/autopilot-pr.json           # may include autopilot-drift if simulated

# Cost ceiling (Stage 2)
echo 'cost: { max_total_usd: 10000 }' > .kaijutsu/autopilot.yaml
KAIJUTSU_AUTOPILOT_TEST_MODE=1 jutsu autopilot run "test"  # capped at $100
KAIJUTSU_AUTOPILOT_HARD_CAP_OVERRIDE=200 KAIJUTSU_AUTOPILOT_TEST_MODE=1 jutsu autopilot run "test"  # works

# Marker (Stage 3)
jutsu init && grep version=0.11.0 AGENTS.md           # present

# CI: Go test suite + cobra wiring + JSON parse
cd cli && go test ./... -count=1                      # all green
```

---

## Squash + tag

After Stage 3 + final swarm pr-review pass:

```bash
git checkout main
git merge --no-ff v0.11.0-autopilot -m "Merge v0.11.0-autopilot: ..."
git tag -a v0.11.0 -m "v0.11.0 — autopilot v2 + RunPipeline + persona pack"
git push origin main
git push origin v0.11.0
brew upgrade momentmaker/tap/jutsu
```

---

## Risks (from spec, recap)

| Risk | Mitigation |
|---|---|
| `runSwarmPipeline` extraction regresses v0.6+ swarm behavior | Pre-refactor `--replay` capture + post-refactor diff (deterministic, byte-identical) |
| Autopilot loop blows cost cap | Hard ceiling = `const MaxAutopilotCostUSD = 100.0` in `cli/internal/cli/autopilot.go`; env-var override per-shell |
| Multi-agent groupthink | At least 1 HTTP-backed persona for training-corpus diversity; lens-blind-spot warning |
| Prompt injection through pipeline | Sanitize artifacts between stages; hard cost cap at runner |
| Stage 1 extraction balloons | Stage 1 is **load-bearing for Stages 2+3** (autopilot Phase 5 calls `swarm.RunPipeline` directly). Cannot ship 2+3 without 1. If extraction proves too large for v0.11.0, defer **all of v0.11.0** to v0.11.x and ship persona pack additions only as a smaller v0.11.0a patch. |

---

## Stage decomposition note

Each stage = doc-review on the stage's design decisions → implement → polish loop → swarm pr-review → squash. v0.11.0 = single merged branch, single tag.
