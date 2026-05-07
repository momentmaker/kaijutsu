-- v0.9.0 — dream lens column + finding position. Promotes lens from
-- a [lens:<name>] prefix in summary to a first-class indexed column,
-- enabling per-lens precision tracking in the weighter. Adds
-- position for PR-comment auto-detection's stable per-finding ID.
--
-- Backfill: NULL for rows where summary doesn't carry the prefix
-- (most pre-v0.8 rows, plus non-dream presets). The recorder
-- populates lens for new dream rows; non-dream rows leave it NULL
-- forever (the column is dream-specific in spirit, but lives on the
-- shared findings table to avoid a join on every weight lookup).
--
-- position is meaningful for runs whose comments humans interact
-- with via sync-pr; old rows get NULL and sync-pr skips them with
-- a logged warning.
--
-- Spec: docs/specs/2026-05-07-v0.9.0-feedback-loops.md §1
-- Plan: IMPLEMENTATION_PLAN.md Stage 1

ALTER TABLE findings ADD COLUMN lens TEXT;
ALTER TABLE findings ADD COLUMN position INTEGER;

-- Backfill lens from [lens:<name>] summary prefix. LIKE patterns
-- side-step bracket-counting fragility; SQLite default LIKE treats
-- '[' and ']' as literals (no character-class semantics).
--
-- The 8-lens vocabulary below MUST stay in sync with
-- swarm.DreamLensesAll() in cli/internal/swarm/preset.go. v0.9 freezes
-- the set; any future lens addition in v1.0+ requires a new migration
-- (e.g. 0003_lens_<name>.sql) that backfills new rows AND updates the
-- recorder validator's whitelist. Do NOT add lens names by editing
-- 0002 in place — migrations are forward-only.
UPDATE findings SET lens = 'honest'     WHERE lens IS NULL AND summary LIKE '[lens:honest]%';
UPDATE findings SET lens = 'fit'        WHERE lens IS NULL AND summary LIKE '[lens:fit]%';
UPDATE findings SET lens = 'gaps'       WHERE lens IS NULL AND summary LIKE '[lens:gaps]%';
UPDATE findings SET lens = 'wild'       WHERE lens IS NULL AND summary LIKE '[lens:wild]%';
UPDATE findings SET lens = 'adversary'  WHERE lens IS NULL AND summary LIKE '[lens:adversary]%';
UPDATE findings SET lens = 'inverse'    WHERE lens IS NULL AND summary LIKE '[lens:inverse]%';
UPDATE findings SET lens = 'status-quo' WHERE lens IS NULL AND summary LIKE '[lens:status-quo]%';
UPDATE findings SET lens = 'time'       WHERE lens IS NULL AND summary LIKE '[lens:time]%';

-- Lens-aware weight lookup index. Composite layout matches the
-- weighter's lens-aware query: WHERE codebase_fp=? AND preset=?
-- AND provider=? AND persona=? AND lens=?
CREATE INDEX IF NOT EXISTS idx_findings_lens_lookup
    ON findings(codebase_fp, preset, provider, persona, lens, action_at DESC);
