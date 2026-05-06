package findings

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/momentmaker/kaijutsu/cli/internal/swarm"
)

// RunMeta is the per-run context the recorder needs to denormalize
// each finding into a row. RunID is the swarm cache key (used to link
// findings back to runs/<key>/ for replay), CodebaseFP is from
// Fingerprint, Preset is the preset name (e.g. "pr-review"). The
// caller passes ProviderForPersona so the recorder doesn't have to
// re-resolve the persona registry: legacy mode uses the agent name as
// the provider; persona mode looks up the underlying provider via
// the personaAdapter.
type RunMeta struct {
	RunID              string
	CodebaseFP         string
	Preset             string
	ProviderForPersona map[string]string
}

// RecordRun inserts one row per finding from results. All inserts run
// in a single transaction; partial failure rolls back the entire
// batch (a half-recorded run is worse than no record).
//
// Returns the number of rows actually written. Skips AgentResult
// entries with Err set (errored agents have nothing useful to record),
// and skips empty Findings slices.
//
// Best-effort caller pattern: the swarm pipeline wraps this in a
// log-and-continue so a bad DB never blocks the markdown render.
func RecordRun(s *Store, meta RunMeta, results []swarm.AgentResult) (int, error) {
	if s == nil || s.db == nil {
		return 0, errors.New("recorder: nil store")
	}
	if meta.RunID == "" {
		return 0, errors.New("recorder: RunID required")
	}
	if meta.CodebaseFP == "" {
		return 0, errors.New("recorder: CodebaseFP required")
	}
	if meta.Preset == "" {
		return 0, errors.New("recorder: Preset required")
	}

	tx, err := s.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin: %w", err)
	}
	stmt, err := tx.Prepare(`
		INSERT INTO findings(
			run_id, codebase_fp, preset, provider, persona,
			severity, file, line_range, summary, reasoning, confidence
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		_ = tx.Rollback()
		return 0, fmt.Errorf("prepare: %w", err)
	}
	defer stmt.Close()

	written := 0
	for _, r := range results {
		if r.Err != "" || len(r.Findings) == 0 {
			continue
		}
		// Provider lookup with fallback: persona mode populates the
		// map with persona→provider; legacy mode usually doesn't, in
		// which case the agent name IS the provider (claude/codex/
		// gemini). Either way we always have a non-empty provider
		// string, which the schema requires.
		provider, ok := meta.ProviderForPersona[r.Agent]
		if !ok || provider == "" {
			provider = r.Agent
		}

		for _, f := range r.Findings {
			reasoning := nullableString(f.Reasoning)
			confidence := nullableConfidence(f.Confidence)

			if _, err := stmt.Exec(
				meta.RunID,
				meta.CodebaseFP,
				meta.Preset,
				provider,
				r.Agent,
				string(f.Severity),
				f.File,
				f.LineRange,
				f.Summary,
				reasoning,
				confidence,
			); err != nil {
				_ = tx.Rollback()
				return 0, fmt.Errorf("insert finding (%s/%s): %w", r.Agent, f.Summary, err)
			}
			written++
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit: %w", err)
	}
	return written, nil
}

// nullableString converts an empty string to a SQL NULL. Spec marks
// reasoning as NULLABLE because not every agent emits reasoning; we
// preserve the distinction in the DB so downstream queries can tell
// "agent omitted" from "agent emitted empty string".
func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// nullableConfidence converts the zero value to NULL. Confidence is
// REAL in the schema and NULLABLE; agents that don't emit it leave
// the field at Go's zero (0.0), which we record as NULL rather than
// as a literal 0.0 (which would mean "agent claims no confidence" — a
// different signal). Real zero confidence is exotic enough that we're
// happy to fold it into NULL; if a future spec needs to distinguish,
// Finding.Confidence becomes *float64.
func nullableConfidence(c float64) any {
	if c == 0 {
		return nil
	}
	return c
}

// CountForRun returns the number of finding rows recorded for the
// given run_id. Used by tests + by future `jutsu finding list --run
// <id>` to verify the recorder wrote what it claimed.
func CountForRun(s *Store, runID string) (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM findings WHERE run_id = ?`, runID).Scan(&n)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	return n, nil
}
