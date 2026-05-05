# Phase 2 — `jutsu swarm` pluggable presets + the doc-review flywheel

Phase 1 hardcoded one preset (`pr-review`) into the binary. Phase 2
makes presets first-class, ships four new ones, and migrates existing
spec/plan/decision skills to use a shared `doc-review` preset for
their final review pass — turning `jutsu swarm` into the universal
QA gate for kaijutsu artifacts.

---

## Locked decisions (from Phase-2 brainstorm)

| Q | Choice |
|---|---|
| Output assembler | **Built-in Go**, not skill-shipped. Skill ships per-agent prompts + synthesizer prompt + debate prompt only. Avoids skill-template DSL scope creep + skill-shipped-XSS risk in PR comments. |
| Cache key for non-diff inputs | **SHA256(preset + normalized_input) truncated to 12 chars.** Same input → same cache → free `--replay` tuning. |
| refactor-plan multi-file input | **Single concatenated prompt.** Cross-file refactors require cross-file context. Cap total input size; surface "too big — narrow scope" error if over. |
| security-audit vs `pr-review --lens security` | **Standalone preset.** Different inputs (design docs, not just diffs), different severity taxonomy (CVSS-flavored), different output (threat model with attack vectors), different defaults (`--full` ON). |
| Auto-install missing presets | **No.** Hard error + hint: `jutsu install <preset>`. Auto-install = silent net + fs op; user must opt in to privileged skill. |
| `--post-comment` for non-PR presets | **pr-review-only.** brainstorm/refactor-plan/security-audit/doc-review print + cache. Generic `--output-to file\|gist\|issue\|slack` deferred to Phase 3. |
| doc-review section addressing | **Section path + paragraph index** (e.g. `section: "Acceptance Criteria"`, `paragraph: 3`) instead of file:line. Markdown-aware. |
| Severity taxonomy per preset | Each preset declares its own. pr-review/doc-review keep `blocker\|issue\|minor\|info`. security-audit uses `critical\|high\|medium\|low` (CVSS-aligned). brainstorm/refactor-plan use `recommended\|alternative\|risky`. |

---

## The flywheel insight

Three existing kaijutsu skills already have ad-hoc final-review passes:

- `spec-driven-development` — "fresh-eyes pass before unblocking implementation"
- `planning-and-task-breakdown` — "Final review pass via blunder-hunt + scope-check"
- `decide` — "multi-model critique on record"

Each invented its own mechanism. Phase 2 unifies them by introducing
**`doc-review`** as the canonical final-review preset. Each artifact-
producing skill drops its custom review mechanism and calls
`jutsu swarm doc-review <artifact.md>` instead.

Result: every spec, plan, decision, design doc, RFC produced by a
kaijutsu skill gets multi-agent review for free, on demand.

This is what makes Phase 2 not "more presets" but **swarm becomes the
universal QA gate for kaijutsu skills**.

---

## CLI surface

```
jutsu swarm pr-review        --pr <n> | --diff-from-branch <ref>
jutsu swarm doc-review       <path-to-markdown>
jutsu swarm brainstorm       "<prompt>"
jutsu swarm refactor-plan    <path>... --goal "<goal>"
jutsu swarm security-audit   --pr <n> | <path>...
```

Common flags carry across (`--quick|--full`, `--strict`, `--max-cost`,
`--synthesizer`, `--replay`, `--allow-secrets`, `-y`).

`--post-comment` only fires for `pr-review`. For other presets it
warns + skips.

---

## Preset registry

Built-in presets compiled into the binary. Per-repo + per-user skill-
loaded prompt overrides (already wired in Phase 1's
`LoadPresetWithSkillOverrides`). Phase 2 generalizes:

```go
type Preset struct {
    Name             string
    Description      string
    InputKind        InputKind  // diff | prompt | files | markdown
    PerAgent         map[AgentName]string
    Synthesizer      string
    Debate           string
    SeverityVocab    []Severity     // preset-specific severity strings
    Assembler        AssemblerFn    // built-in Go function, takes results+clusters → markdown
    CachePathPart    string         // "<name>-runs" segment under .kaijutsu/
    ConfigBaseName   string         // "<name>.yaml" under .kaijutsu/
}

type InputKind int
const (
    InputDiff InputKind = iota   // pr-review, security-audit (PR mode)
    InputFiles                   // refactor-plan, security-audit (file mode), doc-review
    InputPrompt                  // brainstorm
)

type AssemblerFn func(results []AgentResult, synthDraft string, clusters []FindingGroup) string
```

`PresetRegistry.Find(name)` walks: built-in → installed skill (with
`preset.yaml` declaring metadata) → error.

Skill-installed presets ship a `preset.yaml` alongside `prompts/` so
the orchestrator knows the input kind + severity vocabulary without
recompiling.

