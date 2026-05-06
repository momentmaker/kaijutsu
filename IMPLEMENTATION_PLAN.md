# IMPLEMENTATION_PLAN.md — v0.7.0 quality fingerprinting

Spec: [`docs/specs/2026-05-06-v0.7.0-quality-fingerprinting.md`](docs/specs/2026-05-06-v0.7.0-quality-fingerprinting.md)
ADR:  [`docs/decisions/2026-05-06-quality-fingerprinting.md`](docs/decisions/2026-05-06-quality-fingerprinting.md)

(Replaces the v0.4 Phase-1 plan that lived here previously — that work shipped + merged in v0.4/v0.5.)

## Pre-flight decisions (one-time, sign off before Stage 1)

| Decision | Choice | Rationale |
|---|---|---|
| SQLite Go lib | **`modernc.org/sqlite`** (pure Go) | No CGo → goreleaser cross-compile stays clean; ~5MB binary growth acceptable |
| Migrations | **`go:embed migrations/*.sql`** | Compile-time inclusion; no runtime FS lookup; testable |
| DB lifecycle | Lazy open, singleton per process, no explicit Close | Cobra subprocess model already short-lived; SQLite WAL handles process exit cleanly |
| Concurrency | **WAL mode** for the DB | Multi-process safe; reads don't block writes |
| Test isolation | `t.TempDir()` + `t.Setenv("HOME", ...)` per test | Same pattern Stage 5 polish established for `agent_subcommands_test` |

**Open before Stage 1**: confirm `modernc.org/sqlite` vs `mattn/go-sqlite3`. Recommend modernc.

---

## Stage 1 — Schema + storage primitives

**Goal:** `cli/internal/findings/` package with `Store` type that opens/migrates `~/.kaijutsu/findings.db`. Idempotent migration apply. NO CLI surface yet, NO recording wired.

**Success:**
- `store_test.go` passes: create-from-empty, re-open existing, schema_version idempotency, double-Open safe
- `~/.kaijutsu/findings.db` mode `0600` verified
- Forward-only schema apply: `0001_init.sql` runs once; subsequent `Open()`s no-op
- Min SQLite version `3.8` documented in CHANGELOG

**Files:**
- `cli/internal/findings/store.go`
- `cli/internal/findings/migrations/0001_init.sql`
- `cli/internal/findings/store_test.go`

**Tasks:**
1. `go get modernc.org/sqlite`
2. Schema DDL: `findings` table per spec (id PK autoincrement, NOT NULL fields per spec, NULLABLE for reasoning/confidence, partial index on `WHERE user_action IS NULL`)
3. Composite index `(codebase_fp, preset, provider, persona, action_at DESC)`
4. `Store` type: `Open() (*Store, error)`, `Close() error`, internal `db *sql.DB`
5. `applyMigrations(db)` — read embedded SQLs in order, check `schema_version` table, apply pending in a transaction
6. chmod `0600` on file creation + re-assert on every Open
7. Tests via `t.TempDir()` + `t.Setenv("HOME", tmp)`

**Risks:**
- modernc.org/sqlite + transitive deps may bloat go.mod meaningfully. Audit after `go mod tidy`.
- File-mode setting is platform-specific (Windows ignores). Document.

**Polish gate before Stage 2.**

---

## Stage 2 — Recorder + fingerprint

**Goal:** Recorder writes one row per finding after FanOut. Fingerprint computation per the spec's 5-step fallback chain. Best-effort: failure logs to stderr, never blocks swarm pipeline.

**Success:**
- Real swarm doc-review run against this branch's spec creates the DB and inserts ~30 rows
- `sqlite3 ~/.kaijutsu/findings.db "SELECT count(*) FROM findings WHERE run_id=?"` matches the run's findings count
- Fingerprint chain tested across all 5 fallback paths (origin, upstream, alphabetical, no-remote git, non-git)
- DB write failure → stderr warning, swarm pipeline still produces markdown output

**Files:**
- `cli/internal/findings/recorder.go` — `Recorder.Insert(runID, codebaseFP, preset string, results []swarm.AgentResult) error`
- `cli/internal/findings/fingerprint.go` — `Fingerprint(cwd string) string`
- `cli/internal/findings/recorder_test.go`
- `cli/internal/findings/fingerprint_test.go`
- `cli/internal/cli/swarm.go` — post-FanOut hook: `recorder.Insert(...)` after the existing `overlayPersonaCosts` call

