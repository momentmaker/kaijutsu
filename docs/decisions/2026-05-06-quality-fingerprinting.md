# ADR: Quality fingerprinting via local SQLite + per-tuple confidence weighting

**Date:** 2026-05-06
**Status:** Accepted, implemented in v0.7.0
**Spec:** [`docs/specs/2026-05-06-v0.7.0-quality-fingerprinting.md`](../specs/2026-05-06-v0.7.0-quality-fingerprinting.md)
**Author:** rubberduck

## Context

v0.6 swarm dispatches the same N personas with equal voice every time. After 50+ runs against a given codebase the user has implicit knowledge of which agent's findings consistently land vs which agent's findings consistently get dismissed — but jutsu doesn't capture that signal. The 51st run weights gemini's pattern-consistency finding the same as it weights gemini's previous 50 findings the user dismissed without action.

The compounding effect: swarm value plateaus. (N - K) low-quality findings per run become accepted noise the user trains themselves to skim past. The "more agents = better signal" thesis weakens as noise accumulates uniformly across providers.

## Decision

Three intertwined choices:

1. **Local SQLite store** at `~/.kaijutsu/findings.db`. Records every finding from every swarm run with denormalized provider/persona/preset/codebase tags so per-tuple precision math is one indexed query.

2. **Per-tuple confidence weighting** keyed by `(provider, persona, preset, codebase_fp)`. Three-state algorithm: cold-start (weight=1.0, byte-identical v0.6 behavior), bootstrap (1≤actioned<10 → weight=0.7), mature (≥10 → precision = accepted/(accepted+dismissed), clamped [0.05, 1.0]).

3. **Sliding window of 200 actioned findings per tuple** measured by `action_at`. Drift detection without manual retraining; new model performance dominates after ~50 actioned findings.

Synthesizer integration is small: `clusterFindings` secondary sort uses `weighted_consensus = sum(unique reporter weights)` instead of raw `ConsensusOf`. Display behavior unchanged by default; `--show-weights` flag adds weight to disagreement-table column headers.

## Why these three together

### Why local SQLite (not a flat file, not a service)

- **SQLite is goroutine-safe and battle-tested.** WAL mode supports concurrent reads with one writer; sufficient for the swarm dispatch pattern (write batch after fan-out, read at start of next dispatch).
- **Indexed queries scale.** Composite index on `(codebase_fp, preset, provider, persona, action_at DESC)` lets the sliding-window query walk the index without a scan even at 100K+ rows.
- **Migration story is solved.** Numbered `migrations/NNNN_*.sql` files + a `schema_version` table give us forward-compatible schema evolution from day one. Forward-only by design — downgrades intentionally unsupported.
- **Flat file (CSV / JSON-Lines) was considered and rejected:** every weight lookup would require either parsing the whole file or building an in-memory index on every CLI invocation. Cold start for stats/synthesis would be O(N).
- **Service (sqlite over network, postgres) considered and rejected:** v0.7 is local-only by design. Multi-machine sync is a v0.8+ concern with privacy implications worth their own ADR.

### Why per-tuple weighting (not per-provider, not global)

- **Codebase context dominates precision.** Claude's pattern-consistency lens may have 90% precision on a Go codebase and 40% on a Rust one (different idiom corpus). A per-provider weight averages those into noise.
- **Preset context dominates precision.** Codex's edge-case lens is great at pr-review, mediocre at brainstorm. A per-(provider, preset) weight captures the difference; per-(provider) doesn't.
- **Persona context dominates precision.** `paranoid-security-claude` and `default-claude` are the same provider but produce findings with different precision profiles. Per-(provider) collapses them.
- The 4-tuple `(provider, persona, preset, codebase_fp)` is the smallest grouping that captures all relevant context without exploding into too-fine cells (every tuple would be perpetually undercounted).

### Why sliding window of 200 actioned findings

- **Catches model drift.** When Anthropic ships claude-opus-5, the previous claude-opus-4.7's precision data shouldn't dominate the weight forever. 200 actioned findings × ~3 actioned-per-run ≈ 60 swarm runs of memory; the model-update signal dominates after ~17 swarm runs (rough math — see Risks).
- **Enough to compute reliable precision.** At 200 samples, precision has tight enough confidence intervals (sqrt(p(1-p)/n) at p=0.5 = 0.035) that we trust the value to ±5%.
- **Bounded write amplification.** Sliding window doesn't truncate the DB; it just LIMITs the read query. So the table grows linearly forever; users explicitly clear via `jutsu finding clear --older-than 365d`.

