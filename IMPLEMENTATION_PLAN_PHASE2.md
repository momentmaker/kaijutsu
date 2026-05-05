# Phase 2 — `jutsu swarm` pluggable presets + the doc-review flywheel

Phase 1 hardcoded one preset (`pr-review`) into the binary. Phase 2
makes presets first-class, ships four new ones (doc-review,
brainstorm, refactor-plan, security-audit), and migrates the three
artifact-producing skills (spec-driven-development,
planning-and-task-breakdown, decide) to use the shared `doc-review`
preset for their final review pass.

**flywheel** (used throughout this doc): the pattern where every
artifact-producing skill in kaijutsu calls `jutsu swarm doc-review
<artifact>` at the end of its workflow, getting multi-agent QA for
free without re-inventing review mechanisms. See "The flywheel
insight" section below.

---

## Locked decisions (from Phase-2 brainstorm)

| Q | Choice |
|---|---|
| Output assembler | **Built-in Go**, not skill-shipped. Skill ships per-agent prompts + synthesizer prompt + debate prompt only. Avoids skill-template DSL scope creep + skill-shipped-XSS risk in PR comments. |
| Cache key for non-diff inputs | **First 12 lowercase hex chars of SHA256(preset-name + NUL + canonicalized_input)** — applies to both `InputFiles` and `InputPrompt` and to all non-diff presets (doc-review, brainstorm, refactor-plan, file-mode security-audit). Canonicalization rules: (a) `InputFiles` — sort paths lexically before concat, prefix each section with `--- <path>\n` so file order doesn't change the hash; (b) `InputPrompt` — `strings.TrimSpace` + collapse runs of internal whitespace to single space + Unicode NFC normalize. Truncation to 12 chars (48 bits) is fine for single-repo cache history; on collision the second run overwrites the first (last-write-wins, no error). Each preset's stage acceptance section MUST reference these rules verbatim, NOT re-state simpler ones. |
| Input byte caps per kind | `InputDiff`: 200 KB (matches gemini -p stdin fallback threshold + leaves room for prompt overhead). `InputFiles`: 200 KB total across all paths. `InputPrompt`: 8 KB. Over-cap aborts with "narrow scope, split files, or shorten prompt" hint. Phase 2 ships fixed defaults; per-preset override via skill metadata is deferred to Phase 3 (would require a `MaxBytes int` field on the Preset struct + plumbing through ResolveInput). Tracked as Open Question 5. |
| refactor-plan multi-file input | **Single concatenated prompt** (with sorted paths per the canonicalization rule above). Cross-file refactors require cross-file context. Cap from the row above applies. |
| security-audit vs `pr-review --lens security` | **Standalone preset.** Different inputs (design docs, not just diffs), different severity taxonomy (CVSS-flavored), different output (threat model with attack vectors), different defaults (`--full` ON). |
| Auto-install missing presets | **No silent install at swarm time.** `jutsu swarm <preset>` with the preset uninstalled hard-errors with `jutsu install <preset>` hint. Note: this is distinct from `deps.skills` transitive install at `jutsu install` time, which IS user-initiated (the user typed `jutsu install spec-driven-development` and accepts the dep tree). The two consent gates are separate: install-time perms prompt (Phase 1) vs swarm-time multi-model consent (Phase 1). Both fire once per repo. Stage 4 documents this in each migrated skill's SKILL.md so users aren't surprised. |
| `--post-comment` for non-PR presets | **pr-review-only flag.** Implementation: subcommands for non-PR presets do NOT register the `--post-comment` flag at all (Phase-2 wired). If a user has a stale alias that passes it, cobra rejects with "unknown flag". Output for non-PR presets is print to stdout + cache write — that's the "print + cache" model. Generic `--output-to file\|gist\|issue\|slack` deferred to Phase 3. |
| doc-review section addressing | **Section path + paragraph index** (e.g. `section: "Acceptance Criteria"`, `paragraph: 3`) instead of file:line. Markdown-aware. |
| Severity taxonomy per preset | Each preset declares its own. pr-review/doc-review keep `blocker\|issue\|minor\|info`. security-audit uses `critical\|high\|medium\|low\|informational` (CVSS-aligned). brainstorm/refactor-plan use `recommended\|alternative\|risky\|speculative` (4 levels — `speculative` covers wild ideas worth recording but not yet defensible). |

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
    InputKind        InputKind  // diff | files | prompt
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

