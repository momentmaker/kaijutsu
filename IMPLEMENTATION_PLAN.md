# IMPLEMENTATION_PLAN.md — v0.14.0

Spec: `docs/specs/2026-05-09-v0.14.0-persona-browse-and-preset-usage.md`
Branch: `v0.14.0-persona-browse`

Three stages. Stages 1+2 are small Go ergonomics; Stage 3 is the dream-locked landing page rebuild.

## Stage 1: `jutsu agent persona browse`

**Goal**: read-only persona discovery cobra subcommand. Reads existing agents.yaml + built-ins; outputs aligned table (TTY) / JSON (pipe) / paste-ready yaml (`--yaml`); supports `--tag` / `--provider` / `--source` filters.

**Success Criteria**:
- New `cli/internal/agents/registry_query.go` exposes `BrowsePersonas(resolved *Resolved, builtins map[string]*Persona, filters BrowseFilters) []BrowseRow`. Pure function over resolved registry; cli pkg renders.
- `BrowseRow` includes Name, Provider, Tags, Source, SystemPrompt (full text; truncation is renderer's job).
- `BrowseFilters` exposes Tag / Provider / Source string fields; empty = no filter; combinable.
- New `cli/internal/cli/agent_persona_browse.go` cobra subcommand attached under existing `jutsu agent` cmd group. RunE: load resolved agents config + built-ins, apply filters, render output. Auto-flips JSON when stdout is a pipe (existing agent-first pattern).
- `--json` forces JSON regardless of pipe detection.
- `--yaml` emits paste-into-`agents.yaml`-ready yaml block.
- **`--json` + `--yaml` together = error** ("--json and --yaml are mutually exclusive; pick one"). Cobra MarkFlagsMutuallyExclusive enforces.
- **Multi-value filter flags** (`--tag` / `--provider` / `--source`): single-value only in v0.14. Multiple `--tag` invocations = last wins (cobra StringVar default). Help text + spec doc explicitly note this constraint. v0.14.x can promote to StringSliceVar with OR semantics if user demand surfaces. **Concrete format** (per spec Decision #1): top-level `personas:` map, each persona keyed by name, sub-keys mirror `~/.kaijutsu/agents.yaml` schema (`provider:`, `system_prompt:`, `tags:`, `model:` if set). Test fixture pins this layout.
- TTY table truncates `system_prompt` to ≤ 60 chars + ellipsis (per spec doc-review). JSON / yaml output preserves full text.
- Multi-config persona collision (project + home) — project wins per existing `agents.Resolve` semantics; browse surfaces project version with `source=user:project` (spec Decision #3).
- Built-in shadow (user agents.yaml defines a name matching a built-in) — user wins per existing `agents.Resolve` overlay; browse surfaces user version with `source=user:project` or `user:home`.
- All existing tests stay green.

**Tests**:
- `cli/internal/agents/registry_query_test.go`:
  - `TestBrowsePersonas_NoFilterReturnsAll`
  - `TestBrowsePersonas_TagFilter`
  - `TestBrowsePersonas_ProviderFilter`
  - `TestBrowsePersonas_SourceFilter`
  - `TestBrowsePersonas_CombinedFilters`
  - `TestBrowsePersonas_StableSortByName`
  - `TestBrowsePersonas_ProjectShadowsHome` (multi-config collision: project wins; only project version surfaces)
  - `TestBrowsePersonas_UserShadowsBuiltin` (user agents.yaml redefines built-in name; user version surfaces with user source)
- `cli/internal/cli/agent_persona_browse_test.go`:
  - `TestAgentPersonaBrowse_TTYRendersTable`
  - `TestAgentPersonaBrowse_TTYTruncatesSystemPrompt` (≤ 60 chars + ellipsis)
  - `TestAgentPersonaBrowse_PipeRendersJSON`
  - `TestAgentPersonaBrowse_JSONHasFullSystemPrompt`
  - `TestAgentPersonaBrowse_YamlFlagRendersPasteReadyFormat` (top-level `personas:` map; verifies layout)
  - `TestAgentPersonaBrowse_YamlRoundTripsThroughLoadGlobalConfig` (write `--yaml` output to a tmp file in the global config path, run `agents.LoadGlobalConfig`, assert the loaded persona matches the original by name + provider + system_prompt + tags + model)
  - `TestAgentPersonaBrowse_JSONAndYamlMutuallyExclusive` (passing both → cobra error citing the exclusion)
  - `TestAgentPersonaBrowse_FiltersHonored`

**Status**: Not Started

## Stage 2: `jutsu finding stats` + housekeeping

**Goal**: preset usage tracking via findings.db query. Plus marker bump + CHANGELOG/ROADMAP/README first-pass (final-pass after Stage 3 lands).

**Success Criteria**:
- New `cli/internal/findings/stats.go` exposes `Stats(store *Store, opts StatsOpts) ([]StatsRow, error)`. Reads existing `findings` table (no migration); groups by `preset` / `persona` / `provider` per `opts.By`; filters by `opts.Since` window. **NO source-resolution logic in findings package** — keeps findings/ pure data + avoids the swarm/registry import-cycle / abstraction-leak concern flagged by plan doc-review. `StatsRow` returns the raw `Group` (preset/persona/provider name) + `Count` only.
- `StatsOpts.Since` accepts `<N>d` / `<N>w` / `<N>mo` / `<N>y` shorthand via custom parser; sub-day grain (`30s` / `5m` / `2h`) falls through to `time.ParseDuration`. **Zero value (`0` / `0s` / `0d`)** = error ("--since must be positive; pass empty string for all-time"). **Empty string** = all-time. **Negative** = error. **Cobra flag default** = `90d` (so bare `jutsu finding stats` shows 90-day window per spec); explicit `--since ""` overrides to all-time.
- `StatsOpts.By` enum: `preset` (default) / `persona` / `provider`. Group-by reads `findings.preset` / `findings.persona` / `findings.provider` columns directly (denormalized at recorder time per `0001_init.sql`). **All three columns coalesce NULL → `(unknown)`** via SQL `COALESCE` (preset NULL is unlikely per recorder's `meta.Preset` required-arg contract, but legacy rows could exist; defensive).
- **Source resolution lives in `cli/internal/cli/finding_stats.go`**, NOT in findings package. CLI handler enriches each `StatsRow.Group` (when `opts.By == "preset"`) by querying `swarm.DefaultRegistry()`: `UserSource(name) == "" && Find(name) succeeds` → `built-in`; `UserSource(name) != ""` → `user`; not in registry → `unknown`. Source filter applies AFTER enrichment. Group-by persona/provider rows have no source (set to empty string).
- New `cli/internal/cli/finding_stats.go` cobra subcommand attached under existing `jutsu finding` group.
- TTY = aligned table; pipe = JSON. `--json` forces JSON.
- Empty findings.db → `(no recorded runs in window)` message + exit 0 (NOT error).
- `init_agents_fragment` marker `0.13.0` → `0.14.0`. `init_agents_fragment_test.go` assertion updated.

**Tests**:
- `cli/internal/findings/stats_test.go`:
  - `TestStats_CountsByPreset`
  - `TestStats_HonorsSinceWindow`
  - `TestStats_GroupByPersona`
  - `TestStats_GroupByProvider`
  - `TestStats_NullPersonaCoalescesToUnknown`
  - `TestStats_SourceFilter_BuiltinVsUser`
  - `TestStats_SourceUnknownForUnregisteredPreset`
  - `TestStats_EmptyDB_NoError`
  - `TestParseSinceDuration_DaysWeeksMonthsYears`
  - `TestParseSinceDuration_FallsThroughToStdParser`
  - `TestParseSinceDuration_RejectsNegative`
- `cli/internal/cli/finding_stats_test.go`:
  - `TestFindingStats_TTYTable`
  - `TestFindingStats_PipeJSON`
  - `TestFindingStats_HelpListsFlags`
  - `TestFindingStats_EmptyDBNoErrorExit`
- `cli/internal/cli/init_agents_fragment_test.go` — `version=0.14.0` assertion.

**Status**: Not Started

## Stage 3: Landing page rebuild

**Goal**: dream-locked landing page lands at `kaijutsu.dev` (GitHub Pages). Three sub-phases: 3a synthesis (already done in spec), 3b content spec, 3c implementation.

**Success Criteria**:
- **3a synthesis**: ADR at `docs/decisions/2026-05-09-landing-page-direction.md` distills the spec's locked-direction block into a one-page reference. Captures: locked one-sentence pitch, banned-words list, page-section order, voice rules, mascot constraints, success metric. Created during Stage 3 (NOT pre-existing); spec block is the source of truth.
- **3b content spec** at `docs/specs/2026-05-09-landing-page-content.md`: concrete copy + visual specifics derived from 3a. Run `jutsu swarm doc-review --personas claim-auditor-deepseek,architecture-purist-gemini` on it before 3c. **Apply doc-review findings**: address all `blocker` + `issue` severity findings; defensible-skip `minor` + `info` with one-line rationale per skip in a "doc-review triage" section appended to the content spec. Same standard as v0.10+ doc-review handling.
- **3c implementation**:
  - Extend `cli/cmd/sitegen/main.go` to add a hero-block template hook BEFORE the catalog table. Catalog continues to render from `docs/skills.json` unchanged.
  - Hero, single-agent failure-mode demo (real v0.13 disagreement-table screenshot or embed), quickstart, cross-vendor portability section, cost-per-bug-found, trust model, catalog, contributing, footer — in order.
  - Mascot at ≤ 120px hero placement.
  - Charcoal/paper/ink primary palette; #3DDC97 as accent only.
  - Mobile responsive ≥ 375px width (iPhone SE baseline).
  - Lighthouse Performance + Accessibility ≥ 90 (manual run; pre-merge gate).
  - Drop "pre-alpha" framing from any user-facing copy on the landing page (CLAUDE.md / AGENTS.md keep it for contributors).
  - NO "reviewed by N LLMs" badge; replace with concrete artifact embed (link to a v0.13 swarm pr-review run with kaijutsu-pr-review marker).
  - Cynical-read defense paragraph addressing "wrapper" charge.
  - Site at `kaijutsu.dev` reflects new page within ~minutes of merge to main (existing GitHub Pages workflow).
  - `kaijutsu.org` 301 → `kaijutsu.dev` (existing redirect; verify intact).

**Files**:
- `docs/decisions/2026-05-09-landing-page-direction.md` (3a)
- `docs/specs/2026-05-09-landing-page-content.md` (3b)
- `docs/index.html` (3c — rewritten; preserves catalog-fetch wiring)
- `docs/landing.css` (3c — extracted from inline `<style>` if it grows past ~200 lines; otherwise inline)
- `docs/og-image.png` (3c — social-share open-graph image, 1200×630)
- `cli/cmd/sitegen/main.go` (3c — extended with hero-block template hook)

Plus housekeeping pulled into Stage 3 final commit:
- `CHANGELOG.md` `[0.14.0]` entry covering all 3 stages + landing-page direction summary.
- `ROADMAP.md` — flip persona-browse + preset-stats rows [ ] → [x]; add v0.14.x landing-page-iteration note if any.
- `README.md` — short v0.14 paragraph pointing at `kaijutsu.dev` for the new pitch.

**Tests**: docs/HTML changes — no Go tests added. Stage 3 final pre-merge gate: Lighthouse score check + visual smoke at 375px / 1280px viewports.

**Status**: Not Started

---

## End-state verification (after all 3 stages)

```bash
brew upgrade momentmaker/tap/jutsu
jutsu --version                                              # 0.14.0

# Stage 1
jutsu agent persona browse                                   # table
jutsu agent persona browse --tag security                    # filtered
jutsu agent persona browse --json | jq .                     # JSON
jutsu agent persona browse --yaml > /tmp/p.yaml              # paste-ready

# Stage 2
jutsu finding stats                                          # 90-day preset count
jutsu finding stats --since 7d --by persona                  # weekly persona
jutsu finding stats --source user                            # user-defined only
jutsu finding stats --json | jq '.[]'                        # machine-readable

# Marker
jutsu init && grep version=0.14.0 AGENTS.md

# CI
cd cli && go test ./... -count=1                             # every package green

# Stage 3 — landing page
ls docs/decisions/2026-05-09-landing-page-direction.md       # ADR
ls docs/specs/2026-05-09-landing-page-content.md             # content spec
curl -fsSL https://kaijutsu.dev | grep -E "<title>|tagline"  # new page served
# Lighthouse: open in Chrome, run audit, confirm Perf + A11y ≥ 90
```

---

## Squash + tag

After Stage 3 + final swarm pr-review pass:

```bash
git checkout main
git merge --no-ff v0.14.0-persona-browse -m "Merge v0.14.0-persona-browse: persona browse + preset stats + landing page"
git tag -a v0.14.0 -m "v0.14.0 — persona browse + preset usage tracking + landing page rebuild"
git push origin main
git push origin v0.14.0
brew upgrade momentmaker/tap/jutsu
```

---

## Risks (recap from spec)

| Risk | Mitigation |
|---|---|
| Stage 3 bottlenecks Stages 1+2 ship | Stages 1+2 land as separate commits FIRST; Stage 3 lands as separate commits. If Stage 3 stalls 7+ days post-Stage-2 squash, split: tag v0.14.0 with Stages 1+2; ship Stage 3 as v0.14.1. |
| Landing page CSS regresses on mobile | iPhone SE width (375px) test before merge; Lighthouse Accessibility ≥ 90 catches readability/contrast. |
| Sitegen hero-block hook breaks catalog drift CI | Catalog rendering path unchanged; hero block is additive template hook BEFORE the catalog. lint-skills.yml drift check still validates skills.json. |
| Dream lens-blindspot warnings → over-correction in implementation | Synthesis ADR captures the warnings inline + tells the implementer NOT to over-correct (explicit guardrails: some warmth allowed, mascot allowed). |
| Cynical-read defense paragraph reads as defensive | Voice = factual, not defensive. Sentence shape: "what kaijutsu does that's not just a wrapper" — affirmative, not "but actually we…" |
| `--since` duration parser drifts from `time.ParseDuration` semantics | Custom parser only handles `d` / `w` / `mo` / `y` suffixes; everything else falls through to `time.ParseDuration` for source-of-truth. Tests pin both paths. |
| Source resolution misclassifies historical user presets as future built-ins | Currently zero retired built-ins; if v1.0+ retires one, those rows correctly classify as `unknown`. v0.14.x can add a `source` column + migration if granular tracking needed. |
