# ADR: `dream` skill + `swarm dream` preset as pre-implementation interrogation primitive

**Date:** 2026-05-07
**Status:** Accepted, implemented in v0.8.0
**Spec:** [`docs/specs/2026-05-07-v0.8.0-dream-skill-and-preset.md`](../specs/2026-05-07-v0.8.0-dream-skill-and-preset.md)
**Author:** rubberduck

## Context

kaijutsu's pre-implementation pipeline had a gap. The path was `decide / spec-driven-development / planning-and-task-breakdown → incremental-implementation → polish → ship`. There was no formal step before deciding — no skill that asks "is this idea even worth pursuing? what are we missing? what's the strongest case against?"

The maintainer had been running an ad-hoc version of this manually with a recurring 7-question prompt template ("what do you think? what are honest thoughts? how does it fit the project's vibe? what's intuition saying? what aren't we asking? how could this be more awesome? wild ideas?"). The pattern worked. It deserved to be a skill.

Three other observations sharpened the design:

1. **kaijutsu has a unique multi-agent infrastructure** (v0.6 driver abstraction + persona dispatch). Other tools have multi-lens prompts; only kaijutsu can fan-out N agents × M lenses.
2. **v0.7 quality fingerprinting** can record per-(persona, preset, codebase) precision. A new preset gets weighter tracking for free if the schema can absorb it.
3. **No tool currently asks "should we build this?"** Every coding-agent skill is for HOW to build. Pre-commitment exploration is the empty slot.

## Decision

Three intertwined choices:

1. **Ship `dream` as both standalone skill AND multi-agent preset.** Standalone `/dream <topic>` walks lenses sequentially in a single agent session; `jutsu swarm dream <topic>` dispatches the N×lenses matrix across personas and synthesizes the cells into a 4-section markdown report. Both share the same lens definitions; only dispatch shape differs.

2. **8 lenses split into 4 always-on + 4 opt-in extras.** Base = honest / fit / gaps / wild (4 critical-or-meta-or-generative-or-contextual angles). Extras = adversary / inverse / status-quo / time (4 more critical or generative or temporal angles). Coverage balance: 4 critical, 2 generative, 1 contextual, 1 temporal.

3. **Lens-in-summary recording instead of schema migration.** Each finding's `summary` field starts with `[lens:<name>]` (whitelisted to 8 names); each finding's `reasoning` starts with `load_bearing: true|false`. v0.7 findings DB ingests these rows under the existing schema; v0.8.x can add a dedicated `lens TEXT` column when 30+ real dream sessions have informed the right shape.

## Why these three together

### Why standalone + multi-agent preset (not one or the other)

- **Standalone alone**: dream becomes "a thinking template you load into Claude" — duplicates anthropic skill-creator / superpowers brainstorming superficially. No moat.
- **Preset alone**: dream becomes "another swarm flavor" — same shape as brainstorm, harder to explain why it differs, no daily-use solo path.
- **Both**: standalone is the cheap daily-use path (single Claude session, no swarm cost); preset is the high-stakes interrogation (multi-agent matrix surfaces lens-blind-spots that a single agent's lens output can't detect).

### Why 8 lenses, 4 base + 4 extras

- **More than 8 = checklist not exploration.** Reviewers caught this during the doc-review pass: "lens fatigue / ceremony" was the top risk for v1.
- **Fewer than 4 = too narrow.** honest + gaps alone misses generative / temporal axes; the shape collapses to "list strengths and weaknesses + meta-evaluation" which reduces to existing critic patterns.
- **Base 4 covers the most common interrogation needs.** Critical (honest), contextual (fit), meta (gaps), generative (wild). Acceptable for ~90% of dreams.
- **Extras 4 cover stakes-dependent angles.** adversary for security-flavored topics; inverse for surprising orthogonality; status-quo for "should we even ship" decisions where do-nothing is a real option; time for ideas with multi-year decay/compounding.
- **Locked rationale**: `time` was chosen over `pre-mortem` after analysis — pre-mortem is critical-shaped (yet another failure-mode lens) while time is a unique temporal axis nothing else covers. Pre-mortem's failure-narrative value is partially recoverable from adversary + gaps; time's temporal axis is recoverable from no other lens.
- **`null` lens renamed to `status-quo`** to dodge JSON / YAML / Go reserved-word collisions in the lens-name whitelist.

### Why lens-in-summary instead of schema migration

- **v0.7 just shipped the findings DB.** Adding a column on the v0.8.0 ship would be premature — we don't yet know the right shape for dream-specific data.
- **Lens-in-summary works today**: existing `severity`, `summary`, `reasoning`, `confidence` columns absorb everything. The `[lens:<name>]` prefix is parseable; the whitelist of 8 names is closed.
- **v0.8.x migration is straightforward**: parse the prefix from existing summary fields, populate a new `lens TEXT` column, fall back to NULL for non-dream rows. Forward-compat by design.
- **v0.7's "schema lock-in is irreversible" principle** explicitly warned against speculation-driven schema design. Same principle applies here.

### Why anti-sycophancy is a prompt-level guardrail (not runtime check)

- **Runtime output verification needs an eval harness.** kaijutsu doesn't have one yet (deferred to v0.8.x along with `dream-as-eval-corpus`).
- **Prompt-level forbid list is a real guardrail.** It tells the model what NOT to produce; the regression test (parse each prompt file, assert forbid phrases survive) prevents accidental softening across releases.
- **Imperfect but correct shape**: the prompt is the only place a v0.8.0 anti-sycophancy rule can live. Adding a runtime check would be premature without the eval infrastructure to validate the runtime check itself works.

