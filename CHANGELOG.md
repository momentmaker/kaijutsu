# Changelog

All notable changes to kaijutsu (the registry + skills) and `jutsu` (the CLI). The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses [SemVer](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.12.0] — 2026-05-08

Tier A swarm presets — three new presets that share a structural signature: **hypothesis generation + cross-agent ranking by evidence**. Multi-agent disagreement IS the differentiator (single-agent produces the obvious answer + misses adversarial angles).

Picks survived a `jutsu swarm dream --mode full --lenses all` adversarial pass on 7 candidates. Spec: `docs/specs/2026-05-08-v0.12.0-tier-a-presets.md`.

### Added

- **`jutsu swarm test-gap --code <path> --tests <path>`** — surfaces failure scenarios MISSING from existing tests. Multi-agent imagines edge cases / races / resource exhaustion / malformed input / integration boundaries / state issues; synthesizer clusters by category + ranks by severity + corroboration count. Pairs with `/polish`: polish ensures tests pass, test-gap ensures they cover.
- **`jutsu swarm bug-repro "<bug description>" [--files <paths>]`** — vague bug → ranked repro hypotheses across categories (state / race / env / input / version) + minimal-repro steps for top hypothesis. Differs from brainstorm (generates SOLUTIONS) and pr-review (hunts BUGS in a diff). Bug-repro reasons from a bug REPORT — symptoms only.
- **`jutsu swarm code-archaeology --code <path> [--git-log <since>]`** — explain WHY legacy code looks the way it does. Multi-agent generates competing historical-context theories (workaround / era-pattern / abandoned-migration / perf-opt / security-mitigation / accidental-complexity) with evidence citations from git history; synthesizer marks theories as corroborated / single-source / contested. Use BEFORE refactoring legacy code; surfaces the "why is this weird" answer faster than reading commit history manually.
- **3 new public APIs** in `cli/internal/swarm/`: `BuildTestGapPrompt(testsContent)`, `BuildBugReproPrompt(filesContent)`, `BuildCodeArchaeologyPrompt(historyContent)`. All follow the v0.9 reverse-preset two-slot pattern (`{{PLACEHOLDER}}` via `strings.Replace` + literal `%` escaped to `%%` to protect downstream `fmt.Sprintf` from format-directive interpretation — the v0.8 dreamLensWild safety pattern).
- **`fetchGitLog`** in `cli/internal/cli/swarm_code_archaeology.go` — best-effort git-log fetch with graceful degradation per spec Decision #9: failures (not a git repo, gh not on PATH, no commits) yield empty history + stderr warning, NOT a hard error. 50 KB cap on captured log content (independent of 200 KB code cap).

### Changed

- **`init_agents_fragment` marker version**: `0.11.0` → `0.12.0`. Older versions still detected via the version-agnostic prefix.

### Notes

