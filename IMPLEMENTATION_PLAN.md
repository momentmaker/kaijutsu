# IMPLEMENTATION_PLAN.md — v0.10.1

Spec: `docs/specs/2026-05-07-v0.10.1-polish.md`
Branch: `v0.10.1-polish`

> **Update during implementation**: Stage 1 (`runSwarmPipeline`
> extraction + stub replacement) was **deferred to v0.11.0**. The
> measured 470 LOC of cobra-coupled logic is too large for a
> polish patch; v0.10.1 ships Stages 2 + 3 only. See CHANGELOG
> `[0.10.1]` notes for the deferral rationale.

## Stage 1: Stage 2 stub replacement + CI doc pivot — DEFERRED TO v0.11.0

**Goal**: `eval preset` + `eval swarm-skill` invoke real `swarm.RunPipeline`. Stub stderr warning gone. CI workflow comment pivots from "deferred" to "intentional + permanent".

**Success Criteria**:
- New `cli/internal/swarm/pipeline.go` exposes `RunPipeline(ctx, opts) (*Result, error)`.
- `cli/internal/cli/swarm.go::runSwarmPipeline` becomes a thin cobra → `PipelineOpts` adapter.
- `presetModeResolver.Resolve` calls `swarm.RunPipeline` per `<preset>:<mode>`.
- `presetModeResolver` no longer emits one-shot stderr stub warning.
- `eval swarm-skill` resolver also routes through `swarm.RunPipeline` (with-skill side loads the SKILL.md, without-skill side does not).
- `eval-skills.yml` header comment reframes harness-gate as intentional design, not deferral.
- All existing swarm tests still green (no v0.6/v0.7/v0.8/v0.9 behavior regression).
- `eval preset` and `eval swarm-skill` per-side `output.txt` artifacts are non-byte-identical when baseline ≠ challenger configuration (regression test).

**Tests**:
- `swarm/pipeline_test.go` — exercises `RunPipeline` end-to-end via stub agents.
- `cli/eval_swarm_shape_test.go` — assert `presetModeResolver.Resolve` invokes pipeline (mock).
- Existing `swarm/*_test.go` should stay green; nothing about pre-existing behavior changes.

**Status**: Deferred to v0.11.0 (extraction scope exceeds polish-patch budget)

## Stage 2: `--dry-run` sweep

**Goal**: `--dry-run` flag on `install` / `upgrade` / `remove` / `publish`. Prints planned mutations + exits 0. Zero filesystem / lockfile / network writes.

**Success Criteria**:
- Each of the 4 commands has a `--dry-run` cobra flag.
- With `--dry-run`: skill directory + lockfile + manifest unchanged after invocation.
- Without `--dry-run`: behavior byte-identical to v0.10.0.
- Markdown output for TTY; JSON output when piped (auto-flip preserved).
- `publish --dry-run` does not call `gh pr create`.

**Tests**:
- `cli/internal/cli/install_test.go` — dry-run leaves `.claude/skills/`, `.agents/skills/`, lockfile untouched.
- `cli/internal/cli/upgrade_test.go` — dry-run prints diff but doesn't write.
- `cli/internal/cli/remove_test.go` — dry-run leaves skill in place.
- `cli/internal/cli/publish_test.go` — dry-run mocks `gh` and asserts no invocation.

**Status**: Complete

## Stage 3: Error enumeration sweep + housekeeping

**Goal**: Every error rejecting an enum value names the valid set. Plus CHANGELOG / ROADMAP / README / marker bump.

**Success Criteria**:
- Audit each of the 8 sites listed in the spec's "Touch list" table; fix any error path that rejects a value without naming the valid set.
- At least 3 regression tests pinning specific error-message substrings: `TestErrorEnum_PresetName`, `TestErrorEnum_PersonaName`, `TestErrorEnum_LicenseSPDX`.
- Each regression test asserts (a) error fires when given an invalid value AND (b) error message contains the full valid set as a substring.
- Stage 3 PR body lists the 8 sites + per-site status (fixed / already-good / N/A).
- `init_agents_fragment` marker version bumped to `0.10.1`.
- `agentsFragmentMarkerStart` constant updated; existing `init_agents_fragment_test.go::TestWriteAgentsFragment_UpgradesAcrossVersions` checks `version=0.10.1`.
- `CHANGELOG.md` `[0.10.1]` section.
- `ROADMAP.md` flips `eval preset` + `eval swarm-skill` from [stub] to [x].
- `README.md` v0.10 section adds short v0.10.1 sub-paragraph.

**Tests**:
- `cli/internal/cli/swarm_test.go` (or wherever appropriate) — assert "unknown preset" error contains "pr-review" and "dream".
- `cli/internal/cli/init_agents_fragment_test.go` — `version=0.10.1` assertion already covers marker bump.

**Status**: Complete

---

## End-state verification (after all 3 stages)

```bash
# Stub replacement
jutsu eval preset --evals skills/core/dream/evals/evals.json   # no stub warning
# (real per-mode pipeline outputs, meaningful diff)

# Dry-run
jutsu install pr-review --dry-run; ls .claude/skills/pr-review/ # ENOENT
jutsu upgrade --dry-run                                          # diff printed, no writes

# Error enumeration
jutsu swarm not-a-preset                                         # names valid presets

# Marker
jutsu init && grep version=0.10.1 AGENTS.md                      # present

# CI
gh run list --workflow eval-skills.yml --limit 1                 # green on push
```

---

## Squash + tag

After Stage 3 + final swarm pr-review pass + polish gate:

```bash
git checkout main
git merge --no-ff v0.10.1-polish -m "Merge v0.10.1-polish: ..."
git tag -a v0.10.1 -m "v0.10.1 — ..."
git push origin main
git push origin v0.10.1
brew upgrade momentmaker/tap/jutsu
```