`PresetRegistry.Find(name)` walks: built-in registry → skill-loaded
prompt overrides on top → error if name unregistered. Built-in always
wins on the metadata fields (`InputKind`, `SeverityVocab`,
`CachePathPart`, `ConfigBaseName`); the skill can ONLY override the
prompt strings (`PerAgent[*]`, `Synthesizer`, `Debate`). This
prevents a malicious skill from altering input-shape or severity
semantics by shipping a malformed `preset.yaml`.

Phase-2 scope: `preset.yaml` is RESERVED — no schema defined yet.
Built-in presets register themselves via Go init(); skill-shipped
presets are deferred to Phase 3 once a schema + signing story is
specified. Until then, every preset is built-in.

---

## Stages

### Stage 0 — Foundation already shipped (no work) ✅

Listed for traceability — these Phase-1 items are prerequisites that
Stages 1–7 build on. Nothing to deliver here; verify before starting
Stage 1.

- **gemini tool-call abort fix** — `--approval-mode plan` flag wired
  in `cli/internal/swarm/agent.go` (Phase-1 polish, commit 5ef038e)
  + the no-tools instruction in the gemini lens prompt. Without
  these, gemini in `-p` mode tries to invoke its file-read tool,
  blocks waiting for approval on stdin, hits the 180s timeout. The
  fix lets gemini participate in every Stage 3–7 preset that uses
  InputFiles or InputDiff.
- **`--grant-consent` flag** — wired in commit 9a9eb9f. Without it,
  Stage 4's migrated skills (running headless from inside an agent
  CLI session) hang on the first-run consent prompt. Each migrated
  skill's SKILL.md (Stage 4) tells users to pre-grant consent.
- **`--approval-mode plan` is a belt-and-suspenders for gemini
  specifically**; if a future Stage 5–7 preset uses a different
  agent invocation pattern (e.g. routing prompts through a custom
  binary), re-evaluate this assumption.

---

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
- `Find(name)` precedence pinned in code comment + test: built-in
  registry is the source of truth for metadata; missing-name
  produces a clear error listing available presets
- `jutsu swarm --help` lists registered presets via
  `BuiltinPresets()` (resolves Open Question 4)

**Out of scope this stage:** new presets, new input kinds, skill-
shipped presets (those need a `preset.yaml` schema + signing — Phase 3).

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

The cache key generalization (per the canonicalization rules pinned
in the locked-decisions row above — Stage 2 implementation MUST
follow those rules verbatim, not re-invent simpler ones):
- `InputDiff`: head SHA (existing, unchanged)
- `InputFiles`: SHA256(`<preset-name>` + sorted-paths-with-`--- <path>\n`-prefix-per-section) → first 12 lowercase-hex chars
- `InputPrompt`: SHA256(`<preset-name>` + TrimSpace + collapsed-whitespace + NFC) → first 12 lowercase-hex chars

The `<preset-name>` salt prevents cross-preset cache collisions when
the same input bytes feed different presets (e.g., the same diff
reviewed by both pr-review and security-audit).

**Acceptance:**
- pr-review still works (regression check)
- A trivial test preset with `InputPrompt` runs via mocked agents
- Cache dir layout: `.kaijutsu/<preset>-runs/<key>/`
- Tests verify canonicalization: `refactor-plan a.go b.go` and
  `refactor-plan b.go a.go` produce the same cache key; `brainstorm
  "  hi   world  "` and `brainstorm "hi world"` produce the same
  cache key.
- Tests verify byte-cap enforcement: an `InputFiles` invocation
  with total bytes > 200 KB returns the "narrow scope" abort error
  before any agent is invoked; same for `InputPrompt` > 8 KB.

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

Findings reference markdown file + line range using the existing
`file` + `line_range` schema fields. The skill's per-agent prompts
ask the model to MENTION the section name in its `reasoning` text
when relevant ("In **Acceptance Criteria** at line 42, ..."), but
the structured fields stay file:line for disagreement-table parity
with pr-review. A future Phase-3 enhancement could add explicit
`section`/`paragraph` fields with a markdown-aware assembler;
deferred to keep Stage 3 scope tight.