---

## Stages

### Stage 1 — Preset registry refactor

Move `pr-review` out of hardcoded `preset.go` into a registered
preset. Generalize the cache + config path lookups to use the
preset's `CachePathPart` / `ConfigBaseName` instead of hardcoded
`pr-review-runs/` and `pr-review.yaml`.

**Acceptance:**
- `jutsu swarm pr-review` continues to work end-to-end with no
  observable behavior change
- `cache.go` `CacheDir` takes preset name (or preset struct)
- `consent.go` `LoadConfig`/`SaveConsent` parametrized by preset
- A new `registry.go` exposes `BuiltinPresets()` + `Find(name)`
- Existing `swarm.go` swapped to use registry
- Tests cover registry lookup + cache-path generation per preset

**Out of scope this stage:** new presets, new input kinds.

---

### Stage 2 — Generic input handling

Each preset declares `InputKind`. `swarm.go` dispatches input fetch
per kind:

- `InputDiff` (pr-review, security-audit-PR-mode): existing logic via
  `FetchPRContext` / `FetchBranchDiff`
- `InputFiles` (refactor-plan, doc-review, security-audit-file-mode):
  read files into a single concatenated input. Cap total bytes; abort
  with helpful error if over.
- `InputPrompt` (brainstorm): take from positional CLI arg

The cache key generalization:
- `InputDiff`: head SHA (existing)
- `InputFiles`: SHA256 of concatenated file contents → 12 hex chars
- `InputPrompt`: SHA256 of normalized prompt → 12 hex chars

**Acceptance:**
- pr-review still works (regression check)
- A trivial test preset with `InputPrompt` runs via mocked agents
- Cache dir layout: `.kaijutsu/<preset>-runs/<key>/`

**Out of scope:** the actual new presets — those land in Stages 3–7.

---

### Stage 3 — `doc-review` preset (the flywheel) ⭐

Skill `skills/core/doc-review/` (rich layout) with prompts tuned for
markdown artifacts:

- claude: completeness — missing edge cases, undefined terms,
  ambiguity, scope creep, internal contradictions
- codex: implementability — "this section says X but no concrete
  way to do it", validation gaps, vague success criteria
- gemini: consistency — drift from project patterns, references that
  don't resolve, contradictions across sections

Severity vocab: `blocker | issue | minor | info` (same as pr-review).

Findings reference `section:"..."` + `paragraph:N` instead of file:line.
Custom assembler renders these in the disagreement table. Markdown
parser extracts headings + paragraph counts; agents are asked to
report findings by section+paragraph.

Output: same disagreement-table + synthesis structure as pr-review,
adapted for section addressing.

CLI: `jutsu swarm doc-review <path-to-markdown>` (positional arg).
Cache key: SHA256 of file contents.

**Acceptance:**
- `jutsu swarm doc-review IMPLEMENTATION_PLAN_PHASE2.md` produces
  a useful review of THIS plan
- Skill ships prompts/{claude,codex,gemini}.md +
  synthesizer.md + debate.md
- Cache + replay work
- references/section-addressing.md documents the addressing scheme

---

### Stage 4 — Migrate spec/plan/decide skills to call `doc-review`

Update SKILL.md workflows for:

- `spec-driven-development`: replace "fresh-eyes pass" section with
  "Run `jutsu swarm doc-review <spec.md>` before implementation.
  Iterate until disagreement table is clean."
- `planning-and-task-breakdown`: replace "Final review pass via
  blunder-hunt + scope-check" with `jutsu swarm doc-review`.
- `decide`: replace "multi-model critique on record" with
  `jutsu swarm doc-review` on the decision record.

Add `doc-review` to each skill's `deps.skills` so it auto-installs.

**Acceptance:**
- Each migrated skill bumped a minor version (1.x → 1.x+1)
- Each skill's SKILL.md has the new workflow
- `jutsu install spec-driven-development` pulls doc-review transitively
- README highlights "all artifact-producing skills now share doc-review"

---

### Stage 5 — `brainstorm` preset

Skill `skills/core/brainstorm/`. Free-form prompt input. Per-agent
lenses:

- claude: long-horizon framing — what does the ideal end-state look
  like? What's the ambition?
- codex: concrete code-pattern grounding — what existing patterns
  in the codebase or community would solve this?
- gemini: cross-domain analogy — what does this look like in
  related domains? What's been tried?

Severity vocab: `recommended | alternative | risky | speculative`.
Output assembler: ranked-list markdown with tradeoff per option.

CLI: `jutsu swarm brainstorm "how do I rate-limit my API?"`.

**Acceptance:**
- Skill installable
- Output is a usable starting point for `decide`'s decision-journal flow

---

