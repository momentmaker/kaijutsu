# IMPLEMENTATION_PLAN.md — v0.9.0 Feedback-loop hardening

**Spec:** `docs/specs/2026-05-07-v0.9.0-feedback-loops.md`
**Branch:** `v0.9-stage-1-lens-schema` (rotate per stage)
**Release target:** v0.9.0
**Strategy:** 5 sequential stages, each tagged + brewed before next starts. `/polish` after every stage. Every stage compiles + tests-green before commit.

---

## Stage 1 — Lens schema migration + recorder/weighter lens column

**Goal:** `0002_lens.sql` ships, `lens` + `position` columns populated by recorder, weighter optionally keyed by lens with tuple fallback. `jutsu finding seed --dev` lands as a dev-only test affordance. Existing v0.7/v0.8 DBs migrate cleanly.

**Success criteria:**
- `jutsu finding list` against pre-v0.9 DB triggers migration, then renders identically.
- Fresh-install DB has both columns + the `idx_findings_lens_lookup` index.
- Recorder writes `lens` for dream rows (extracted from summary prefix), `NULL` for non-dream rows. `position` is 0..N-1 monotonic per run.
- `weighter.WeightFor(meta)` with `meta.Lens=""` returns identical weight to v0.8.3 (regression-safe).
- `weighter.WeightFor(meta)` 3-tier resolution: (a) lens-specific window above bootstrap → lens-specific weight; (b) lens-specific below bootstrap AND tuple-without-lens above bootstrap → tuple weight; (c) BOTH below cold threshold → cold default. Tier (b) is the load-bearing fallback path that prevents lens fragmentation from starving the weighter.
- `jutsu finding seed --provider … --persona … --lens … --accepts N --dismisses M` works when EITHER `--dev` flag is passed OR `KAIJUTSU_FINDING_SEED=1` env-var is set; either alone enables the subcommand. Production CLI without either rejects with "unknown command \"seed\"" (cobra's standard unknown-subcommand error).

**Files to create:**
- `cli/internal/findings/migrations/0002_lens.sql` — verbatim from spec §1.
- `cli/internal/findings/migrations/migrations_test.go` — `TestMigration0002_BackfillsExistingDreamRows`, `TestMigration0002_HandlesEmptyDB`, `TestMigration0002_Idempotent`.
- `cli/internal/cli/finding_seed.go` — dev-only seed subcommand. Available when EITHER `--dev` flag is passed at the `finding` group level OR `KAIJUTSU_FINDING_SEED=1` env-var is set. Programmatic INSERT with `seed-<unix-ns>` run_id prefix. Help text: "DEV ONLY — seed synthetic actioned findings for weighter calibration testing".
- `cli/internal/cli/finding_seed_test.go` — `TestFindingSeed_DevOnlyFlagGated` (covers flag path), `TestFindingSeed_DevOnlyEnvVarGated` (covers env-var path), `TestFindingSeed_RejectsWhenNeitherSet` (covers production rejection), `TestFindingSeed_InsertsActionedRows`.

**Files to modify:**
- `cli/internal/findings/recorder.go` — extract lens via NEW shared helper `LensFromSummary(summary string) string` that owns the regex (private to package, exported as a stable API for the recorder + any future caller). Existing `dreamSummaryPattern` regex moves into this helper; `ValidDreamFinding` reuses it. Populate `lens` column from helper output; track + populate `position` (loop counter). `RecordRun` signature unchanged (still `(int, int, error)` — added columns are internal). Test pin: `TestLensFromSummary_StableShape` asserts the helper accepts all 8 known lens prefixes and rejects malformed input — guards against accidental regex drift.
- `cli/internal/findings/recorder_test.go` — `TestRecordRun_WritesLensAndPosition`, `TestRecordRun_NonDreamHasNullLens`.
- `cli/internal/findings/weighter.go` — `WeightMeta` struct gains `Lens string` field; `WeightFor` queries lens-specific tuple first, falls back to lens-NULL tuple when below bootstrap threshold; falls back to cold default only when both are below cold threshold.
- `cli/internal/findings/weighter_test.go` — `TestWeighter_LensSpecificMaturePath`, `TestWeighter_LensFallsBackToTupleWhenSparse`, `TestWeighter_LensColdDefaultWhenBothEmpty`.
- `cli/internal/cli/finding.go` — register `seed` subcommand under group, gated by `--dev` / env-var detection.

**Out of scope for Stage 1:**
- Synthesizer adaptive lens-weighting (Stage 2).
- Pruning old rows (v0.9.x).
- Chunked-backfill for large DBs (v0.9.x).

**Polish gate:** zero CRITICAL or ISSUE-severity findings across 4 polish passes; MINOR/INFO findings can defer to a follow-up if explicitly listed in the stage commit message under "Deferred to v0.9.x:". The polish loop's `Pass N: Clean` clause stops on a pass with zero issues at any severity, but the gate to advance to the next stage is the issue/critical threshold.

---

## Stage 2 — Synthesizer adaptive lens-weighting + lens-rotation rule

**Goal:** Dream synthesizer reads per-lens weights, reorders + tier-gates output. LEAD lens rotates on repeat-within-7d. Standalone `/dream` and swarm dream both honor rotation.

**Success criteria:**
- Two synthesis runs with different pre-seeded weighter states produce different lens orderings. Determinism rule: lenses sort by weight DESC; ties broken by canonical-cycle order (`honest, fit, gaps, wild, adversary, inverse, status-quo, time`) — NOT alphabetical. So two lenses both at weight=0.5 always appear in canonical order, regardless of insertion sequence.
- `load_bearing=true` finding from a `weight<0.4` lens renders in the "consider" section, not the load-bearing block.
- `load_bearing=true` finding from a `weight≥0.7` lens renders with "(high-confidence)" badge in the load-bearing block.
- `weighter==nil` path renders identically to v0.8.3 dream output (no reordering, no tier gating).
- Repeat dream within 7d shifts LEAD lens through canonical cycle: `honest → fit → gaps → wild → adversary → inverse → status-quo → time → honest`. `RotateLensOrder(prev []string)` behavior: read prev[0] (the LEAD); look up its index in canonical cycle; return new ordering with cycle[(idx+1) % 8] as new LEAD followed by remaining canonical entries. If prev[0] is empty OR not in the canonical set (data corruption or pre-v0.9 graveyard file), default to `honest` as new LEAD with full canonical ordering.
- `KAIJUTSU_DREAM_ADAPTIVE_LENS=off` env-var disables tier gating + reordering (output identical to v0.8.3).

**Files to modify:**
- `cli/internal/swarm/synth.go` — `Synthesize()` for `preset.Name=="dream"` fetches per-lens weights via injected weighter; reorders cells; gates tier per the 6-cell rule. New helper `applyLensTierGating(findings []Finding, weights map[string]float64) []Finding`. Killswitch checked at top: if `os.Getenv("KAIJUTSU_DREAM_ADAPTIVE_LENS") == "off"`, skip the whole reorder+gate block.
- `cli/internal/swarm/synth_test.go` — `TestSynthesizeDream_ReordersByLensWeight`, `TestSynthesizeDream_LoadBearingTierGating`, `TestSynthesizeDream_NilWeighterFallsBackToV083`, `TestSynthesizeDream_KillswitchDisablesAdaptive`.
- `cli/internal/cli/dream.go` — extend `WriteDreamSession` header (already records `lens_order`); add lens-rotation logic when `--mode full` AND `FindRecentDreamForTopic` returns a hit. Helper: `RotateLensOrder(prev []string) []string` returns next-canonical-cycle ordering.
- `cli/internal/cli/dream_test.go` — `TestRotateLensOrder_CanonicalCycle`, `TestRotateLensOrder_HandlesPartialPriorList`.
- `cli/internal/cli/swarm.go` — wire weighter into synth call for dream preset; pre-rotation step before `BuildDreamPrompt`.
- `skills/core/dream/SKILL.md` — note rotation behavior for standalone `/dream` (already partially in v0.8.0; tighten the wording to match the canonical-cycle rule).

**Out of scope for Stage 2:**
- Pass-2 debate (Stage 3).
- Threshold tuning beyond 0.4 / 0.7 defaults.

**Polish gate:** zero issues across 4 passes; manual smoke test of repeat-within-7d rotation against a real codebase.

---

## Stage 3 — Dream Pass-2 debate template + cobra reject lift

**Goal:** `--mode full` runs end-to-end for dream; merged Pass-1⊕Pass-2 output reaches synthesis + graveyard; non-TTY behavior is safe.

**Success criteria:**
- `jutsu swarm dream --topic "X" --mode full --yes` completes; output contains at least one revision tag from `{[new], [disputes], [revised], [agreed]}`.
- `[agreed]` reachable when ≥2 peers per lens (Stage 3 fixtures pin this).
- `[lens:<name>]` prefix preserved on every Pass-2 finding (validator continues to accept rows).
- Cost prompt fires interactively without `--yes`; non-TTY without `--yes` hard-fails with the spec's exact error string.
- Graveyard file written from Pass-2 result includes `mode: full` in header (already wired in v0.8.3 — Stage 3 just makes it actually fire).

**Files to modify:**
- `cli/internal/swarm/preset.go` — replace `dreamDebatePlaceholder` with the real template from spec §5; update `dreamPreset.Debate` to point at it.
- `cli/internal/swarm/dream_test.go` — `TestDreamDebateTemplate_PreservesLensPrefix`, `TestDreamDebateTemplate_RevisionTagsRecognized`, `TestDreamDebate_AgreedRequiresTwoPeers`.
- `cli/internal/cli/swarm.go` — remove the cobra-layer `--mode full` reject for dream; add cost-prompt + non-TTY hard-fail logic. Helper: `confirmDreamFullModeCost(stdin, stdout, estimate float64, yes bool) error` — TTY+!yes → prompt; !TTY+!yes → return error; yes → no-op. Estimate computed by the existing `swarm/cost.go` helper `EstimateBudget(jobs []Job)` with the Pass-2 multiplier applied (`× 2` for the debate fan-out cost over Pass-1's job set; matches how `--full` cost is computed for non-dream presets).
- `cli/internal/cli/swarm_test.go` — `TestSwarmDream_ModeFullEndToEnd` (uses fake-providers fixture pinned to 2 peers per lens), `TestDreamModeFullCostPrompt_TTY`, `TestDreamModeFullCostPrompt_NonTTYRejects`.

**Out of scope for Stage 3:**
- `--budget <dollars>` flag (deferred to v0.9.x per spec open question 4).
- Pricing-data refresh (deferred per spec).

**Polish gate:** zero issues; integration test against fake-providers passes deterministically.

---

## Stage 4 — Reverse swarm preset

**Goal:** `jutsu swarm reverse --spec X --diff Y` runs end-to-end; produces drift table; `--post-comment` posts under `<!-- kaijutsu-reverse:run-id=… sha=… -->` marker; oversize truncation deterministic.

**Success criteria:**
- Run against a fixture spec/diff pair where 1 ADDED, 1 OMITTED, 1 CHANGED → all 3 surface in the right category.
- `--lie-to-them=on` admits sycophancy-pattern findings (verifiable by feeding a flat-validation phrase fixture).
- Confidence threshold defaults to 0.30; `--confidence-threshold` overrides.
- Oversize fixture (rendered output >60KB) triggers truncation; ordering deterministic across runs given identical input; footer line present; full report on stderr.
- Draft-PR detection (via `gh pr view --json isDraft`) emits softer header.
- Distinct marker prefix from pr-review (`kaijutsu-reverse:` vs `kaijutsu-pr-review:`).

**Files to create:**
- `cli/internal/swarm/reverse.go` — `reversePreset` struct (extends `Preset`); per-agent prompt skeleton from spec §3; `reverseSynthesizer` reusing pr-review's cluster-then-table pipeline; package-level constant `MaxGitHubCommentBytes = 60_000` (4KB safety margin under GitHub's 65,536 limit); deterministic oversize truncation `truncateForGitHub(findings []Finding, capBytes int) ([]Finding, int)` with `(severity, file_path, line_range_start)` ordering. Production callers pass `MaxGitHubCommentBytes`; tests pass smaller caps to trigger truncation deterministically without giant fixtures.
- `cli/internal/swarm/reverse_test.go` — `TestReversePreset_DispatchesPerAgent`, `TestReverseSynth_ClusterByCategory`, `TestReverseSynth_OversizeTruncationDeterministic`, `TestReverseSynth_DraftPRSofterHeader`, `TestReverseSynth_LieToThemFlagToggle`.
- `cli/internal/swarm/comment.go` — extend with `MarkerReverse(runID, sha string) string` returning `<!-- kaijutsu-reverse:run-id=… sha=… -->`.

**Files to modify:**
- `cli/internal/swarm/preset.go` — register `reversePreset` in the preset map.
- `cli/internal/cli/swarm.go` — register `reverse` subcommand; flags `--spec <path>`, `--diff <range>` (default `origin/main...HEAD`), `--confidence-threshold <float>` (default 0.30), `--lie-to-them=<on|off>` (default off), `--post-comment`, `--reverse-spec <path>` flag on `pr-review` for the combined trigger.
- `cli/internal/cli/swarm_test.go` — `TestSwarmReverse_EndToEnd`, `TestPrReviewReverseSpecCombined`.

**Out of scope for Stage 4:**
- Cross-preset finding de-dup (deferred to v1.0 per spec).
- Auto-discovery of spec by branch name (deferred per spec).
- Multi-spec drift detection (deferred per spec).

**Polish gate:** zero issues; smoke test with a real PR + spec from this repo.

---

## Stage 5 — Per-skill provider routing + PR-comment auto-detection

**Goal:** `routing:` parsed from skill.yaml; 3-phase dispatch implemented; `KAIJUTSU_DISABLE_AGENTS` honored. `<!-- finding:<run_id>:<position> -->` marker emitted; sync-pr ingests reactions + replies; weighter receives accept/dismiss writes.

**Success criteria:**
- Skill with `routing.per-persona.honest: [claude]` against an environment where `KAIJUTSU_DISABLE_AGENTS=codex,gemini` → only claude dispatches for honest persona.
- `routing.default` empty list AND omitted block produce identical fallback (registry default).
- `--strict-routing` with unavailable preferred provider → exit 1 + spec error string.
- Phase C invariant: zero-survivor matrix → exit 1 with the spec's error string, regardless of `--strict-routing`.
- `--post-review` posts per-finding line-anchored review comments, each carrying `<!-- finding:<run_id>:<position> -->` on first line.
- Findings without a clean file anchor degrade to inline-marker in the header single-comment (graceful fallback).
- **Marker grammar scope:** sync-pr parses ONLY pr-review markers (`<!-- finding:<id> -->` and `<!-- kaijutsu-pr-review:run-id=… -->`). Reverse-preset markers (`<!-- kaijutsu-reverse:run-id=… -->`) are intentionally NOT parsed — sync-pr is pr-review-only in v0.9 per spec scope. Reverse comments coexist on the PR; sync-pr's parser ignores them by marker-prefix filter.
- `jutsu finding sync-pr <pr>` (dry-run default) prints the `would mark` diff; `--apply` writes; rerun is idempotent.
- Reply-keyword regex matches all 5 grammar variants from spec §2; doesn't match malformed (`Accept` without colon, `accept: X:foo` non-numeric position).
- Cross-channel timestamp resolution: reaction at T+1 + reply-keyword at T+2 → reply wins (later); reverse → reaction wins. **Tie-break (T==T):** reply-keyword wins. Justification: reply is the more deliberate channel (typing a comment vs one-click reaction); deterministic tiebreak prevents nondeterministic test failures on low-resolution clocks. Note: GitHub API returns millisecond-resolution `created_at` timestamps, making true ties vanishingly rare in practice — the tiebreak is for test fixtures with hand-crafted identical timestamps and for any future GitHub API changes.
- Non-threaded comment without run-id reference → ignored (priority-3 fallthrough).

**Files to create:**
- `cli/internal/swarm/routing.go` — `ResolvePersonaProvider(skill *Skill, persona string, available []string, strict bool) (provider string, err error)` implementing Phases A+B+C.
- `cli/internal/swarm/routing_test.go` — full table-driven test of the 3-phase resolution: per-persona hit, default hit, registry fallback, Phase B unavailable + strict, Phase B unavailable + non-strict drop, Phase C zero-survivor hard-fail.
- `cli/internal/swarm/sync_pr.go` — `SyncPR(ctx, pr int, applyMode bool) (*SyncReport, error)`; reply-keyword grammar regex; reaction parse via two `gh api` calls; cross-channel timestamp merge.
- `cli/internal/swarm/sync_pr_test.go` — `TestSyncPR_ParsesReactionMarkers` (with recorded fixtures under `testdata/sync-pr/`), `TestSyncPR_ParsesReplyKeywords` (table-driven grammar cases), `TestSyncPR_IgnoresUnscopedComments`, `TestSyncPR_CrossChannelTimestampOrdering`, `TestSyncPR_DryRunDefault`, `TestSyncPR_IdempotentOnRerun`.
- `cli/internal/swarm/sync_pr_review_render.go` — `--post-review` mode: emits per-finding `gh api ... pulls/{pr}/comments` calls; carries marker on first line; falls back to inline-marker for finding-without-file-anchor.
- `cli/internal/swarm/sync_pr_review_render_test.go` — `TestPostReview_EmitsMarkerOnEachComment`, `TestPostReview_FallbackForFindingWithoutFile`.
- `testdata/sync-pr/` fixtures — captured `gh api` responses (pre-seeded). One PR with both reaction + reply channels; one PR with only reactions; one PR with only replies; one PR with multiple kaijutsu marker comments (re-run scenario).
- `testdata/reverse/` fixtures — captured `gh pr view --json isDraft` responses (one draft PR, one non-draft) for the Stage 4 `TestReverseSynth_DraftPRSofterHeader` test path.

**Files to modify:**
- `cli/internal/skill/skill.go` — add `Routing *Routing` field; `Routing` struct has `PerPersona map[string][]string` + `Default []string`.
- `schemas/skill.schema.json` — add `routing` block per spec §4.
- `cli/internal/agents/detect.go` — honor `KAIJUTSU_DISABLE_AGENTS=<comma-list>` at detection time. Drops listed driver names from the available set after CLI presence detection.
- `cli/internal/agents/detect_test.go` — `TestDetect_HonorsDisableAgentsEnvVar`.
- `cli/internal/cli/swarm.go` — wire `ResolvePersonaProvider` into `assemblePersonaJobs`; emit Phase B/C error strings; register `sync-pr` subcommand under `finding`; add `--auto-sync` flag on `pr-review` + `--post-review` flag.
- `cli/internal/cli/finding.go` — register `sync-pr` subcommand.
- `cli/internal/cli/swarm_test.go` — `TestSwarmDispatch_StrictRoutingFailsOnUnavailable`, `TestSwarmDispatch_PhaseCMinDispatchInvariant`.
- `skills/core/pr-review/skill.yaml` — add `routing:` block declaring per-persona preferences (claude for honest persona; rotation for adversary; etc — ship author-declared defaults from the spec §4 example).
- `skills/core/dream/skill.yaml` — add `routing:` block.

**Out of scope for Stage 5:**
- Auto-route based on observed precision (v1.0 candidate).
- Submitting actual GitHub Reviews via the Reviews API (v1.0 candidate).
- Cross-preset action ingest (v1.0 candidate).

**Polish gate:** zero issues; smoke test against a live PR with both reaction + reply channels.

---

## Cross-stage release housekeeping (after Stage 5)

- Bump `cli/internal/cli/init_agents_fragment.go` marker version: `0.8.2` → `0.9.0`. Verify `TestWriteAgentsFragment_UpgradesAcrossVersions` still passes.
- Update `CHANGELOG.md` `[0.9.0]` entry covering all 5 items.
- Update `ROADMAP.md` — mark v0.9 backlog items shipped.
- Update `README.md` v0.9 section if any user-facing CLI surface changed (sync-pr, reverse, --post-review are user-visible).
- Run `jutsu swarm pr-review --strict --full --post-comment` against the v0.9 branch as final adversarial pass (the v0.8.3 pattern: catch real bugs before tag).
- Tag `v0.9.0`, brew upgrade locally, verify `jutsu --version` reports `0.9.0`.

---

## Test discipline

Every stage:
1. Tests written first (red) per CLAUDE.md TDD philosophy.
2. Implementation minimal-to-pass (green).
3. `go test ./cli/...` green before commit.
4. `/polish` 4-pass autonomous review.
5. Commit + push + brew upgrade.

Integration tests use the fake-providers fixture pattern (already established in v0.6+). Real `gh api` calls in tests are forbidden; all sync-pr tests use captured-fixture JSON.

---

## Rollback plan per stage

- Stage 1: migration is forward-only. Rollback strategy depends on local SQLite version:
  - **SQLite ≥3.35 (Mar 2021)**, includes Homebrew's bundled SQLite on macOS: `sqlite3 ~/.kaijutsu/findings.db "DELETE FROM schema_version WHERE version=2; ALTER TABLE findings DROP COLUMN lens; ALTER TABLE findings DROP COLUMN position;"`.
  - **SQLite <3.35**, recreate-table fallback: dump rows excluding lens/position via `.dump`, manually edit out the columns from the schema and the values from each INSERT, drop + recreate, reload. Documented in `CHANGELOG.md` under `[0.9.0]` known-issues if stage ships and Stage 2+ doesn't follow.
  - Recommended: brew downgrade is the simpler path; the schema is forward-compatible with v0.8.3 reads (extra columns ignored), so users on a stale CLI tolerate a v0.9-migrated DB without action.
- Stages 2-5: pure code changes. Rollback = brew downgrade. No data corruption risk.

---

## Risk + mitigation (operational)

| Risk | Mitigation |
|---|---|
| Stage 1 migration runs, Stages 2-5 stall — users on v0.9.0 with mixed-state DB | Migration is forward-compatible with v0.8.3 reads. Stage 1 ships only with all migration tests green. |
| Stage 5 sync-pr writes wrong action due to grammar bug | `--apply` is opt-in; default dry-run prints diff for human verification. |
| Stage 4 reverse preset over-fires drift findings, drowns PR comments | Confidence threshold + lie-to-them=off are tunable; oversize truncation hard-caps comment size. |
| Stage 3 cost prompt non-TTY behavior misjudged for some CI providers | Hard-fail is the safe default; users add `--yes` explicitly to their CI invocation. |
| KAIJUTSU_DISABLE_AGENTS env-var leaks into production runs | Env-var is opt-in; default empty; documented as test-only in detect.go comment. |
| Stage 2 lens-rotation interacts with Stage 2 adaptive killswitch | Rotation is independent of adaptive lens-weighting (rotation reads graveyard, weighting reads DB). Killswitch (`KAIJUTSU_DREAM_ADAPTIVE_LENS=off`) only disables tier-gating + reordering; rotation still fires. Documented in killswitch help text. Stage 2 polish includes a `TestRotation_FiresWithKillswitchOn` test to lock the behavior. |

---

## Open against spec (decisions surfaced during planning)

None new. All open questions from spec §"Open questions" remain spec-side decisions; plan inherits them as written.