## Why no telemetry

A dedicated section because every product like this faces pressure to add anonymized opt-in sharing of aggregate `(provider, persona, preset, precision)` tuples to a central dashboard.

**Decision: defer to v0.8+ behind its own spec + ADR.** Reasons:

1. **Privacy is irreversible.** Once telemetry ships, removing it post-fact is a trust event. Better to never have it than to add+remove.
2. **Local-only first proves the model.** If per-tuple weighting works for the maintainer and a handful of early adopters, telemetry can be added later. If it doesn't work, telemetry would add noise to the failure analysis.
3. **No clear consent flow.** "Anonymized aggregate" is a research challenge in this domain — finding text contains code snippets that may be identifying. Until we have a credible scrubbing story, sharing is unsafe.
4. **No clear sharing protocol.** Push to where? Federated? Centralized? Each option has its own design + ops cost.

The decision: v0.7 ships with literally no network code in `cli/internal/cli/finding.go`. Verified by an import-list test. Future telemetry is a positive-decision-with-spec, not a negative-default-overridden-by-flag.

## Consequences

### Positive

- Compounding value: jutsu gets smarter the more the user uses it. Every prior swarm run pays dividends on the next.
- Cross-vendor noise asymmetry surfaces explicitly. "Claude is great on auth, gemini's nits get ignored, deepseek's perf finds are gold" becomes machine-readable, not lore.
- New team members joining a project benefit from the existing user's accept/dismiss history (until v0.8 ships sync; until then they'd `jutsu finding export` + import).
- Foundation for v0.8 features: telemetry, multi-machine sync, PR-comment auto-detection, cross-codebase learning all build on this schema.

### Negative

- Cold-start problem: zero value until the user has actioned ~10 findings per tuple. New users see no benefit for the first ~5-10 swarm runs. Mitigated by explicit "(insufficient data, default weight: 0.7)" messaging in `jutsu finding stats`.
- Storage growth: linear in finding count. A user running 5 swarm runs/day × 30 findings/run × 365 days = ~55K rows/year. SQLite handles it but the file grows. `jutsu finding clear --older-than 1y` is the user's lever; not auto-pruned in v0.7.
- Schema lock-in: v0.7 schema decisions constrain v0.8+ features. Mitigated by explicit migration framework + forward-only design + this ADR for the rationale.
- Confirmation bias risk: if user only dismisses findings they disagree with (and never accepts), precision math underestimates. Documented in spec Risks.

### Neutral / deferred

- Configurability (bootstrap weight, window size, DB location) deferred to v0.7.x once we have real-world data on whether users want to tune these knobs. v0.7 ships hardcoded defaults.
- PR-comment auto-detection deferred to v0.8.
- Multi-machine sync deferred to v0.8.

## Alternatives considered

1. **Server-side telemetry** (anonymized aggregate sharing). Rejected for v0.7 — see "Why no telemetry" above. Reserved for v0.8+ behind a separate ADR.
2. **Manual user-tagged weights** (e.g., `~/.kaijutsu/weights.toml` declaring per-agent multipliers). Rejected because it requires the user to do the math jutsu can do automatically. Easier to dismiss every claude finding manually than to compute precision and write a config file.
3. **Per-provider only weighting** (no preset/persona/codebase keys). Rejected because it averages away the very signal the feature exists to surface — see "Why per-tuple weighting."
4. **Hard suppression** (low-weight findings silently dropped). Rejected because it removes user agency and visibility. Down-weighting is reversible (one accept resets the slope); suppression hides issues the user might want to see.
5. **External tools** (build it on top of an existing tool like git-blame quality, code-review-stats). Rejected because no existing tool covers the (provider, persona, preset, codebase) keying jutsu needs.

## Implementation note

7-stage build sequence in the spec; shippable in 5-7 days of focused work. Cold-start UX is the most-load-bearing item — the messaging in `jutsu finding stats` when tuples are below threshold determines whether early users feel the feature is "broken" vs "just collecting baseline."

## Review

This ADR + spec went through 3 rounds of `jutsu swarm doc-review` with 3 personas (claude paranoid + claim-auditor-deepseek + gemini). Round 1: 35 findings. Round 2: 29. Round 3: 31 mostly 1/3 contested — convergence reached. Implementation green-light.