Output: same disagreement-table + synthesis structure as pr-review.

CLI: `jutsu swarm doc-review <path-to-markdown>` (one or more
positional args; multi-file uses Stage 2's InputFiles canonicalization).
Cache key uses the canonicalization rules from the locked-decisions
row above (NOT a re-stated simpler rule).

**Acceptance:**
- `jutsu swarm doc-review IMPLEMENTATION_PLAN_PHASE2.md` produces
  a useful review of THIS plan
- Skill ships prompts/{claude,codex,gemini}.md +
  synthesizer.md + debate.md
- Cache + replay work
- Per-repo prompt overrides honored (re-uses Phase-1 loader)

**Stage 3 exit criterion (closes the bootstrap loop):**
Re-review THIS plan with `jutsu swarm doc-review IMPLEMENTATION_PLAN_PHASE2.md`
once the preset is shipping. Pass condition: doc-review produces
≥1 finding NOT raised by the bootstrap pr-review pass — concrete
proof that the prose-tuned lens adds signal the code-tuned lens
missed. Address all issue-level findings (consensus or not) and
the consensus minor findings before declaring Stage 3 done.
Contested-minor and info-level findings get triaged as v0.5.x
backlog if they don't block flywheel adoption.

---

### Stage 4 — Migrate spec/plan/decide skills to call `doc-review`

Resolution of Open Q1 (delete vs keep fallback): **REPLACE, not
fallback.** The migrated skills' SKILL.md sections that previously
described ad-hoc review mechanisms get replaced with a single line
pointing at doc-review. Reasons: (a) the ad-hoc mechanisms were
unmaintained (each invented different terminology); (b) keeping
them as fallback grows the skill complexity for an edge case where
doc-review isn't installed; (c) doc-review is a transitive dep, so
"not installed" only happens if the user explicitly removed it,
which already breaks the parent skill — fallback wouldn't recover
that. Users running the parent skill with doc-review missing get
a clear error pointing at `jutsu install doc-review`.

Update SKILL.md workflows for:

- `spec-driven-development`: replace "fresh-eyes pass" section with
  "Run `jutsu swarm doc-review <spec.md>` before implementation.
  Iterate until disagreement table shows only consensus minors or
  zero issue-level findings."
- `planning-and-task-breakdown`: replace "Final review pass via
  blunder-hunt + scope-check" with `jutsu swarm doc-review`.
- `decide`: replace "multi-model critique on record" with
  `jutsu swarm doc-review` on the decision record.

Add `doc-review` to each skill's `deps.skills` so it auto-installs.

**Acceptance:**
- Each migrated skill bumped a minor version (1.x → 1.x+1)
- Each skill's SKILL.md has the new workflow + drops the old
  ad-hoc review section
- `jutsu install spec-driven-development` pulls doc-review transitively
  (lockfile shows `installedAs: dep:spec-driven-development`)
- README highlights "all artifact-producing skills now share doc-review"
- Each migrated skill's SKILL.md notes that running the parent skill
  with doc-review uninstalled produces a clear error pointing at
  `jutsu install doc-review` (not silent fallback)

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

Severity vocab: `recommended | alternative | risky | speculative`
(matches the locked-decisions row above; `speculative` covers
ambitious approaches worth recording but not yet defensible).
Output assembler: ordered step list with risk column + estimated
effort.

CLI: `jutsu swarm refactor-plan path1.go path2.go --goal "extract HTTP handler into service"`.
The `--goal` flag is REQUIRED (cobra `MarkFlagRequired`); missing-
goal invocation hard-errors with a usage hint.

Goal text counts against the InputFiles 200 KB cap (it's prepended
to the file body before canonicalization). Practical effect: the
goal gets ~few hundred bytes; cap impact negligible. Tests cover
both: empty `--goal` rejected; goal + files near 200 KB triggers
the byte-cap abort with "narrow scope" hint.

**Acceptance:**
- Skill installable
- `--goal` flag required; tests cover the missing-goal error path
- Goal + files combined respects the 200 KB cap; over-cap test
  asserts the abort fires before any agent invocation
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
Dual input mode: `--pr <n>` / `--diff-from-branch <ref>` selects
InputDiff path (reuses pr-review's resolver); positional file/dir
paths select InputFiles path. Mutual exclusivity enforced — passing
both errors with a "pick one" hint.

Cache-key salting: the preset name (`security-audit`) is included
in the SHA256 input per the locked-decisions canonicalization rule,
so a security-audit run on PR #42 doesn't collide with a pr-review
run on the same SHA. Test asserts distinct cache dirs:
`.kaijutsu/security-audit-runs/<key>/` vs
`.kaijutsu/pr-review-runs/<key>/`.

**Acceptance:**
- Skill installable
- Output references CVSS taxonomy where applicable
- Both input modes (PR mode + files mode) tested end-to-end with
  mocked agents
- Mutual-exclusivity flag check tested
- Cache-collision-prevention test asserts security-audit + pr-review
  on the same diff produce distinct cache dirs
- `--full` mode default verified (a `--quick` invocation must be
  explicit to override)

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
| Skill-shipped per-agent prompts expand the prompt-injection surface — a malicious upstream skill could ship prompts that turn the synthesizer into a confused-deputy (e.g. embed instructions in its agent prompts that the synthesizer later reads as data) | Three layers of defense, wired during the relevant stages (NOT pre-shipped — calling these out explicitly so the threat model isn't claiming infrastructure that doesn't exist yet): (a) **Stages 3–7 acceptance includes**: each new core skill (doc-review, brainstorm, refactor-plan, security-audit) ships with `trust.expected-signer: kaijutsu-core@github` in its skill.yaml AND is added to sign-core.yml's signing manifest. After release tagging, `jutsu install <skill>` hard-fails on signature mismatch (sigstore enforcement, already wired for v0.3.1+ core skills). (b) **Phase-1 INPUT-INTEGRITY block already extended in shared headers** to instruct the model to treat user-content as data; future tightening (Phase 3) will extend that wording to cover skill-loaded prompts at runtime as well. (c) **Stages 3–7 documentation deliverable**: `references/prompt-design.md` per skill includes a skill-prompt-audit recipe for users who install community skills that override prompts. |
| `gemini -p` invokes file-read tools mid-prompt and aborts on missing paths | **Mitigated in Stage 0** (Phase-1 polish). `cli/internal/swarm/agent.go` runs gemini with `--approval-mode plan` (read-only sandbox, auto-approves reads, blocks writes) + the gemini lens prompt explicitly instructs "no tools, reason from input only". Future regression risk if a new preset routes through a different gemini invocation; tests should assert `--approval-mode plan` survives the build. |

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
   **RESOLVED → DELETE.** Stage 4 replaces (not supplements). Reasons
   in Stage 4 body. doc-review is a transitive dep so "missing" only
   happens after explicit `jutsu remove`, which already breaks the
   parent skill.

2. **doc-review on a draft markdown that imports/links to other
   files — should those linked files be loaded into context?**
   **DEFERRED to Phase 3.** Phase 2 = single-file (or multi-file
   concatenated via Stage 2's InputFiles). Link resolution adds a
   crawl mechanism with its own risks (cycles, off-tree paths).

3. **brainstorm + decide chain — should `decide` invoke brainstorm
   automatically?** **DEFERRED.** User chooses. Document the chain
   pattern in decide's SKILL.md (Stage 4) so users know the option.

4. **Should `jutsu swarm` print preset-discovery output on `--help`?**
   **RESOLVED → YES, in Stage 1 acceptance.** `jutsu swarm --help`
   lists registered presets via `BuiltinPresets()`.

5. **Per-preset input byte caps + `--max-cost` defaults — tunable
   via skill metadata?** **DEFERRED to Phase 3.** Phase 2 ships
   fixed defaults (200 KB / 200 KB / 8 KB; $1.00 max-cost).
   security-audit + refactor-plan are higher-stakes and may warrant
   higher caps once observed in the wild. Stage 7 will recommend
   adjustments based on real usage but won't ship the tunability
   plumbing — that's a Phase-3 Preset-struct addition (`MaxBytes`,
   `MaxCostUSD`).

---

## Bootstrap action items

Before Stage 1:
1. Commit this plan
2. Run `jutsu swarm pr-review --diff-from-branch origin/main` against
   it to surface gaps before building
3. Address findings
4. Open PR (or merge to main directly given solo-maintainer model)
5. Begin Stage 1
