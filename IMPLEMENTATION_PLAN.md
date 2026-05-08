# IMPLEMENTATION_PLAN.md — v0.12.0

Spec: `docs/specs/2026-05-08-v0.12.0-tier-a-presets.md`
Branch: `v0.12.0-tier-a-presets`

Three new swarm presets — `test-gap`, `bug-repro`, `code-archaeology`. Each preset = one Stage. All three follow the existing reverse-preset implementation template (`cli/internal/swarm/reverse.go` + `cli/internal/cli/swarm_reverse.go`).

## Stage 1: `test-gap` preset

**Goal**: Ship `jutsu swarm test-gap --code <path> --tests <path>` — surfaces failure scenarios MISSING from existing tests.

**Severity vocab** (matches reverse preset for cross-preset consistency): `[blocker, issue, minor, info]` (defined in `cli/internal/swarm/types.go::Severity`). All 4 valid; synthesizer surfaces `blocker`+`issue` first.

**Reverse-preset template fit verification** (per doc-review #minor on template assumption): the existing reverse preset takes `--spec` (markdown) + `--diff` (code) + builds a custom prompt at command time via `BuildReversePrompt(specContent)`. test-gap follows the same pattern: takes `--code` + `--tests`, builds prompt at command time embedding both. InputKind = the kind that drives `swarm.ResolveInput` dispatch (primary input). Auxiliary inputs bake into the prompt template at the cobra layer.

**Success Criteria**:
- New `cli/internal/swarm/test_gap.go` — `var testGapPreset = Preset{...}` with InputKind = `InputFiles` (primary input is the test file content; code-under-test bakes in via prompt construction), severity vocab `[blocker, issue, minor, info]`, custom synthesizer prompt that emphasizes ranking + clustering.
- New `cli/internal/cli/swarm_test_gap.go` — cobra subcommand registering against `commonSwarmFlags`. Flags: `--code <path>` (required), `--tests <path>` (required, file or directory). Optional: `--include <glob>` (filter to subset).
- `cli/internal/swarm/registry.go::init()` registers `&testGapPreset` alongside the existing 7.
- `cli/internal/cli/swarm.go::newSwarmCmd()` adds `cmd.AddCommand(newSwarmTestGapCmd())`.
- **Input validation** (per spec Decision #8 = "cobra rejects unreadable/nonexistent files; combined input > 200 KB → error with --include hint"): unreadable / nonexistent `--code` or `--tests` paths → cobra error containing the path + `os` error; combined body > 200 KB (matches `cli/internal/swarm/diff.go::maxBytesDiff`) → cobra error with `narrow with --include or pick a smaller path` hint.
- `jutsu swarm test-gap --help` lists the flags + describes the preset.
- `jutsu swarm test-gap --estimate --code <p> --tests <p>` returns a cost projection (one row per agent in --personas mode, one row per detected provider in legacy mode — matches existing `--estimate` output across all presets).
- `jutsu swarm test-gap --code <p> --tests <p> --replay <key>` rehydrates from cache (existing replay infra works because preset registers + cache key is deterministic from input bytes).
- **Acceptance criterion for "surfaces missing scenarios"** (per doc-review #issue on untestable goal): success means the synthesis output contains a `Findings` section listing 1+ ranked clusters when the test fixture has known coverage gaps. Test fixture: a deliberately-incomplete test file paired with code that has documented edge cases not exercised. Test asserts the synthesis surfaces at least one of the known gaps. Quality of finding (does it match what a human reviewer would flag?) is empirically validated in v0.12.x A/B benchmarks; v0.12 acceptance is "produces ranked output, no errors".
- All existing swarm + cli + eval tests stay green.

**Tests**:
- `cli/internal/swarm/test_gap_test.go` — `TestTestGapPreset_Registers` (find by name in defaultRegistry), `TestTestGapPreset_DefaultPromptHasInputSlot` (assert `%s` substitution placeholder), `TestTestGapPreset_SeverityVocab` (matches reverse vocab).
- `cli/internal/cli/swarm_test_gap_test.go` — `TestSwarmTestGap_RejectsMissingCodePath`, `TestSwarmTestGap_RejectsMissingTestsPath`, `TestSwarmTestGap_RejectsOversizeInput`, `TestSwarmTestGap_HelpListsFlags`.

**Status**: Not Started

## Stage 2: `bug-repro` preset

**Goal**: Ship `jutsu swarm bug-repro "<bug description>" [--files <paths>]` — vague bug → ranked repro hypotheses + minimal-repro steps for top hypothesis.

**InputKind choice** (per doc-review #issue on InputKind mismatch): `InputPrompt`. The PRIMARY input that drives `swarm.ResolveInput` dispatch is the bug description (text). Optional `--files` content gets baked into the prompt at the cobra layer (same pattern as reverse preset's `--spec` + `--diff`). InputKind names the dispatch shape, not the totality of bytes sent to agents.

**Severity vocab**: same as test-gap — `[blocker, issue, minor, info]`.

**Success Criteria**:
- New `cli/internal/swarm/bug_repro.go` — `var bugReproPreset = Preset{...}` with InputKind = `InputPrompt`, severity vocab `[blocker, issue, minor, info]`, custom synthesizer prompt for hypothesis-clustering by category (state / race / env / input / version).
- New `cli/internal/cli/swarm_bug_repro.go` — cobra subcommand. Positional arg = bug description (required, non-empty after `swarm.CanonicalizePrompt`). Optional: `--files <paths>` (comma-separated; each file content appended to prompt with size-cap enforcement).
- Registration + AddCommand.
- **Input validation**: bug description max 8 KB after canonicalization (matches `swarm.maxBytesPrompt`); combined bug + `--files` content max 200 KB; oversize → cobra error with hint to narrow files. Empty bug description → cobra error.
- `jutsu swarm bug-repro "test bug" --estimate` returns cost projection.
- `jutsu swarm bug-repro "test bug" --replay <key>` rehydrates.
- Output structure (after synthesis): top 3 hypotheses with category + repro_steps + confidence; runners-up listed without repro_steps.
- All existing tests green.

**Tests**:
- `cli/internal/swarm/bug_repro_test.go` — preset registration + DefaultPrompt has slot + severity vocab.
- `cli/internal/cli/swarm_bug_repro_test.go` — `TestSwarmBugRepro_RequiresBugDescription`, `TestSwarmBugRepro_RejectsEmptyAfterCanonicalize`, `TestSwarmBugRepro_RejectsOversizeFiles`, `TestSwarmBugRepro_HelpDescribesShape`.

**Status**: Not Started

## Stage 3: `code-archaeology` preset + housekeeping

**Goal**: Ship `jutsu swarm code-archaeology --code <path> [--git-log <since>]` — explain WHY legacy code looks the way it does. Plus marker bump + CHANGELOG/ROADMAP/README + final swarm pr-review pass.

**Final swarm pr-review pass** (per doc-review #info clarification): after Stage 3 lands its commits but BEFORE the merge-to-main + tag, run `jutsu swarm pr-review --diff-from-branch main --strict --max-cost 5.0 --yes` against the cumulative `main..HEAD` diff. Same pattern as past releases (v0.10/v0.10.1/v0.11). Apply load-bearing findings; defensible-skip the rest. NOT a separate cobra command — just the existing pr-review preset run against the release branch.

**Severity vocab**: same as test-gap + bug-repro — `[blocker, issue, minor, info]`.

**Decision #9 (reproduced from spec for plan self-containment)**: code-archaeology's git-log fetch is best-effort. If `git log` fails (not a git repo, gh not on PATH, no commits touching path, exit non-zero), preset proceeds with EMPTY `<HISTORY>` context + a one-line stderr warning. Theories degrade gracefully (less evidence to cite). Hard-failing the whole pipeline on a missing git history is over-strict — the code itself is still analyzable.

**Success Criteria**:
- New `cli/internal/swarm/code_archaeology.go` — `var codeArchaeologyPreset = Preset{...}` with InputKind = `InputFiles`, severity vocab `[blocker, issue, minor, info]`, custom synthesizer prompt for theory-clustering with corroborated / contested provenance markers.
- New `cli/internal/cli/swarm_code_archaeology.go` — cobra subcommand. Flags: `--code <path>` (required), `--git-log <since>` (optional; accepts duration like `30d` / `6mo` / `2y` OR git-log-compatible date strings; invalid value → cobra error with examples; default = absent, falls back to `-100` commit cap). Git-log fetch is best-effort per Decision #9.
- Registration + AddCommand + input validation (200 KB cap on `--code` body; git-log capped at 50 KB INDEPENDENTLY).
- Git-log fetch implementation: shells out to `git log` with one of:
  - `git log -100 --format=fuller -- <path>` (default — last 100 commits touching path)
  - `git log --since=<since> --format=fuller -- <path>` (when `--git-log <since>` set)
  Captures stdout; truncates at 50 KB if needed (preserves earliest commits — most-recent are most likely the "why" anyway).
- Marker version bump `0.11.0` → `0.12.0` in `cli/internal/cli/init_agents_fragment.go`. `init_agents_fragment_test.go` assertion `version=0.11.0` → `version=0.12.0`. `jutsu init` smoke writes the new version into AGENTS.md.
- `CHANGELOG.md` `[0.12.0]` entry covering all 3 stages, the dream-survival framing (7 candidates → 3 survivors), and the deferral list (`dep-review`, `api-review`, `migrate`, `postmortem` — with one-line reason each: solo-agent territory / IDE-native soon / too high-stakes / harm-vector). The deferral list lives in CHANGELOG even though it's not in the active roadmap; v0.12 is the release where they were considered + rejected.
- `ROADMAP.md` flips v0.12 row `[ ]` → `[x]` for the 3 shipped presets.
- `README.md` v0.12 section: short paragraph per preset + "when to use which" guidance.
- All existing tests stay green.

**Tests**:
- `cli/internal/swarm/code_archaeology_test.go` — preset registration + DefaultPrompt slots (input + history).
- `cli/internal/cli/swarm_code_archaeology_test.go` — `TestSwarmCodeArchaeology_RejectsMissingCodePath`, `TestSwarmCodeArchaeology_GitLogFailureGracefulDegradation` (mock `git` to return error; assert preset proceeds with empty history + stderr warning), `TestSwarmCodeArchaeology_HelpListsFlags`.
- `cli/internal/cli/init_agents_fragment_test.go` — already covers `version=` assertion; update value.

**Status**: Not Started

---

## End-state verification (after all 3 stages)

```bash
brew upgrade momentmaker/tap/jutsu
jutsu --version                                              # 0.12.0
jutsu swarm --help | grep -E "test-gap|bug-repro|code-archaeology"  # all 3 listed

# Estimate paths (no model dispatch — proves cobra wiring)
jutsu swarm test-gap --estimate --code skills/core/dream/SKILL.md --tests skills/core/dream/
jutsu swarm bug-repro --estimate "intermittent login failure" --files cli/internal/cli/agent_subcommands.go
jutsu swarm code-archaeology --estimate --code cli/internal/swarm/preset.go

# Marker
jutsu init && grep version=0.12.0 AGENTS.md

# Test suite
cd cli && go test ./... -count=1                              # every package green
```

CI:
- `lint-skills.yml` (with v0.11.x auto-regen) handles any catalog drift if a `skills/core/` change is implied (none in v0.12).
- `release-jutsu.yml` produces v0.12.0 tarballs + brew tap update.

---

## Squash + tag

After Stage 3 + final swarm pr-review pass:

```bash
git checkout main
git merge --no-ff v0.12.0-tier-a-presets -m "Merge v0.12.0-tier-a-presets: ..."
git tag -a v0.12.0 -m "v0.12.0 — Tier A swarm presets (test-gap, bug-repro, code-archaeology)"
git push origin main
git push origin v0.12.0
brew upgrade momentmaker/tap/jutsu
```

---

## Risks (recap from spec + plan)

| Risk | Mitigation |
|---|---|
| Shared prompt → 3 copies of same answer (no swarm benefit) | Cross-MODEL diversity is the LOAD-BEARING premise of legacy multi-agent dispatch (shipped since v0.6); empirical disagreement counts in `~/.kaijutsu/findings.db` are the existing evidence. Dream pass on v0.13 plan flagged this premise as possibly self-confirming under RLHF convergence; v0.12.x will run controlled A/B (single-agent vs 3-agent shared-prompt) on benchmark fixtures to validate. If A/B shows single-agent matches multi-agent on test-gap output, escalate to per-agent prompt specialization in v0.13 OR downgrade test-gap from preset to skill. |
| Synthesizer ranking is brittle when reviewers cluster differently | Custom synthesizer prompts explicitly handle clustering; test fixtures pin behavior on contrived multi-reviewer outputs. |
| `code-archaeology` git-log fetch can be huge for old files | 50 KB log cap independent of 200 KB input cap. `--git-log <since>` lets user narrow. |
| `bug-repro` repro_steps may be hallucinated | Synthesizer marks corroborated vs single-source; only top 3 ranked hypotheses get full steps; runners-up listed without steps. |
| Stage 3 bundles housekeeping with preset work | Same pattern as past releases (v0.10, v0.10.1, v0.11). Final-stage rollup. |
| Cobra subcommand sprawl | 3 added; total swarm preset count goes 7 → 10. v0.13 Preset SDK addresses long-term sprawl by making swarm shapes user-composable; v0.12 ships these as reference implementations. |