- **Dream-survival framing**: 7 candidates entered (`dep-review`, `api-review`, `test-gap`, `migrate`, `postmortem`, `code-archaeology`, `bug-repro`); 3 survived. Killed: `dep-review` (high-cost vibes-check on low-entropy data — deterministic tools win), `api-review` (commodity, IDE-native by 2028 per dream's time lens). Deferred: `migrate` (too high-stakes for a v0.12 minor), `postmortem` (harm-vector flagged: "authoritative blame reports enabling targeted harassment" — needs design before shipping).
- **Severity vocab consistency**: all 3 new presets share pr-review/reverse vocab `[blocker, issue, minor, info]` for cross-preset coherence. For code-archaeology specifically, severity means "trust this theory" (blocker = load-bearing, must verify before refactoring) rather than "code-quality severity".
- **Confidence threshold default 0.30** (matches reverse). Hypothesis-generation surfaces SPECULATIVE findings; lower threshold admits them; the synthesizer's ranking + the user's filter via the confidence column do the gatekeeping. pr-review's 0.55 default would over-filter many of test-gap's "what if there's a race here?" findings.
- **Doc-review** caught 13 spec findings + 13 plan findings; all incorporated pre-implementation. Notable: `InputKind` clarification (it names dispatch shape, not totality of bytes — auxiliary `--tests` / `--files` / `--git-log` content bakes into prompt at cobra layer, matches reverse's `--spec` pattern). 200 KB combined-input cap (spec Decision #8) + 50 KB git-log cap (Decision #9) + bug-description max 8 KB.
- **34 new tests** across 6 test files: 7 swarm + 7 cli per preset roughly, covering preset registration, prompt-slot pattern, severity vocab, two-slot escape safety, empty-input fallback, cobra wiring, validation, file-walking, hidden-dir skip, git-log graceful degradation, real-git-repo happy path.
- **Dream lens-blindspot warnings flagged but accepted**: cross-corpus diversity premise possibly self-confirming under RLHF convergence; v0.12.x will run controlled A/B (single-agent vs 3-agent shared-prompt) on benchmark fixtures to validate. If A/B shows no diversity benefit, downgrade preset to skill OR escalate to per-agent prompt specialization in v0.13.

### Deferred to v0.13+

- **`dep-review`**: deterministic tools (`npm audit`, `osv-scanner`, `snyk`) outperform multi-agent for structured-data linting. May land as `jutsu suggest dep-review-tools` (skill-routing, not preset).
- **`api-review`**: short window before vendor IDEs subsume; revisit only if user demand surfaces.
- **`migrate`**: framework migration plan generation — too high-stakes for a v0.12 minor; needs spec + adversarial review pass of its own.
- **`postmortem`**: harm-vector concerns; needs design around blame-report / scapegoating risks before shipping.
- Per-agent prompt specialization (vs current shared-prompt-per-preset). Opt-in v0.13+ if A/B shows shared prompts collapse to identical outputs.

## [0.11.0] — 2026-05-08

Two converging features land in one release: **`swarm.RunPipeline` extraction** (closes the v0.10.1-deferred Stage 1 work — `eval preset` + `eval swarm-skill` ship real implementations instead of stubs) and **autopilot v2** (kaijutsu-distributed intent-to-PR pipeline replacing the `~/.claude/skills/autopilot` v1 skill). The extraction is load-bearing for autopilot v2's reverse-drift gate (Phase 5 calls `swarm.RunPipeline` directly).

Spec: `docs/specs/2026-05-07-v0.11.0-autopilot.md`. Plan: `IMPLEMENTATION_PLAN.md` (3 stages).

### Added

- **`swarm.RunPipeline(ctx, opts) (*Result, error)`** — non-cobra public API for the swarm pipeline. Lives in `cli/internal/swarm/pipeline.go`. Accepts a structured `PipelineOpts` (mirrors `commonSwarmFlags` + I/O writers + callback fields for cli-pkg-only helpers that depend on `findings` pkg). All v0.6/v0.7/v0.8/v0.9/v0.10 swarm behavior preserved verbatim — privacy gate, consent flow, debate, lie-to-them filter, lens rotation, cache, persona resolution, telemetry warning, replay path. Used by the cobra `runSwarmPipeline` adapter, by `eval preset` / `eval swarm-skill`, and by autopilot v2's reverse-drift gate.
- **Stage 2 stub replacement**: `eval preset` + `eval swarm-skill` now invoke real `swarm.RunPipeline` per side. The v0.10 stub stderr warning is gone. Per-side `output.txt` artifacts diverge semantically (synthesis findings_count differs between sides) when the underlying preset:mode pair differs.
- **`autopilot` skill v2** (`skills/core/autopilot/`) — distributed via `jutsu install autopilot`. Replaces the `~/.claude/skills/autopilot` v1 skill. 6-phase pipeline (BRAINSTORM → SPEC → PLAN → BUILD → SHIP → LEARN), 2 gates total (post-brainstorm + GitHub PR review), multi-agent adversarial review at every artifact stage, anti-sycophancy via `jutsu swarm dream` at brainstorm, reverse-drift gate before PR opens (informational, tagged on PR). Cost-capped: $20 soft default, $100 hard ceiling in skill code, env-var override per-shell only.
- **`jutsu autopilot` cobra command group** (`cli/internal/cli/autopilot.go`) — `init` (write `.kaijutsu/autopilot.yaml` from baked-in defaults), `status` (read state file), `abort` (clean state), `resume` (read state, print resume target), `run "<intent>"` (non-interactive entry). The `/autopilot` slash command remains the primary interactive entry point inside an agent CLI session.
- **Cost cap layered model**:
  - Soft cap: `.kaijutsu/autopilot.yaml::cost.max_total_usd` (default $20) + `--max-cost N` CLI flag.
  - Hard ceiling: `MaxAutopilotCostUSD = 100.0` constant in `cli/internal/cli/autopilot.go`. Cannot be raised by editing yaml. `--max-cost` rejected if N exceeds ceiling.
  - Env override: `KAIJUTSU_AUTOPILOT_HARD_CAP_OVERRIDE=N` raises ceiling per-shell. Read once at autopilot start.
- **Test mode**: `KAIJUTSU_AUTOPILOT_TEST_MODE=1` writes the planned PR (title, body, branch, labels) to `.kaijutsu/autopilot-pr.json` instead of invoking `gh pr create`. CI-friendly without requiring a live GitHub remote.
- **3 new built-in personas** in `cli/internal/agents/builtin_personas.go` — all CLI-backed (claude/gemini/codex), no HTTP API keys required:
  - `claim-auditor-claude` — load-bearing-claim audit lens.
  - `cross-file-gemini` — cross-file-consistency lens.
  - `perf-purist-codex` — algorithmic-complexity lens.
- **Pre-existing autopilot SKILL.md archival** — when `jutsu install autopilot` runs and `~/.claude/skills/autopilot/SKILL.md` already exists, the install pipeline copies the existing file to a SIBLING dir (`autopilot.archived/SKILL.md`) before the v2 overwrite. Sibling-not-child placement matters because the install does `RemoveAll(fullDest)` before copy — a child `.archived/` would be nuked.

### Changed

- **`cli/internal/cli/swarm.go::runSwarmPipeline`**: 470 LOC inline implementation → 50 LOC cobra adapter. Real orchestration moved into `swarm.RunPipeline`; the adapter just maps cobra → `PipelineOpts` and wires cli-pkg helpers (findings recorder, dream archive) via callback fields.
- **Moved cli pkg → swarm pkg** (with exported names): `personaAdapter` → `PersonaAdapter`, `assemblePersonaJobs` → `AssemblePersonaJobs`, `pickPersonaSynthesizer` → `PickPersonaSynthesizer`, `stampDriverKind` → `StampDriverKind`, `overlayPersonaCosts` → `OverlayPersonaCosts`, `userSentinel` → `UserSentinel`, plus `BuildEstimateProjections`, `EstimateOpts`, `ListPersonas`, `ListProviders`, `ReportSwarmStderr`. Persona-dispatch tests moved with the source.
- **`init_agents_fragment` marker version**: `0.10.1` → `0.11.0`. Older versions still detected via the version-agnostic prefix.

### Notes

- **Doc-review on spec caught 14 findings**, all incorporated before implementation: undefined v0.6+ behaviors → 8-row pinning table; approved-spec capture defined in state file; PR creation automatable in CI via test-mode hook; semantic divergence replaces brittle byte-equality assertion; persona/env caching documented; reverse-drift uses `swarm.RunPipeline` (not cobra) per Stage 1's architectural intent; layered cost-ceiling threat-surface honestly framed (skill code is signed at install via cosign, yaml is unsigned per-project — different defenses).
- **Doc-review on plan caught 10 findings**, 8 incorporated: removed circular-dep fallback path (Stage 1 is load-bearing for Stages 2+3); added `jutsu autopilot resume` CLI command; clarified `state.approved_spec_path` source; dropped v1 detection heuristic in favor of file-presence-only archival; cost-ceiling abstraction-leak resolved (const lives in Go, skill markdown documents only); idempotency wording corrected to "non-destructive: refuses overwrite without --force"; Decision #6 reference repaired; `--max-cost` flag test added. 2 prompt-injection false-positives skipped (plain markdown headers).
- **Stage 1 polish caught 2 fixes**: dropped dead `EstimateFn` callback in `swarmPipelineEvalAgent.Run`; added `swarm/pipeline_test.go` covering the 5 load-bearing branches missing per Stage 1 success criteria.
- **Adversarial dream pass on the v2 design** (5 load-bearing concerns → all incorporated): cascading errors with no firewall under one-gate model → kept 2-gate default; multi-agent consensus is correlation filter not correctness oracle → never auto-apply doc-review findings; $5/run cost cap is fiction → raised to $20 soft + $100 hard; reviewer/generator boundary preserved (report-only); prompt-injection blast radius compounds → hard ceiling lives in skill code, not yaml.

### Deferred to v0.12+

- Skill-aware orchestration (`jutsu suggest "<phase intent>"` integration).
- Forkable build graph (multiple spec/plan branches at gates).
- Async gates (review on phone, autopilot resumes).
- Multi-candidate impl at final gate.
- Self-improving autopilot via PR-review feedback loop.
- Runtime behavior validation gate (current "success" = PR opened, not feature works).

## [0.10.1] — 2026-05-07

Polish patch. Tier-A/B subset of the agent-native CLI audit (per [trevinsays.com/p/10-principles-for-agent-native-clis](https://trevinsays.com/p/10-principles-for-agent-native-clis)) plus the v0.10.x deferral list re-shaped: live judge dispatch in CI is dropped from the roadmap (CLI > API key — most users have paid Pro/Max/Plus subscriptions on the native CLIs; CI judge dispatch via injected `ANTHROPIC_API_KEY` would silently re-charge them via a separate billing channel). Spec: `docs/specs/2026-05-07-v0.10.1-polish.md`.

### Added

- **`--dry-run` on `install` / `upgrade` / `remove` / `publish`**. Each command resolves + validates as normal but skips every filesystem, lockfile, and network mutation. Dry-run output uses the `[dry-run]` line prefix; `publish --dry-run` does NOT exec `gh` or `git`. Per the agent-native CLI audit principle #4 (safe retries + explicit mutation boundaries).

### Changed

- **Error messages now enumerate valid options** when rejecting an enum value. v0.10.0 sites without enumeration: `resolveEvalAgent` (target/judge agent name), `personaProjections` + `personaJobs` (persona name + provider name), `skill.Validate` (hook event). All now name the valid set inline so an agent or human can self-correct in one retry instead of trial-and-erroring against `--help`. Per the agent-native CLI audit principle #3.
- **`init_agents_fragment` marker version**: `0.10.0` → `0.10.1`. Older versions still detected via the version-agnostic prefix.

### Notes

- **Doc-review caught 7 findings** against the initial spec (claim-auditor-deepseek + performance-deepseek). All 7 incorporated before implementation: unsupported subscription claim softened, `eval swarm-skill` success criteria added, `publish --dry-run` verification added, manual error spot-check upgraded to 8-site touch list + 3 named regression tests, "meaningful diff" replaced with "non-byte-identical output.txt", missing `--evals` edge case noted, `runSwarmPipeline` LOC measured (470, not the initial estimate of 150).
- **Stage 1 of the v0.10.1 plan** (replacing the v0.10 Stage 2 stubs in `eval preset` / `eval swarm-skill` with real `swarm.RunPipeline` integration) is **deferred** — `runSwarmPipeline` is 470 LOC of cobra-coupled logic, larger than fits a polish patch. Stage 1 work is moved to v0.11.0 with its own spec.

## [0.10.0] — 2026-05-08

`jutsu eval` runner — port of agentskills.io / agent-skills-eval upstream to Go + kaijutsu-native swarm-shape eval extensions. Spec: `docs/specs/2026-05-08-v0.10.0-eval-runner.md`. Plan: `IMPLEMENTATION_PLAN.md` (3 stages).

### Added

- **`jutsu eval skill <path>`** — single-skill eval (parity with agent-skills-eval upstream). Reads `<skill>/evals/evals.json` files unchanged (3 captured fixtures pinned under `cli/internal/eval/testdata/agent-skills-eval-fixtures/v1/`). Runs target × 2 (with_skill / without_skill), judge grades each output, writes iteration-N artifact tree, generates static HTML report.
- **4-stage tolerant judge JSON parser**: raw json.Unmarshal → balanced-bracket extract (string + escape aware; handles nested objects) → retry with stricter prompt suffix → indeterminate. `--strict` treats indeterminate as failure (safe default).
- **`jutsu eval persona`** — head-to-head two personas on the same prompt. Side dirs use persona names verbatim. Parallel dispatch (concurrent persona runs are cheap; each = single agent invocation).
- **`jutsu eval preset`** — head-to-head two preset modes (e.g. dream quick vs full). Sequential dispatch (each side = full swarm pipeline; parallel would blow `--max-cost`). Stage 2 stub: dispatches claude regardless of preset:mode pair with one-shot stderr warning so users don't read green output as "preset modes meaningfully differ" — real swarm.runSwarmPipeline integration is v0.10.x.
- **`jutsu eval swarm-skill`** — does loading a skill into a swarm preset's pipeline change outcomes. Same stub limitations as preset.
- **`kaijutsu.{swarm,personas,presets}`** extension blocks under evals.json's top-level `kaijutsu:` key. Forward-compat: upstream parsers ignore the block per JSON-permissive parsing. `additionalProperties: false` is set at the top level but the upstream-parser SKIP test pinned in `cli/internal/eval/upstream_compat_test.go` will validate the contract once a stable agent-skills-eval `--validate` flag lands (v0.10.x candidate).
- **`evals/judge.md`** per-skill judge override. Required placeholders: `{assertion}` and `{output}` — missing either hard-fails at LoadJudgeTemplate parse time. Judge prompt template carries INPUT-INTEGRITY rules (prompt-injection mitigation per the v0.4 pr-review pattern).
- **Workspace lock** with TTL/PID stale-clear: PID-not-alive OR mtime>2h auto-clears with stderr warning. Boundary tests pin the 2h threshold (1h59m blocks, 2h01m clears). Windows portability via `pidalive_windows.go` build-tag split.
- **Cost guard**: pre-flight `--max-cost` cap (default $20) aborts BEFORE any model dispatch when estimate exceeds cap. Mid-suite per-call ratio guard aborts pending evals when actual per-call cost > 2× estimate. Mirrors v0.9 dream cost prompt discipline. Parity across all 4 subcommands (skill + 3 swarm shapes).
- **Static HTML report** with embedded JSON. No external CSS/JS, no infra to host. Renders pass/fail summary table + per-eval drill-down + cost rollup + line-level diff view (200-line cap per side, non-deterministic noise stripped via fixed regex set). HTML-escapes user content (skill names, eval names, assertions, reasons) to prevent XSS in hosted reports.
- **Path-traversal sanitization** on eval IDs + side names. Defense-in-depth alongside parser-level validation; malicious or malformed inputs can't escape the iteration directory.
- **`--baseline-from <git-ref>`** stateful `--strict` mode. Reads prior tag's `eval-baseline.json` artifact via `git show <ref>:<path>`, compares per-(eval-id, side) pass/fail. Regression = challenger fails what baseline passed. First-tag-with-coverage policy: missing prior baseline → exit 0 with warning; broken-floor seeding rejected unless `--accept-baseline` flag passed.
- **CI workflow** `.github/workflows/eval-skills.yml` runs the eval-package test suite + cobra-wiring smoke + `evals.json` parse-check on every push to main, every PR touching the eval surface, every `v*` tag, and `workflow_dispatch`. Stage 3 ships the harness gate; live judge dispatch + stateful `--baseline-from <prior-tag>` regression check are scoped to v0.10.x once secrets-injected `ANTHROPIC_API_KEY` is wired.
- **3 core skills now eval-covered**: `dream`, `pr-review`, `scope-check` ship with `evals/evals.json`. Coverage gate at `TestEvalCoverage_CoreSkillsHaveEvalsJson` fails CI if any allowlisted skill loses coverage.

### Changed

- **`init_agents_fragment` marker version**: `0.9.0` → `0.10.0`. Older versions still detected via the version-agnostic prefix; replace stays clean.

### Notes

- **Adversarial swarm pr-review** caught real bugs across all three Stage commits. Stage 3: 7 fixes — `LoadBaselineFromGitRef` switched from English-stderr substring matching to a locale-independent two-step probe (`git rev-parse --verify` then `git cat-file -e`); `filepath.ToSlash()` on the baseline path before `git show` (Windows `\` broke the spec); `projectRoot()` + `filepath.Rel()` errors now surface instead of silently falling back to absolute paths; `--baseline-from` now requires `--strict` (matches CHANGELOG framing); 5 new tests for `LoadBaselineFromGitRef` covering happy path, absent-at-tag, bad-ref, bad-JSON, empty-args; `eval-skills.yml` rewritten as honest harness gate (live dispatch deferred, would have failed CI on first tag); missing `fixtures/sample.diff` for the dream `kaijutsu.swarm` extension. Final cumulative pr-review across `main..HEAD`: 4 fixes — `filepath.Abs` on both args before `filepath.Rel` (mixed-abs/rel from relative `--workspace` would have errored on a real run); `--baseline-from` validation moved to top of `RunE` (was running after the full eval suite, wasting wallclock + budget); `LoadBaselineFromGitRef` now requires explicit `repoRoot` arg + sets `cmd.Dir` on every git invocation (would have walked up to wrong repo from a subdirectory); `truncateOutput` rewinds to a UTF-8 rune boundary (byte-indexed slicing produced invalid UTF-8 in the HTML report). Stage 1: 4 fixes (Windows portability via build-tag split, balanced-bracket JSON parser replacing the naive non-greedy regex, ctx-aware sem acquire, `matchesAny` pattern grammar documented). Stage 2: 7 fixes (`--max-cost` parity in swarm-shape runners, presetModeResolver stderr stub warning, B==C parse validation, shape-kind switch default branch, `MarkFlagRequired("evals")`, ctx-aware sem acquire on parallel persona path, `personaResolver` lookup discipline). Different agents caught different lenses — claude on design correctness, gemini on parity/consistency, deepseek (v4-flash, ~$0.01-0.02 per review) on perf/portability.
- **Stage 2 stubs are loud, not silent**. preset/swarm-skill modes both dispatch claude regardless of preset:mode pair (real swarm.runSwarmPipeline integration is v0.10.x). The stub emits a one-shot stderr warning so users don't ship product decisions on green-but-meaningless output.

### Deferred to v0.10.x

- Real preset/swarm-skill dispatch via `swarm.runSwarmPipeline` (Stage 2 stub returns claude regardless of preset:mode).
- Persona-registry resolution against `agents.yaml` (Stage 2 supports claude/codex/gemini drivers only).
- Upstream-parser forward-compat test wiring (pending stable agent-skills-eval `--validate` flag).
- `findings.db` integration (synthetic eval pass/fail vs user accept/dismiss precision math — needs an ADR; storing both signals could pollute the v0.7 weighter).
- Concurrency parity: `RunPersonaSuite` is parallel; preset + swarm-skill stay sequential because each side = full swarm pipeline (parallel would blow `--max-cost`). Documented Stage 2 invariant.
- Move `TestEvalCoverage_CoreSkillsHaveEvalsJson` allowlist from test source to `skills/core/.eval-coverage.yaml` manifest so the source-of-truth lives next to the skills.
- Live judge dispatch in CI (Stage 3 ships the harness; secrets-injected `ANTHROPIC_API_KEY` for `gh actions` test runs is v0.10.x).
- `jutsu eval bench` — cross-skill comparison ("which skill is best at X?").
- HTML reasoning-diff (judge-side reasoning comparison; v0.10 ships pure-output diff only).
- Cross-eval diffing ("v1.0.0 of skill X now fails 3 evals it passed in v0.9.5") — needs eval-result history storage, separate v0.11+ feature.
- Eval marketplace / shared eval suites (`jutsu install eval:foo`).
- Three nearly-identical regression-check functions in runner_swarm.go could collapse to one generic helper. Refactor-only; doesn't affect correctness.
- `concurrency` flag accepted but ignored by preset/swarm-skill subcommands (parity gap with persona).
- Judge cost estimate omits prompt-template overhead (perf-accuracy nit; cost shown is conservative underestimate).

## [0.9.1] — 2026-05-07

Patch fixing 3 user-surfaced ergonomics issues. No new features.

### Fixed

- **Constraint resolver: dep version mismatched against repo tags.** `deps.skills: doc-review@^0.1` resolved to repo tag v0.1.0 (where doc-review didn't exist yet), causing install to fail with "skills/core/doc-review/ doesn't exist at this ref". Fix: `resolveRef` now walks tags newest→oldest, fetches the skill's `skill.yaml` at each candidate ref via raw.githubusercontent.com, and matches the constraint against the SKILL's internal version (not the repo tag's). Affected `spec-driven-development` + `planning-and-task-breakdown` (both depend on `doc-review@^0.1`); should also fix any other skill with cross-version deps.
- **`jutsu list` showed "no skills installed" in directories without project lockfiles**, even when global skills were present. Fix: when no project lockfile exists AND `--global` wasn't passed, fall back to the global lockfile + label the source. Users who only install globally now see their installed skills by default; explicit `--global` still works.
- **`jutsu search "."` (matches everything) scrolled off-screen** with multi-sentence descriptions. Fix: truncate description to 80 chars in the table view; add a footer with match count + hint to use `jutsu info <name>` for full descriptions. New `--full` flag suppresses truncation.

## [0.9.0] — 2026-05-07

Feedback-loop hardening — five-item bundle closing v0.7+v0.8 gaps. Spec: `docs/specs/2026-05-07-v0.9.0-feedback-loops.md`. Plan: `IMPLEMENTATION_PLAN.md` (stages 1-5).

### Added

- **Lens schema migration** (`0002_lens.sql`) — `lens TEXT` + `position INTEGER` columns on findings table. LIKE-based backfill for pre-v0.9 dream rows; `idx_findings_lens_lookup` and `idx_findings_run_position` indexes added. Backfill restricted to `preset='dream'` so non-dream rows with literal `[lens:...]` summary prefixes don't get false-tagged.
- **Per-lens precision tracking** (`Weighter.WeightForLens`) — 3-tier fallback: lens-specific → tuple-without-lens → cold default. Synthesizer reads per-lens weights via new `SynthOpts.LensWeights` field; canonical 8-lens cycle order in the `lens_weights:` prompt block.
- **`KAIJUTSU_DREAM_ADAPTIVE_LENS=off`** killswitch suppresses the lens-weights block entirely (synth output byte-identical to v0.8.3).
- **Dream Pass-2 debate** — real critique template emits `[new]/[disputes]/[revised]/[agreed]` revision tags; preserves `[lens:<name>]` prefix + `load_bearing:` reasoning prefix; `[agreed]` documented as requiring peer count >= 2. Cobra reject on `--mode full` for dream lifted.
- **Cost prompt + non-TTY hard-fail** — `swarm dream --mode full` confirms estimated cost interactively; non-TTY without `--yes` hard-fails (refuses to dispatch silently in CI).
- **Lens rotation rule** — `--mode full` repeat-dreams within 7d shift the LEAD lens through the canonical 8-lens cycle. `RotateLensOrder` + `ReadDreamLensOrder` + `tryRotateDreamLenses` helpers; threaded via `commonSwarmFlags.dreamLenses`.
- **`jutsu swarm reverse`** — new spec-vs-impl drift detector. `--spec <path>` + `--diff <range>` (default `origin/main...HEAD`); ADDED/OMITTED/CHANGED/AMBIGUOUS categories; `BuildReversePrompt` bakes spec via `{{SPEC_CONTENT}}` placeholder + escapes `%` → `%%` to protect downstream `fmt.Sprintf`. `ReverseTruncate` enforces `MaxGitHubCommentBytes = 60_000` cap deterministically (severity, file, line_range_start ordering). `MarkerReverse` distinct from `kaijutsu-pr-review`. `--confidence-threshold` defaults to 0.30 (vs pr-review's 0.55) — drift detection benefits from admitting more findings. `--lie-to-them=on|off` override (off by default).
- **`SynthOpts.ConfidenceFloor`** — drops findings below threshold before clustering. Zero floor = byte-identical v0.7 behavior. Reverse preset's lower default ships through this; other presets opt in via the same field.
- **Per-skill provider routing** — `skill.yaml` `routing:` field declares per-persona + default preferred-provider lists. JSON schema + `Routing` struct in `cli/internal/skill/`. `ResolvePersonaProvider` 3-phase resolution (declarative → runtime availability → minimum-dispatch invariant).
- **`KAIJUTSU_DISABLE_AGENTS=<comma-list>`** env-var honored at `Available()` — escape hatch for forcing a provider subset without uninstalling CLIs.
- **`jutsu finding sync-pr <pr>`** — ingests human accept/dismiss decisions from PR comment replies into the findings DB. Reply-keyword grammar: `^[\s>]*(accept|dismiss)\s*:\s*<run_id>:<pos>\s*$` (case-insensitive verb, `>`-quoting tolerated). Dry-run by default; `--apply` writes. Idempotent on re-run. 30s timeout on `gh` subprocess prevents CI stalls. `FindByRunPosition` resolves `(run_id, position)` → finding row.
- **`jutsu finding seed`** dev-only subcommand — gated by `--dev` flag (uses `cmd.InheritedFlags()` for robust ancestor lookup) OR `KAIJUTSU_FINDING_SEED=1`. `SilenceUsage:true` so the gate error doesn't leak the Hidden subcommand's existence. Per-row monotonic `action_at` increment for deterministic weighter window selection.
- **`LensFromSummary`** helper owns the `[lens:<name>]` regex (recorder + validator share).

### Changed

- **Diff cap 200KB → 500KB**, **files cap 200KB → 500KB**, **per-agent timeout 180s → 600s** — surfaced when running adversarial swarm pr-review on the v0.9 PR itself (252KB diff). 500KB ≈ 125K input tokens, still well under claude's 1M context. Cap remains as safety net against accidental "merge of 50 commits" → unintended remote-API blast.
- **`init_agents_fragment` marker version** `0.8.2` → `0.9.0`. Older versions still detected via the version-agnostic prefix and replaced cleanly.

### Notes

- **Adversarial swarm pr-review** caught 6 real bugs squashed before tag — 1 BLOCKER from gemini (spec content with `%` chars broke downstream `fmt.Sprintf` despite the `strings.Replace` indirection — escape `%` → `%%` before bake), 5 ISSUEs from claude + deepseek (missing `(run_id, position)` index on sync-pr hot path, gh subprocess no timeout, shared action_at across seeds, `seedDevGate` parent-walk vs `InheritedFlags`, Hidden seed cmd help dump on gate error). Different agents caught different bugs — claude on design/correctness, gemini on format-string traps + dry-run drift, deepseek (v4-flash) on perf + I/O bounds.
- **In-session pr-review** (claude lens applied directly without subprocess) caught 1 ISSUE the swarm crashed on: bot-marker skip filter case-sensitive vs case-insensitive regex mismatch.

### Deferred to v0.9.x

- `--post-review` render mode (per-finding line-anchored review comments + reactions)
- `--auto-sync` flag on pr-review
- `--reverse-spec` combined trigger on pr-review
- Draft-PR softer-header detection in reverse
- Per-finding marker emission in pr-review renderer
- `routing.disabled` escape hatch in `~/.kaijutsu/agents.yaml`
- `applySyncPRActions` per-batch transaction wrapping (re-run is idempotent so partial failure recovers)
- `(run_id, position)` UNIQUE constraint at schema (in-memory counter handles correctly in practice)
- Confidence-threshold full plumbing for non-reverse presets
- Warning when diff > 100KB AND timeout < 300s (observe usage first)

## [0.8.3] — 2026-05-07

Quick-wins bundle from the v0.7.x + v0.8.x followup backlog. Seven small-but-high-value items shipped in one release rather than dribbled across micro-tags.

### Added

- **`--db` flag for `jutsu finding *`** — persistent across the whole `finding` group. Resolution: `--db` flag > `KAIJUTSU_FINDINGS_DB` env > default `~/.kaijutsu/findings.db`. Closes a v0.7 deferral (env-var-only was test-friendly but user-hostile).
- **`jutsu dream` subcommand group** — `jutsu dream list` (browse the graveyard for current codebase) + `jutsu dream clear --older-than 365d --yes` (auto-prune). Mirrors the `jutsu finding clear` UX (dry-run default, `--yes` for destructive op, `--all-codebases` to escape scope).
- **Dream graveyard auto-write** — every `jutsu swarm dream` invocation now archives its synthesis output to `~/.kaijutsu/dreams/<fp>-<slug>-<YYYYMMDD-HHMMSS>.md` (file mode `0600`, dir mode `0700`). Header carries `topic` (YAML-quoted) + `codebase_fp` + `lens_order` + `mode` + `full` + `created_at`. Lens order in header preps the future lens-rotation rule on repeat-dreams within 7 days. New helpers: `WriteDreamSession` / `FindRecentDreamForTopic` / `SlugifyTopic` / `SanitizeFP`.
- **Synthesizer-coda truncation post-processor** (`swarm.StripDreamCoda`) — programmatic backstop to the prompt-level HARD STOP rule in the dream synthesizer. Strips trailing "In summary" / "Overall" / "In conclusion" / "Let me know" / etc. paragraphs that models occasionally emit despite the prompt instruction. Skips lines inside markdown tables + code fences. Targets the LAST coda phrase (so an "Overall" used earlier as a transitional marker doesn't truncate downstream content).
- **Runtime lens-prefix validation** in the recorder — dream-preset rows that fail the `[lens:<name>]` summary prefix or `load_bearing: true|false` reasoning prefix get skipped at recorder time with a stderr count. Whitelist of 8 lens names. Whitespace-tolerant (extra/zero whitespace around `:` accepted, case-insensitive bool). Keeps v0.9 schema migration source data clean.
- **Per-file anti-sycophancy regression test** for `skills/core/dream/prompts/{base,extras}/*.md` — Go-side test only covered the assembled swarm-preset prompt; the markdown files (loaded by standalone `/dream`) were unguarded against accidental softening of the forbid list.

### Changed

- **Recorder hook moves below Debate** — `--full` mode now records the MERGED Pass-1 ⊕ Pass-2 result set, so `[new]` / `[disputes]` / `[agreed]` revisions land in the v0.7 findings DB. `--quick` mode unchanged. New `swarm.MergePasses` public symbol; `cli/swarm.go` computes the merged set + passes to `recordFindingsBestEffort`.
- **`findings.RecordRun` signature**: `(int, error)` → `(int, int, error)`. Second int is the dream-validation skip count. Tests + `swarm_recorder.go` updated.

### Notes

- **Lens-rotation rule on repeat-dreams** is half-shipped: `WriteDreamSession` records `lens_order` in the header (the prerequisite), but the actual rotation logic on repeat-within-7-days isn't wired in v0.8.3 because `--lenses` flag selection isn't yet plumbed through to the swarm dispatch path. v0.8.x followup.
- **All 7 items** had been in the v0.7.x or v0.8.x backlog as deferrals. Bundle ships them together to close the "half-done" feeling without dribbling micro-patches.
- **Adversarial pr-review** caught 5 real fixes squashed before tag (StripDreamCoda first-vs-last bug, `WriteDreamSession` hardcoded `full: false`, `FindRecentDreamForTopic` missing fp-sanitize, validator over-strict on whitespace, YAML topic value unquoted). All fixed; 1 false positive dismissed (openFindingsStore signature concern — only finding subcommands call it).

## [0.8.2] — 2026-05-07

Agent-first design lens + four agent-discoverability features. Sets the foundation for fresh AI agents to use jutsu without reading project docs.

### Added

- **Design principle codified in `AGENTS.md`**: "agent-first, human-friendly". Default lens for every command / output / behavior. Machine-parseable + well-formed FIRST. Aesthetically pleasing for humans SECOND. They compose ~90% of the time.
- **`output.AutoFormat()` helper** + `BindFormatFlags()` (`cli/internal/cli/output.go`): TTY detection picks markdown / table / colored when stdout is a TTY, JSON when stdout is a pipe / redirect / agent capture. `--format json` / `--json` / `--format markdown` overrides. Pattern matches `gh` / `jq -C` / `kubectl`. Lands as helper in v0.8.2 — applied to NEW commands (suggest, describe); existing commands gain auto-format as v0.8.x followups.
- **`jutsu describe`** (`cli/internal/cli/describe.go`): JSON catalog of every command + flag + description. Always JSON regardless of TTY (this command exists FOR agents, not humans). Hidden from `--help` listing — agents discover via convention. Schema versioned at 1; agents can detect format changes without breaking on unknown fields.
- **`jutsu suggest <task>`** (`cli/internal/cli/suggest.go`): keyword-rank installed skills against a free-form task description. Walks `skills/core/` + `skills/community/`, scores each by token overlap (name 3x weight, tags 2x, description 1x per token). Returns top-N ranked. Auto-formats output. Stable JSON shape (`schema_version` + `task` + `count` + `suggestions[]`) for agent consumption.
- **`jutsu init` writes AGENTS.md fragment**: kaijutsu-managed delimited block (`<!-- kaijutsu:start name=jutsu-cli version=0.8.2 -->` / `<!-- kaijutsu:end -->`) teaching fresh agents the jutsu CLI surface — describe / suggest / install / swarm / finding quick reference. Idempotent re-init replaces content between markers; user content outside markers preserved verbatim. `--skip-agents-md` flag for users managing AGENTS.md separately.
- **`docs/project-memory.md`**: schema convention doc (not installable skill). Restored from the `project-memory` skill deleted in v0.8.1's curation pass. Cross-skill memory contract: directory layout + entry schema + read/write/dedupe rules. Consumers (`unstuck`, community/`session-retro`, community/`journal`) reference this doc.
- **`AGENTS.md` refresh**: stripped stale v0-stage references ("currently empty / stubbed" — every claim was false). Added "Current state (as of v0.8.1)" summary + "Recent design decisions" pointers. Rewrote "Don't do" with v0.8 era rules (don't break agent-first lens, don't ship TUIs, don't conflate kaijutsu.json with `.kaijutsu/agents.yaml`, etc.).

### Notes

- **No new commands consume `output.AutoFormat()` yet beyond suggest + describe.** Existing commands (list, search, info, finding-list, finding-stats, swarm) gain auto-format as v0.8.x followups — applying across all of them was scoped out of v0.8.2 to keep ship size tractable.
- **MCP server explicitly NOT shipping in v0.8.x.** Dream session on the MCP idea (2026-05-07) found the "self-discovery via MCP" premise was partially false (users still hand-configure MCP per client). Status quo (CLI + AGENTS.md fragment + JSON outputs) reaches more clients with less risk. Revisit when MCP protocol stabilizes.
- **`jutsu suggest` v0.8.2 ships keyword ranking.** v0.9.x may swap for embedding similarity or LLM-based ranking once usage signal informs the right shape. Outside the kaijutsu monorepo `jutsu suggest` returns no matches — registry-remote scan is v0.9 work.
- **No interactive TUIs / charm-ecosystem deps added.** Per design principle: TUIs break pipes, contradict project philosophy. Auto-format gives humans pretty output without sacrificing agent pipeline-friendliness.

## [0.8.1] — 2026-05-07

Patch release. Two changes:

- **Fix**: dream Wild lens `'10% better'` literal corrupted topic substitution via `fmt.Sprintf` — agents received `%!s(MISSING)` instead of the user's topic. Single-character escape (`'10%'` → `'10%%'`). Regression test added (parses every dream prompt for fmt-error markers across base + extras lens sets).
- **Curation**: core skills tier 28 → 17. Moved to `skills/community/`: code-simplification, security-and-hardening, journal, session-retro, readme-update, dcg. Deleted: decide, agent-doctor, multi-model-synth, lie-to-them, project-memory (schema doc moves to `docs/project-memory.md` in v0.8.2). Cleaner first-party canon for fresh users + agents discovering kaijutsu.

## [0.8.0] — 2026-05-07

`dream` skill + `swarm dream` preset. Pre-implementation interrogation primitive — walks any topic through 4-8 cognitive lenses with anti-sycophancy gates baked into every prompt. Standalone `/dream` for single-agent walks; `jutsu swarm dream` for the multi-agent matrix.

Spec: [`docs/specs/2026-05-07-v0.8.0-dream-skill-and-preset.md`](docs/specs/2026-05-07-v0.8.0-dream-skill-and-preset.md).

### Added

- **`dream` skill** at `skills/core/dream/` (rich layout). 8 lenses split into 4 base (always run) and 4 opt-in extras:
  - **Base**: honest · fit · gaps · wild
  - **Extras**: adversary · inverse · status-quo · time
  - Coverage balance: 4 critical, 2 generative, 1 contextual, 1 temporal.
- **Anti-sycophancy gates** baked into every lens prompt: explicit forbid list of "Great question!" / "Excellent!" / "You're absolutely right!" / hedging language. The Go-side regression test (`TestBuildDreamPrompt_BaseHasAntiSycophancy`) asserts the forbid phrases survive in the assembled swarm-preset prompt across future edits. The skill-side markdown prompts (`skills/core/dream/prompts/`) carry the same forbid list but are not yet covered by an automated per-file test — v0.8.x followup.
- **`load_bearing` calibration** — 4 concrete criteria for marking a finding load-bearing: changes whether to proceed / reveals new constraint / hidden assumption invalidated / "wait — that changes things" reaction. Drives consistent severity mapping (`load_bearing` × confidence → blocker / issue / minor / info).
- **`jutsu swarm dream <topic>`** preset — multi-agent matrix mode. Each persona runs ALL selected lenses; synthesizer aggregates the N×lenses cells into a 4-section report:
  - Load-bearing insights (top, sorted by confidence)
  - Cross-lens consensus (insight surfaced from 2+ lenses)
  - Lens-unique findings (single cell — possible noise OR the perspective others missed)
  - Lens-blind-spots warning (all-agents-agreed on same lens → suspected model-shared training bias, not consensus signal)
- **`--lenses` flag** controls the lens set: `base` (default, 4), `all` (8), or comma-list (subset). Whitespace-tolerant + dedupes + validates each name.
- **Default `--max-cost` raised** for swarm dream: 3.00 (base) / 5.00 (`--lenses=all`). Mirrors the matrix-size cost increase. Users override via `--max-cost N` or `--estimate` to project actual cost first.
- **`--mode full` rejected for dream** in v0.8.0 — Pass-2 debate over lens-cell findings is undefined. Error message redirects users to `--lenses=all` for matrix-expansion. Lifted in v0.8.x once a dream-debate template is designed.
- **Lens-in-summary encoding** records dream output to v0.7 findings DB without schema migration. Each finding's `summary` field starts with `[lens:<name>]` (whitelist of 8 names); `reasoning` field starts with `load_bearing: true|false`. v0.8.x will migrate to a dedicated `lens TEXT` column once 30+ real dream sessions inform the right shape.
- **Severity vocab unchanged**: dream uses standard kaijutsu vocabulary (blocker / issue / minor / info), keeping it composable with `jutsu finding *` subcommand group from v0.7.
- **Synthesizer hard-stop rule** guards against the trailing-coda failure mode at the prompt level: "if you find yourself starting any sentence after the final table that doesn't BELONG to one of the four sections, STOP." This is a model-instructed guardrail, not programmatic enforcement; runtime truncation deferred to v0.8.x once we see whether models violate the rule in practice.

### Changed

- **Skill catalog grows to 28 core skills** (was 27); sitegen catalog at 51 entries (was 50).
- **`registry/index.json`**: pointers to 3 anthropic skills (skill-creator, frontend-design, mcp-builder) and 5 obra/superpowers skills (subagent-driven-development, using-git-worktrees, verification-before-completion, systematic-debugging, executing-plans). Authors maintain upstream; jutsu just routes.
- **Existing skill upgrades**:
  - `verification-before-completion` ported as a new kaijutsu primitive (not finding-triage gate; verifies CODE state).
  - `unstuck` v0.3 — Step 0.5 evidence-gathering before articulation (iron law adapted from obra/superpowers/systematic-debugging).
  - `polish` v0.3 — Step V verification gate composes verification-before-completion to enforce convergence ≠ correctness.
  - `incremental-implementation` v0.2 — per-slice two-stage review chain (spec-compliance subagent first, then code quality via polish), continuous-execution rule between independent slices.
  - `planning-and-task-breakdown` v0.3 — plan-doc header template, file-structure-up-front section, bite-sized 2-5-minute checkbox steps inside each slice.

### Privacy

- **No new network surface.** Dream skill composes existing swarm primitives which carry their own v0.7 privacy boundaries. Dream itself adds nothing.
- **Graveyard at `~/.kaijutsu/dreams/`** (Stage 3 deferred to v0.8.x for the auto-write helpers): directory mode `0700`, files `0600` when manually written. Same boundary as findings.db.

### Notes

- **Cold-start UX**: dream output looks identical for a new user vs an experienced user — there's no per-(persona, lens) precision tracking yet. v0.8.x will add the dedicated `lens TEXT` column on findings + Weighter integration so the synthesizer can downweight low-precision lenses per codebase.
- **Anti-sycophancy is a prompt-level guardrail**, not a runtime check. Models may still produce performative agreement under load. The file-content regression test guards against accidental softening of the prompt text; runtime output verification deferred to v0.8.x (needs an eval harness which kaijutsu doesn't yet have).
- **`brainstorm` preset and `dream` skill are NOT substitutes.** brainstorm = "give me 5 options to solve X"; dream = "is X worth pursuing?". Use them in sequence on big decisions: dream first to validate the problem shape, brainstorm to enumerate solutions.

## [0.7.0] — 2026-05-06

Quality fingerprinting + confidence-weighted synthesizer. `jutsu swarm` now learns from your accept/dismiss actions: every finding goes into a local SQLite store at `~/.kaijutsu/findings.db`, and the synthesizer weights each agent's vote by its observed precision per (provider, persona, preset, codebase) tuple. The 51st run finally weights gemini's pattern-consistency lens differently from the 50 you've already dismissed.

Spec: [`docs/specs/2026-05-06-v0.7.0-quality-fingerprinting.md`](docs/specs/2026-05-06-v0.7.0-quality-fingerprinting.md).
ADR: [`docs/decisions/2026-05-06-quality-fingerprinting.md`](docs/decisions/2026-05-06-quality-fingerprinting.md).

### Added

- **Local SQLite findings store** at `~/.kaijutsu/findings.db` (mode `0600`, WAL journal). Pure-Go driver via `modernc.org/sqlite` — no CGo, goreleaser cross-compile stays clean. Forward-only migration loader (`cli/internal/findings/migrations/NNNN_*.sql`); `schema_version` table tracks applied migrations. Minimum SQLite version: 3.8 (partial-index syntax `WHERE user_action IS NULL`).
- **Codebase fingerprint** — 5-step resolution chain (`cli/internal/findings/fingerprint.go`): git origin → upstream → alphabetical-first remote → `local-git:<hash>` → `local-fs:<hash>`. Anchors on `(remote, work-tree-basename)` so different fork dirs get distinct fps but identical clones across machines collide.
- **`jutsu finding` subcommand group**:
  - `list [--run <id>] [--pending] [--all-codebases] [--codebase <fp>]` — defaults to most recent run for current cwd's codebase.
  - `accept <id> [--reason "..."] [--cross-codebase]` / `dismiss <id> [--reason "..."] [--cross-codebase]` — cross-codebase guard refuses by default when the targeted finding's `codebase_fp` differs from cwd's.
  - `stats [--codebase <fp>] [--all-codebases]` — per-(provider, persona, preset) precision report. Tuples with < 10 actioned findings render as `(insufficient data)` + `0.70 (bootstrap)` weight; ≥ 10 render the clamped precision. Header includes the current codebase fp for copy-paste into other commands.
  - `clear [--codebase <fp>] [--older-than 90d] [--yes] [--dry-run] [--all-codebases]` — destructive; defaults to dry-run unless `--yes`. Days-suffix duration parser (`90d`, `12h`, `30m`); negative durations rejected so `--older-than -1h` can't resolve to a future cutoff. Runs `VACUUM` after delete.
  - `export <path> [--codebase <fp>] [--all-codebases]` — portable JSON dump with top-level `schema_version: 1` for forward-compat imports.
- **`Weighter.WeightFor(provider, persona, preset, codebaseFp)`** in `cli/internal/findings/weighter.go` — three-state algorithm:
  - Cold-start (DB absent OR 0 actioned for tuple) → `1.0` (byte-identical v0.6.2 behavior).
  - Bootstrap (1 ≤ actioned < 10) → `0.7` (slight skepticism without dismissing).
  - Mature (actioned ≥ 10) → `accepted/(accepted+dismissed)`, clamped to `[0.05, 1.0]`.
  - Sliding window of 200 actioned findings per tuple, measured by `action_at` — drift catches up after ~50 actioned findings against a new model.
- **Synthesizer integration** — `clusterFindings` secondary sort uses `weighted_consensus = sum(unique reporter weights)` (each agent contributes its weight EXACTLY once per cluster, even if it merged multiple findings). Tiebreaker order: weighted_consensus desc → ConsensusOf desc → severity desc → key asc. Cold-start (no weights data) collapses byte-for-byte to v0.6.2 ordering.
- **`--show-weights`** flag on every swarm subcommand. Off by default in v0.7 — adopt weights via `jutsu finding stats` first; v0.7.x or v0.8 may flip the default.
- **Synthesizer prompt `weights:` section** — emitted only when at least one weight differs from 1.0. Cold-start prompts are byte-identical to v0.6.2.
- **Best-effort recording** — DB write failures emit a single stderr warning line and the swarm pipeline continues. Quality fingerprinting is non-critical; a corrupt or read-only DB never blocks the markdown render.
- **`KAIJUTSU_FINDINGS_DB` env override** — used by tests for isolation; user-facing override via `--db` flag deferred to v0.7.x.

### Changed

- **`swarm.Synthesize` + `SynthesizeWithDebate` signatures** — both now take a `SynthOpts` struct (Weights + ShowWeights). Empty SynthOpts reproduces v0.6.2 behavior byte-for-byte. `--replay` passes empty SynthOpts so cached output stays reproducible across weight updates.
- **`personaAdapter`** gains `providerName` so the recorder + weighter can resolve provider per persona without re-loading the agents registry.

### Privacy

- **No network calls** from any `jutsu finding` subcommand. Enforced by an import-list test (`finding_test.go`): `cli/internal/cli/finding.go` MUST NOT import `net/*`, bare `net`, or `golang.org/x/net/*`. Catches both intentional telemetry and accidental drag-ins.
- **`jutsu finding clear`** wipes targeted rows + runs `VACUUM` to reclaim disk. `jutsu finding export` writes local JSON only — never POSTs anywhere.
- **`~/.kaijutsu/findings.db`** lives in `$HOME`, not in any repo, never reachable from git history. Mode `0600` re-asserted on every Open.

### Notes

- **Cold-start UX**: synthesis behavior is byte-identical to v0.6.2 for users with no findings DB OR with a fresh DB that hasn't accumulated any actioned findings yet. The feature delivers value after ~10 actioned findings per tuple — `jutsu finding stats` shows progress toward that threshold.
- **`--full` mode caveat**: the recorder hook fires after Pass-1 fan-out, before `swarm.Debate`. Pass-2 `[new]`/`[disputes]`/`[agreed]` revisions don't get DB rows in v0.7. Weight math is unaffected (agent identity is preserved across passes); summary text divergence is the visible cost. Move scheduled for v0.7.x.
- **Performance**: `BenchmarkWeightFor` against a 50K-row DB returns in ~80μs per lookup on an Apple M3 (spec budget < 5ms).
- **Schema lock-in mitigated** by the migration framework + forward-only design + ADR captures the rationale.

## [0.6.0] — 2026-05-06

Multi-provider agents. The biggest CLI surface change since v0.4 — `jutsu swarm` is no longer locked to claude/codex/gemini. Driver abstraction lets users plug in any OpenAI-compat HTTP provider (DeepSeek, GLM, Kimi, local Ollama), wrap a native CLI with env overrides via `cli-compat`, or invoke MCP servers as deterministic peers alongside LLMs.

Spec: [`docs/specs/2026-05-05-v0.6.0-multi-provider-agents.md`](docs/specs/2026-05-05-v0.6.0-multi-provider-agents.md).
ADR: [`docs/decisions/2026-05-05-driver-abstraction.md`](docs/decisions/2026-05-05-driver-abstraction.md).

### Added

- **`AgentDriver` interface** in `cli/internal/agents/` — formalizes what a swarm participant is. Four concrete drivers: `cli`, `http`, `cli-compat`, `mcp`.
- **`http` driver** — direct OpenAI-compat + Anthropic-compat HTTP, no SDK dependencies. Anthropic ephemeral cache_control on system block (≥50% cost reduction on cache hit per spec AC). OpenAI prompt-cache awareness via `prompt_tokens_details.cached_tokens`. Both protocols extract real billed cost from response usage.
- **`cli-compat` driver** — wraps a base cli with env override + telemetry-kill defaults. Per-process one-shot warning makes the metadata-leak risk explicit (suppressible via `--no-telemetry-warning`).
- **`mcp` driver (stdio)** — JSON-RPC 2.0 handshake (initialize + notifications/initialized + tools/call) against MCP servers. Free at the API level (Result.CostUSD = 0). Reference stub server in `cli/internal/agents/testdata/stub_mcp_server/`. semgrep-mcp config example in `cli/internal/agents/mcp_examples.md` (doc-only).
- **`agents.yaml` two-layer config** — `~/.kaijutsu/agents.yaml` (global catalog) + `<repo>/.kaijutsu/agents.yaml` (project enabled list + overrides). Schema version 1 enforced. Loader hard-fails on unsupported versions with migration hint.
- **Personas as first-class swarm participants** — 7 built-ins shipped: 3 `default-*` with empty system_prompt (v0.5 cache compat) + 4 reference flavored (`paranoid-security-claude`, `pragmatic-codex`, `architecture-purist-gemini`, `brainstorm-creative-claude`). Auto-synthesis of `default-<provider>` for any enabled provider lacking one. Personas can declare tags (`security`, `architecture`, etc.) for skill `requires_persona_tags` gates.
- **`--personas <name>...` flag** on every swarm subcommand. Dispatches the named personas in parallel; legacy v0.5 path unchanged when flag absent.
- **`--estimate` dry-run** — per-persona cost projection table with input/output token estimates and total. Char-count tokenizer (±20% accuracy per spec D9). Stale rate-card warning at 90+ days. Aggregate budget warning when projected exceeds `--max-cost`.
- **`--no-telemetry-warning` flag** to suppress the cli-compat one-shot stderr warning.
- **`jutsu agent` subcommand group** — full surface:
  - `list [--personas]` — resolved providers + personas in deterministic table.
  - `doctor` — driver-aware health probes: cli `<cmd> --version`, cli-compat (binary + env), http (env + GET `/models`), mcp (Stage 6 stub).
  - `add <name> [--global] [--driver kind ...]` — catalog or non-catalog provider declaration.
  - `enable <name>` / `disable <name>` — toggle project enabled list (config preserved).
  - `remove <name> [--global] [--force]` — hard delete; `--global` honors `JUTSU_REPO_SCAN_ROOTS` env var (colon-separated paths, max-depth 4) for cross-repo reference detection; refuses without `--force` when references found.
  - `test <name>` — driver-aware probe: cli/cli-compat `--version`, http GET `/models` with 1-token completion fallback, mcp deferred.
  - `migrate [--prefer legacy|yaml|merge]` — converts deprecated `kaijutsu.json.agents` to `agents.yaml` `enabled:`. Idempotent on subsequent runs.
- **Vendored provider catalog**: claude/codex/gemini cli + deepseek/glm/kimi/ollama-local http with rate cards (date 2026-05-06). `jutsu agent add deepseek` writes the catalog default; users override via `--driver` flags or YAML edit.
- **`SaveGlobalConfig` + `SaveProjectConfig`** in `cli/internal/agents/load.go` — yaml.v3 round-trip with schema version preservation.
- **`SanitizeForLog`** in `cli/internal/agents/sanitize.go` — redacts known sensitive env values (`*_API_KEY`, `*_TOKEN`, `*_SECRET`, `*_PASSWORD`, plus bare provider key names) from log strings.

### Changed

- **`jutsu swarm <preset>`** — `--personas` is the recommended dispatch mode. Legacy v0.5 path (auto-detect native CLIs) remains the default when `--personas` is absent and produces byte-identical cache keys for backward compat.
- **`InvokeOpts`** gained `ExtraEnv map[string]string` — used by cli-compat driver to inject BASE_URL / API_KEY / telemetry-kill envs on the child process.
- **`Result`** struct gained `Driver DriverKind` + `Err string` + `CacheStatus CacheStatus` — drivers report cost + cache outcome; pipeline overlays real billed cost over the EstimateCostUSD char-count fallback when HTTP driver reports `Result.CostUSD > 0`.
- **`Provider`** type extended with cli-compat (`BaseCLI`, `Env`, `EnvKey`), MCP (`Transport`, `Command`, `Endpoint`, `Headers`, `HeadersLiteral`, `ToolName`, `TimeoutSec`), and `Cost *CostRates` (rate card) fields.
- **`kaijutsu.json.agents`** field is now deprecated in favor of `<repo>/.kaijutsu/agents.yaml` `enabled:` list. `agent migrate` automates the conversion. Removal scheduled for v0.7.

### Deprecated

- `kaijutsu.json.agents` field. Use `agents.yaml enabled:` (or run `jutsu agent migrate`).

### Notes

- **Backward compatibility:** users without `agents.yaml` files get byte-identical v0.5 cache keys. Existing `jutsu swarm pr-review` invocations require zero config changes.
- **Cost/leak/perf trade-offs:** the spec ADR explains why each driver kind exists. `cli-compat` is the path of least resistance for "use deepseek through claude CLI" but inherits Claude Code's harness behavior + leaks metadata via undocumented telemetry endpoints; `http` is the recommended default for non-native providers.
- **What's NOT shipped (deferred to v0.7+):** synthesizer `[deterministic]` tag rendering for MCP findings, MCP info-severity floor, MCP http transport, OpenAI prompt-cache padding to the 1024-token threshold, retry policy, streaming partial findings, quality fingerprinting (SQLite), persona registry, multi-stage swarm pipelines.

## [0.5.0] — 2026-05-05

`jutsu swarm` Phase 2: pluggable presets (pr-review, doc-review, brainstorm, refactor-plan, security-audit) on the same primitive. See `git log v0.4.1..v0.5.0` for the full set.

## [0.4.x] — earlier

See git history. Highlights: `jutsu swarm` primitive (Phase 1 — pr-review only), Sigstore enforcement, permission prompts, hooks as first-class artifacts, cascade-aware `jutsu remove`, `jutsu publish --auto`.
