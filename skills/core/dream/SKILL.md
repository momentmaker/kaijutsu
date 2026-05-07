---
name: dream
description: Pre-implementation interrogation. Walks any topic through 8 cognitive lenses to surface hidden assumptions, weak premises, orthogonal angles, temporal decay, and adversarial misuse BEFORE the topic becomes a spec / plan / commit. Use when the user says "what do you think about X?", "should we do X?", "is X worth pursuing?", "let's brainstorm" (note: this is NOT brainstorm — see "vs brainstorm preset" below), or invokes /dream. Composes via jutsu swarm dream for multi-agent matrix mode.
---

# dream

Pre-implementation interrogation. The user has an idea. Before they spec it, plan it, or commit to it — pressure-test it through 8 lenses.

This is the EARLIEST stage in the kaijutsu pipeline:

```
dream → spec → plan → implement → polish → ship
```

Nothing else covers this slot today. `brainstorm` preset gives you 5 OPTIONS to solve a problem; `dream` interrogates whether the problem is the right shape in the first place.

## When to invoke

- The user says "what do you think about X?" / "should we do X?" / "is X worth pursuing?"
- Before any non-trivial spec/plan
- When the user types `/dream <topic>`
- When kaijutsu's `unstuck` reaches its walk-away level — a fresh re-entry can manually invoke `dream` on the topic to break the groove (no automatic integration in v0.8.0)
- Composed by future skills via `deps.skills`

## When NOT to invoke

- Trivial fix (1-line bug, typo, dependency bump). Dream's lens machinery is overkill for a "should I rename this var?" question.
- The user has already committed. dream is for OPEN ideas, not retrospective justification.
- The topic IS already structured (spec exists, plan exists). Use `scope-check` for cost-tier classification or `polish` for review-and-fix instead.

## vs `brainstorm` preset

| | brainstorm preset | dream skill |
|---|---|---|
| Question shape | "Give me 5 ways to solve X" | "Should we even pursue X?" |
| Output | options to pick | perspectives to weigh |
| Stage | after commitment to solve X | before commitment |
| Lens count | 1 prompt per agent | 4-8 lenses per agent |
| Compose with | implementation skills downstream | spec-driven-development downstream |

Both can be useful in sequence: `dream` first to validate the problem shape, then `brainstorm` to enumerate solutions. Don't substitute one for the other.

## Anti-sycophancy gates (NON-NEGOTIABLE)

Every lens prompt opens with explicit forbid list. The user does not need encouragement. They need real perspective. Dream is worse than useless if it produces validating noise.

Forbidden phrasings:

- "Great question!" / "Excellent point!" / "You're absolutely right!"
- "This is interesting" / "fascinating" / "thoughtful" — without specifying WHY
- "Could be worth considering" / "Might be helpful" / "Could potentially" — hedging that means nothing
- Any opening that validates before substantive content

These appear inside every `prompts/base/*.md` and `prompts/extras/*.md`. The regression test in `tests/` parses each file and asserts the forbid list is present (guards against accidental softening of the prompt text — does NOT verify model output, which would need an eval harness deferred to v0.8.x).

## The 8 lenses

### Base lenses (always run)

| Lens | Asks | Mode |
|---|---|---|
| **honest** | What's actually good and bad about this — no sycophancy, no diplomatic balance | critical |
| **fit** | Does this match the project's vibe / direction / unspoken constraints | contextual |
| **gaps** | What aren't we asking? What hidden assumptions ride along? | meta-critical |
| **wild** | Orthogonal extensions; "what's the 10x version?" | generative |

### Extras (opt-in via `--lenses all` or `--lenses <comma-list>`)

| Lens | Asks | Mode |
|---|---|---|
| **adversary** | How does a hostile actor abuse this? Worst-case interpretation | critical / security |
| **inverse** | What if we did the OPPOSITE? "Stop NOT-X" often truer than "do X" | generative-via-negation |
| **status-quo** | What if we don't do this? Status-quo strength + opportunity cost of inaction | critical |
| **time** | Pick a date 2 years out. What changed? Is this still relevant or quaint? | temporal |

Coverage: 4 critical, 2 generative, 1 contextual, 1 temporal.

## Output shape

Each lens emits a JSON ARRAY of objects (multiple thoughts per lens permitted; honest lens often surfaces 3-5):

```json
[
  {
    "lens": "gaps",
    "thought": "We aren't asking how this interacts with users on the legacy auth path...",
    "load_bearing": true,
    "confidence": 0.85
  },
  {
    "lens": "gaps",
    "thought": "Cost projection assumes 4 personas; --lenses=all doubles it.",
    "load_bearing": false,
    "confidence": 0.70
  }
]
```

### `load_bearing` — calibrated definition

A finding is `load_bearing: true` if at least ONE of:

- It changes whether the idea should proceed at all (kill-or-continue signal).
- It reveals a constraint that wasn't part of the original framing.
- It surfaces a hidden assumption that, if wrong, invalidates the idea's premise.
- The user reading it would say "wait — I didn't know that" or "that changes things" rather than "good point" / "noted".

A finding is `load_bearing: false` for "noted, but doesn't change the path" observations. Models that flag everything as load-bearing produce noise.

### Severity mapping (when recording to v0.7 findings DB via swarm dream preset)

| `load_bearing` | confidence | severity |
|---|---|---|
| true | ≥ 0.8 | blocker |
| true | < 0.8 | issue |
| false | ≥ 0.5 | minor |
| false | < 0.5 | info |