**Tasks:**
1. Implement Fingerprint with 5-step fallback (git origin → git upstream → alphabetical first remote → local-git → local-fs)
2. Implement Recorder.Insert: open Store, batch INSERT in one transaction, return error
3. Hook into runSwarmPipeline: after results materialize, before markdown render
4. Best-effort wrapper: log + continue on error
5. Tests use git fixtures (init repos in t.TempDir, set remotes via `git remote add`)

**Risks:**
- Shelling out to `git` from inside our binary — exec cost per run. Acceptable (~50ms).
- Fingerprint stability across machines: depends on git remote being identical (HTTPS vs SSH vs git@). Spec acknowledges this as v0.7 behavior; users with mixed access patterns get separate fps.
- Transaction failure mid-batch → all-or-nothing. Acceptable for findings.

**Polish gate before Stage 3.**

---

## Stage 3 — `jutsu finding` subcommand group

**Goal:** Full CLI surface (list, accept, dismiss, stats, clear, export). All flow through Store. Cross-codebase guard on accept/dismiss. Clear defaults to dry-run.

**Success:**
- End-to-end: swarm run → `jutsu finding list --pending` shows pending findings → `jutsu finding accept 5` → `jutsu finding stats` shows the tuple updated → `jutsu finding clear --older-than 365d --yes` works
- Cross-codebase guard: `accept <id>` against finding from different fp errors out without `--cross-codebase`
- Stats header: `current codebase fp: <16-char hex>` line present
- `clear` without `--yes` prints would-delete count + exits 0

**Files:**
- `cli/internal/cli/finding.go` — cobra command group
- `cli/internal/cli/finding_test.go` — integration via temp HOME

**Tasks:**
1. `newFindingCmd()` group registration in `root.go`
2. Per-subcommand cobra implementations (list / accept / dismiss / stats / clear / export)
3. Cross-codebase guard: read fp before accept/dismiss, error unless flag passed
4. Dry-run default for clear: refuse without `--yes` OR `--dry-run`
5. Export JSON with top-level `schema_version: 1` field
6. Import-list test: `cli/internal/cli/finding.go` MUST NOT import `net/http`, `net`, `net/url` (privacy guarantee)
7. Tests for each subcommand via tempdir + tempdb

**Risks:**
- ID stability: global autoincrement. If user does `clear --older-than 365d` then accepts an old id, NotFound error. Acceptable UX.
- `--reason` text could be long; SQLite handles via TEXT. No truncation.

**Polish gate before Stage 4.**

---

## Stage 4 — Weighter + synthesizer integration

**Goal:** Weighter computes per-tuple precision with sliding window. Synthesizer reads weights, applies to clusterFindings sort, optionally renders in disagreement table.

**Success:**
- Weighter unit test: 3-state algorithm (cold=1.0 / bootstrap=0.7 / mature=precision) verified at boundaries (0, 1, 9, 10, 50, 200, 201)
- Sliding window test: insert 250 actioned findings; verify only most-recent 200 by `action_at` influence the weight
- Integration test with fixture DB: pre-populate 50 actioned findings biased toward gemini-right + codex-wrong; run synthesis; verify cluster ordering reflects weights
- Performance benchmark: `BenchmarkWeightFor` against 50K-row DB returns < 5ms per lookup
- Backward compat snapshot: with no DB, swarm output for fixed input matches v0.6.2 byte-for-byte
- `--show-weights` flag wired to swarm subcommands; off by default

**Files:**
- `cli/internal/findings/weighter.go` — `Weighter.WeightFor(provider, persona, preset, codebaseFp string) float64`
- `cli/internal/findings/weighter_test.go` — incl. benchmark
- `cli/internal/swarm/synth.go` — extend clusterFindings + renderDisagreementTable
- `cli/internal/swarm/synth_test.go` — extend with weighted-consensus tests
- `cli/internal/cli/swarm.go` — wire `--show-weights` flag