### Stage 6 — `refactor-plan` preset

Skill `skills/core/refactor-plan/`. Files + goal input. Per-agent
lenses:

- claude: architectural decomposition — what's the right new shape?
- codex: stepwise risk — what order minimizes regression risk?
- gemini: pattern consistency — does the proposed shape match
  existing patterns in the repo?

Severity vocab: `recommended | alternative | risky`. Output assembler:
ordered step list with risk column + estimated effort.

CLI: `jutsu swarm refactor-plan path1.go path2.go --goal "extract HTTP handler into service"`.

**Acceptance:**
- Skill installable
- Output is consumable by `planning-and-task-breakdown` as input

---

### Stage 7 — `security-audit` preset

Skill `skills/core/security-audit/`. Diff OR files input. Per-agent
lenses:

- claude: auth + data flow — boundaries, identity, trust transitions
- codex: injection + privilege escalation — concrete attack vectors
- gemini: dep + supply-chain — third-party trust, version drift

Severity vocab: `critical | high | medium | low | informational`
(CVSS-aligned). Output assembler: threat model + attack vector list +
prioritized recommendations.

`--full` defaults ON for this preset (security is high-stakes).

CLI: `jutsu swarm security-audit --pr 42` or `jutsu swarm security-audit cmd/server/`.

**Acceptance:**
- Skill installable
- Output references CVSS taxonomy where applicable

---

## Build sequence

Stage 1 → 2 (foundation) → 3 → 4 (the flywheel) → 5 → 6 → 7 (the
preset catalog).

Stages 1–4 are the high-leverage chunk and could ship as v0.5.0-rc1.
Stages 5–7 round out the catalog and ship as v0.5.0.

After Stage 4: write a v0.5.0 announcement post highlighting the
"swarm-as-QA-gate" pattern. Tag and release.

---

## Risks + mitigations

| Risk | Mitigation |
|---|---|
| Preset registry refactor breaks pr-review (Phase 1 regression) | Stage 1 acceptance criterion: pr-review behavior unchanged. Existing tests + smoke runs catch drift. |
| doc-review prompts too generic, find too many false positives | Per-repo prompt overrides (already supported via Stage 1 of Phase 1) let teams calibrate. References doc covers tuning. |
| Skill auto-install via deps loads doc-review on first install of spec/plan/decide; consent prompt fires unexpectedly | Document in each migrated skill's SKILL.md that doc-review is pulled transitively + the consent gate fires once per repo. |
| Token-budget overruns on InputFiles concatenation | Stage 2 caps total bytes; abort with "narrow scope or split files" message. |
| Agents misinterpret `section:` addressing scheme for markdown | Stage 3 prompts include explicit examples + a small reference doc. |
| security-audit severity vocab confusion with pr-review's | Document each preset's vocabulary clearly. Disagreement table renders the preset's severity strings verbatim. |
| Phase 2 spec itself contains errors only doc-review would catch | Bootstrap: review THIS plan with `jutsu swarm pr-review --diff-from-branch origin/main` before starting Stage 1. Code-tuned lens gives ~partial signal but better than nothing. |

---

## Out of scope (deferred)

- Generic `--output-to file|gist|issue|slack` flag (Phase 3+)
- Skill-shipped Go templates for output assembly (Phase 3+; security
  + sandboxing concerns)
- `jutsu swarm brainstorm` chained into `jutsu swarm refactor-plan`
  (chain-of-presets — interesting but premature)
- Live-coding mode / continuous review (Phase 3+)
- PR-of-the-PR auto-fix (Phase 3+)

---

## Open questions

1. **Should the existing review-pass logic in spec/plan/decide skills
   be DELETED or kept as fallback when doc-review isn't installed?**
   Lean toward fallback: graceful degradation if user has the skill
   but not the preset.

2. **doc-review on a draft markdown that imports/links to other
   files — should those linked files be loaded into context?**
   Phase 2 says no (single-file input). Phase 3 could add link
   resolution.

3. **brainstorm + decide chain — should `decide` invoke brainstorm
   automatically?** Probably no; user chooses. But document the chain
   pattern in decide's SKILL.md.

4. **Should `jutsu swarm` print preset-discovery output on `--help`?**
   `jutsu swarm --help` listing installed presets + their input kinds
   is high-value UX. Add to Stage 1.

5. **Per-preset `--max-cost` defaults?** security-audit + refactor-plan
   are higher-stakes; might warrant a higher default budget. Stage 7
   will tune.

---

## Bootstrap action items

Before Stage 1:
1. Commit this plan
2. Run `jutsu swarm pr-review --diff-from-branch origin/main` against
   it to surface gaps before building
3. Address findings
4. Open PR (or merge to main directly given solo-maintainer model)
5. Begin Stage 1