## Process

### Step 0: Graveyard recall

Before running lenses, check `~/.kaijutsu/dreams/` for prior dreams on the same topic (scoped to the current cwd's v0.7 codebase fingerprint by default; `--all-codebases` drops the scope).

Match: file naming `<codebase-fp>-<topic-slug>-<YYYYMMDD-HHMMSS>.md` within 7 days of now.

If a match exists:
1. Surface the prior dream summary to the user.
2. Apply the **lens-rotation rule**: drop the prior dream's LEAD lens (the first lens in its run order, recorded in the file header) to the END of this dream's lens list. Promote the second lens to LEAD.
3. Example: prior dream ran `[honest, fit, gaps, wild]`. New lens order: `[fit, gaps, wild, honest]`.
4. v0.8.0 ships this rotation rule hardcoded; v0.8.x may make it configurable.

Recall logic is local to the dream skill — reads `~/.kaijutsu/dreams/` directly, no shared schema.

### Step 1: Run lenses

Standalone `/dream <topic>`:
- Runs all 4 base lenses sequentially in the current agent session
- `--lenses all` expands to 8 lenses (4 base + 4 extras); `--lenses honest,gaps,inverse` for explicit subset
- Each lens prompt is loaded from `prompts/base/<name>.md` or `prompts/extras/<name>.md`
- The lens prompt template gets `%s` replaced with the topic
- Each lens produces a JSON array per the output shape above

`jutsu swarm dream <topic>` (multi-agent preset, see Stage 2):
- Each persona runs ALL lenses in parallel
- Cell count = `personas × lenses`
- Default 4 personas × 4 base = 16 cells; `--lenses all` = 32
- Synthesizer aggregates the matrix at the end

### Step 2: Stop condition (standalone only)

After each lens completes, scan its output for `load_bearing: true`. If at least one load-bearing insight surfaced, prompt the user:

> "Lens X surfaced a load-bearing insight: <thought>. Stop here and act on this, or continue the remaining lenses?"

User chooses. Default for swarm dream preset: no early-out — all cells run in parallel, synthesizer surfaces load-bearing cells visually at the top.

### Step 3: Synthesize

Standalone:
- Per-lens findings sorted by confidence
- Load-bearing insights flagged at top
- Cross-lens consensus surfaced (2+ lenses converging on the same insight = signal)

Swarm preset:
- Cluster matching insights across the matrix
- Surface lens-blind-spots: if all agents converged on the same answer for the same lens, flag as suspected model-shared training bias
- Surface lens-unique findings: a single agent's lens producing load-bearing insights no other cell produced is high-signal worth investigating

### Step 4: Archive to graveyard

Write the session output to `~/.kaijutsu/dreams/<codebase-fp>-<topic-slug>-<YYYYMMDD-HHMMSS>.md`. File mode 0600. Header records:

```yaml
---
topic: <topic>
codebase_fp: <fp>
lens_order: [honest, fit, gaps, wild]   # the LEAD-first order; future rotations consult this
mode: standalone | swarm
full: true | false
created_at: <ISO 8601>
---
```

Body: rendered prose summary (preserved for human re-read) + structured JSON appendix (parseable for v0.8.x lens-precision tooling).

Topic-slug derivation: lowercase + non-alphanumeric → `-` + collapse repeats + trim leading/trailing → 50-char cap. Stable + readable + bounded.

## Anti-pattern: dreaming as ceremony

If dream is invoked on every micro-decision, the user starts producing template-shaped filler answers. The skill should preserve EXPLORATION, not become a checklist.

Signals dream is being misused:
- Topic is too narrow ("should I rename this var?")
- All findings are `load_bearing: false` (filler, no signal)
- User skips reading the output and just wants the slug for the graveyard

Counter (user-side): if a session's findings are ALL `load_bearing: false` AND ALL confidence < 0.5, treat that as a signal that the topic was too narrow OR the agents are filling in template noise. Step away, reframe the topic, retry. v0.8.x may automate this detection; v0.8.0 ships user judgment.

## Hard rules

- **Anti-sycophancy preamble is in every lens prompt.** Don't soften it across releases.
- **`load_bearing` is binary.** Models forced to pick between true/false; no `partially_load_bearing` middle.
- **Topic-slug + codebase-fp scope every graveyard file.** Cross-codebase recall is opt-in only.
- **Standalone has early-out; swarm runs all cells.** Different shapes, both valid.
- **Output is structured JSON + prose archive.** JSON is composable; prose is honest. Ship both.
- **No network calls from dream itself.** Composes existing swarm primitives (which have their own privacy boundaries via v0.7); adds no new network surface.

## Composes

- `swarm dream` preset (Stage 2) — multi-agent matrix mode, records lens outputs to v0.7 findings DB via `[lens:<name>]` summary prefix
- (deferred) `convergence-detect` — optional stop signal for multi-round dreams; v0.8.x candidate when usage data justifies it

## Future (v0.8.x)

- Per-lens precision tracking via dedicated `lens TEXT` schema column on findings
- Adaptive lens selection by topic embedding
- Lens-emphasis weighting in synthesizer (downweight low-precision lenses per codebase)
- Auto-prune for graveyard (`jutsu dream clear --older-than 365d`)
- `dream-as-eval-corpus`: "did wild's predictions come true 6 months later?" closes the prediction loop