**Tasks:**
1. Weighter.WeightFor: sliding-window query (`SELECT user_action FROM findings WHERE codebase_fp=? AND preset=? AND provider=? AND persona=? AND user_action IS NOT NULL ORDER BY action_at DESC LIMIT 200`)
2. Apply 3-state algorithm
3. clusterFindings: pass weights map, compute weighted_consensus per cluster (unique reporters), use as new secondary sort key
4. renderDisagreementTable: if `--show-weights`, append `(0.85)` to column header
5. Synthesizer prompt: include `weights:` section when any weight ≠ 1.0
6. `--show-weights` flag in `commonSwarmFlags`

**Risks:**
- Provider lookup at synthesis time — synth.go currently doesn't know provider per AgentResult. Already addressed by `AgentResult.Driver` from v0.6.1; same path can recover provider via persona registry. Document the lookup chain in code comment.
- Synthesizer prompt format change: existing `--full` debate prompt may need adjustment. Check + update.
- Performance: ensure the index actually serves the query. Run `EXPLAIN QUERY PLAN` during dev.

**Polish gate before Stage 5.**

---

## Stage 5 — Docs, CHANGELOG, ROADMAP, release

**Goal:** ADR (already done). docs/multi-agent.md extended. CHANGELOG. ROADMAP updated for v0.8 deferrals. Release validated.

**Success:**
- Verification script (from spec Verification section) passes end-to-end
- brew tap formula generates → 0.7.0
- release-jutsu workflow green on tag
- sign-core green
- CHANGELOG entry + ROADMAP v0.8 row
- `docs/multi-agent.md` gains "Quality fingerprinting" section

**Files:**
- `docs/multi-agent.md` (extend)
- `CHANGELOG.md` (entry under `## [0.7.0]`)
- `ROADMAP.md` (v0.7 marked done; v0.8 row enumerates deferrals)

**Tasks:**
1. Doc extension covering: store location + privacy boundary, weight algorithm, CLI usage examples, opt-out
2. CHANGELOG: Added (Schema/Recording/CLI/Weighting/Synthesizer/Privacy), Changed, Deprecated (none), Notes
3. ROADMAP: v0.7 → ✅; v0.8 row gains telemetry / sync / PR-comment / multi-stage / streaming / reverse / persona-registry / per-skill-routing
4. Tag v0.7.0 + push
5. Smoke verify against fresh /tmp dir

**Risks:**
- Spec Verification depends on having ~50 swarm runs to demonstrate weighted ordering. Either pre-populate fixture DB OR document as "manual verification after some real usage" in CHANGELOG.

---

## Cross-stage concerns

### Test data strategy

- Stage 1-3: tempdir + tempdb. Cheap, isolated.
- Stage 4: fixture DB committed to `cli/internal/findings/testdata/fixture_50_runs.sqlite` for reproducible weighted-synthesis tests. Generate once via a `gen_fixture.go` build-ignored helper.

### Performance budget

- WeightFor < 5ms (50K-row DB) — gate via `BenchmarkWeightFor`
- Recorder.Insert < 50ms (one batch) — gate via `BenchmarkRecorderInsert`
- Both verified in Stage 4 polish.

### Adversarial review at branch end

After Stage 5 — same flow as v0.6: run `jutsu swarm pr-review --diff-from-branch main --personas paranoid-security-claude,performance-deepseek,default-gemini,claim-auditor-deepseek` against the cumulative diff. Expect to surface 3-8 real bugs (rate concurrency, schema edge cases, cli ergonomics). Squash before merge.

### Branch / merge / tag flow

- Branch: `v0.7-impl` (off main, after spec lands)
- Per-stage commits with polish loops
- Final adversarial review round before merge
- Merge with `--no-ff` to preserve stage history
- Tag `v0.7.0` annotated
- Push triggers goreleaser + sign-core

### Time estimate

- Stage 1: ~half day (SQLite library + migration + tests)
- Stage 2: ~half day (fingerprint chain + recorder + git fixtures)
- Stage 3: ~1 day (CLI surface — biggest scope)
- Stage 4: ~1 day (Weighter + synth integration + benchmarks + fixture DB)
- Stage 5: ~half day (docs + release)
- Polish + adversarial review: ~half day

Total: ~4-5 focused days.

---

## Sign-off needed before Stage 1 starts

1. **modernc.org/sqlite vs mattn/go-sqlite3** — recommend modernc (no CGo, cross-compile clean)
2. **JUTSU_FINDINGS_DB env var deferred to v0.7.x?** — spec says yes; confirm
3. **Verification script's "50 runs" demonstration** — pre-populate fixture OR doc as manual?