### Why we deferred receiving-code-review skill (the related-but-rejected idea)

This deserves its own bullet because earlier brainstorming considered porting `obra/superpowers/skills/receiving-code-review` as a kaijutsu primitive. Rejected because:

- v0.6 `--full` Pass-2 debate already provides agent-side discipline (agents critique each other before synth)
- v0.6 `--strict` lie-to-them filter strips sycophancy from synth output
- v0.7 per-tuple weights encode user's accept/dismiss precision history
- v0.6 multi-persona consensus diversifies bias

Adding receiving-code-review = HUMAN ceremony layered on top. Conflicts with kaijutsu's "trust the swarm" philosophy. Pre-implementation interrogation (dream) is HUMAN-ASSISTED PRE-COMMITMENT exploration. Reviewing-after-the-fact discipline is ALREADY HANDLED by the swarm's consensus + weights mechanisms. Decision: trust the swarm harder, ship `dream` instead.

## Consequences

### Positive

- **Closes the pre-implementation gap.** kaijutsu now has a primitive for the "should we even build this?" stage.
- **Operationalizes the maintainer's existing prompt template.** What was an ad-hoc 7-question copy-paste is now a structured 8-lens primitive with anti-sycophancy gates.
- **Multi-agent matrix is uniquely-kaijutsu.** No other tool has the swarm infrastructure to ship this shape.
- **Sets up v0.8.x learning loop.** Once 30+ dream sessions accumulate, v0.8.x's `lens TEXT` column + lens-precision tracking gives the synthesizer real data to downweight low-precision lenses per codebase.

### Negative

- **Cold-start UX**: dream output looks identical for new vs experienced users. No per-lens precision tracking until v0.8.x.
- **Synthesis cost grows with matrix size.** 4-persona × 8-lens = 32 cells in `--lenses=all` mode. `--max-cost` defaults raised but still real money for high-stakes runs.
- **Anti-sycophancy is a guardrail, not a guarantee.** Models may still produce performative agreement under load. Real fix needs an eval harness (v0.8.x).
- **Lens-in-summary encoding is transient.** Future schema column will need a migration parsing the `[lens:<name>]` prefix from legacy data. Documented + planned.
- **`--mode full` rejected for dream until v0.8.x.** dream-debate template is undefined; the reject is the conservative choice. Users redirected to `--lenses=all`.

### Neutral / deferred

- **Graveyard auto-write helpers** deferred to v0.8.x. Standalone `/dream` describes the format in SKILL.md; user (or a future helper script) writes the file.
- **Adaptive lens selection by topic** deferred — v0.8.0 ships static lens choice via `--lenses`. v0.8.x can pick lenses based on topic embedding + past-precision data.
- **dream-as-eval-corpus** deferred. "Did wild's predictions come true?" is the v0.8.x research question that closes the prediction loop.
- **Cross-user dream pollination** deferred. Same trust model as v0.7 telemetry deferral — irreversible privacy decision needs its own spec.

## Alternatives considered

1. **One-lens-per-agent** (each persona gets a single lens). Rejected because synthesizer can't surface lens-blind-spots without N agents running the SAME lens — the all-agents-agreed warning needs cross-agent comparison within a lens.
2. **Schema migration in v0.8.0** (add `lens TEXT` column up front). Rejected because spec design without 30+ real dream sessions would be guessing. v0.7 explicitly warned against this.
3. **dream as a swarm preset only** (no standalone skill). Rejected because the daily-use path needs to be cheap (single agent) — multi-agent matrix is the high-stakes mode, not the only mode.
4. **dream as a standalone skill only**. Rejected because the multi-agent matrix is the unique-kaijutsu shape; only-standalone makes dream indistinguishable from anthropic skill-creator + superpowers brainstorming.
5. **Pre-mortem instead of time as the 8th lens.** Rejected after axis-coverage analysis — pre-mortem is yet another critical-flavored lens; time is a unique temporal axis no other lens covers. See spec for full rationale.
6. **`null` as a lens name.** Rejected due to JSON / YAML / Go reserved-word collisions. Renamed to `status-quo`.
7. **`receiving-code-review` skill port** (the discipline-gate alternative). Rejected because v0.6 + v0.7 mechanisms already cover the discipline need via agent-side critique + weights, not human ceremony. See "Why we deferred receiving-code-review" above.

## Implementation note

3-stage build: skill scaffold + 8 prompts → swarm preset wiring → docs + ADR. Spec went through 1 doc-review round (31 findings, 11 real fixes applied; 5 minor cleanups; 3 false-positive dismissals). Stage 1 polish caught 8 issues across 4 passes. Stage 2 polish caught 1 issue (`--mode full` reject).

## Review

This ADR + spec went through 1 round of `jutsu swarm doc-review` with 4 personas (paranoid-security-claude + claim-auditor-deepseek + default-gemini + performance-deepseek). 31 findings, convergence on the structural issues (stop-condition ambiguity, matrix math contradictions, graveyard mode permissions, `--max-cost` defaults missing, `load_bearing` flagger contradiction, lens-rotation LEAD-undefined). All structural issues fixed before Stage 1 implementation began. Implementation green-light.
