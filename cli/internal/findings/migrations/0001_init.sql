-- v0.7.0 — quality fingerprinting initial schema.
--
-- See docs/specs/2026-05-06-v0.7.0-quality-fingerprinting.md and
-- docs/decisions/2026-05-06-quality-fingerprinting.md for design.

-- Tracks applied migration versions. Loader checks this table to
-- decide which NNNN_*.sql files still need to run. Forward-only —
-- no down-migration support; v0.8+ adds new files, never modifies
-- old ones.
CREATE TABLE IF NOT EXISTS schema_version (
    version    INTEGER PRIMARY KEY,
    applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- One row per finding from every swarm run.
--
-- Denormalized provider/persona/preset/codebase tags so the per-tuple
-- precision math (Weighter.WeightFor) is a single indexed query.
--
-- user_action: 'accepted' | 'dismissed' | NULL (pending). Bootstrap
-- weight kicks in at >= 1 actioned finding; mature at >= 10.
CREATE TABLE IF NOT EXISTS findings (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id        TEXT    NOT NULL,
    codebase_fp   TEXT    NOT NULL,
    preset        TEXT    NOT NULL,
    provider      TEXT    NOT NULL,
    persona       TEXT    NOT NULL,
    severity      TEXT    NOT NULL,
    file          TEXT    NOT NULL,
    line_range    TEXT    NOT NULL,
    summary       TEXT    NOT NULL,
    reasoning     TEXT,            -- nullable: agent may omit
    confidence    REAL,            -- nullable: agent may omit
    created_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    user_action   TEXT,            -- 'accepted' | 'dismissed' | NULL
    action_at     TIMESTAMP,
    action_reason TEXT
);

-- Composite index serves the sliding-window weight query:
--   SELECT user_action FROM findings
--   WHERE codebase_fp=? AND preset=? AND provider=? AND persona=?
--     AND user_action IS NOT NULL
--   ORDER BY action_at DESC LIMIT 200
-- Trailing action_at DESC lets the planner walk the index instead of
-- scanning. Verified via EXPLAIN QUERY PLAN during Stage 4 dev.
CREATE INDEX IF NOT EXISTS idx_findings_lookup
    ON findings(codebase_fp, preset, provider, persona, action_at DESC);

-- Partial index for "show me unactioned findings" (jutsu finding list
-- --pending). Cheap because most findings settle into accepted/
-- dismissed within a few days, leaving the partial index small.
-- Requires SQLite >= 3.8 (2014).
CREATE INDEX IF NOT EXISTS idx_findings_pending
    ON findings(id) WHERE user_action IS NULL;
