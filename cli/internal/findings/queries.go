package findings

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Row is the in-memory shape of a single findings row. Mirrors the
// schema; nullable columns become *string / *float64 so callers can
// distinguish "agent omitted" from "agent emitted empty".
type Row struct {
	ID           int64
	RunID        string
	CodebaseFP   string
	Preset       string
	Provider     string
	Persona      string
	Severity     string
	File         string
	LineRange    string
	Summary      string
	Reasoning    *string
	Confidence   *float64
	CreatedAt    time.Time
	UserAction   *string // "accepted" | "dismissed" | nil (pending)
	ActionAt     *time.Time
	ActionReason *string
}

// ListOpts narrows the rows returned by ListFindings. Empty defaults
// match the spec's `jutsu finding list` UX:
//   - CodebaseFP set, RunID empty, PendingOnly false, AllCodebases
//     false → most recent run's findings for this codebase.
//   - RunID set → that run's findings (CodebaseFP ignored).
//   - PendingOnly → user_action IS NULL only.
//   - AllCodebases → ignore CodebaseFP filter.
type ListOpts struct {
	CodebaseFP    string
	RunID         string
	PendingOnly   bool
	AllCodebases  bool
	Limit         int // 0 = no limit (use sparingly — there is no auto-prune)
}

// ListFindings returns rows ordered by id desc (most-recent first).
// When RunID is unset and AllCodebases is false, scopes to the most
// recent run for CodebaseFP — defined as MAX(run_id) by created_at,
// so two runs with the same key collapse into one (matches the cache
// model where one cache key == one run).
func ListFindings(s *Store, opts ListOpts) ([]Row, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("queries: nil store")
	}

	q := `SELECT id, run_id, codebase_fp, preset, provider, persona,
	             severity, file, line_range, summary, reasoning,
	             confidence, created_at, user_action, action_at,
	             action_reason
	      FROM findings`
	clauses := []string{}
	args := []any{}

	switch {
	case opts.RunID != "":
		clauses = append(clauses, "run_id = ?")
		args = append(args, opts.RunID)
	case !opts.AllCodebases && opts.CodebaseFP != "":
		// Scope to most recent run_id for the codebase. ORDER BY
		// created_at DESC, id DESC: SQLite's CURRENT_TIMESTAMP has
		// second precision, so two runs within the same second tie
		// on created_at — id (autoincrement) breaks the tie since
		// later runs have higher ids.
		clauses = append(clauses,
			`run_id = (
				SELECT run_id FROM findings
				WHERE codebase_fp = ?
				ORDER BY created_at DESC, id DESC LIMIT 1
			)`,
			"codebase_fp = ?",
		)
		args = append(args, opts.CodebaseFP, opts.CodebaseFP)
	}

	if opts.PendingOnly {
		clauses = append(clauses, "user_action IS NULL")
	}

	if len(clauses) > 0 {
		q += " WHERE " + joinAnd(clauses)
	}
	q += " ORDER BY id DESC"
	if opts.Limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", opts.Limit)
	}

	return scanRows(s.db.Query(q, args...))
}

// GetByID fetches a single row. Returns sql.ErrNoRows when missing —
// callers MUST check for this so the cross-codebase guard's "id not
// found" error is distinguishable from "id exists in a different
// codebase".
func GetByID(s *Store, id int64) (*Row, error) {
	rows, err := s.db.Query(`SELECT id, run_id, codebase_fp, preset, provider, persona,
		         severity, file, line_range, summary, reasoning,
		         confidence, created_at, user_action, action_at,
		         action_reason
		  FROM findings WHERE id = ? LIMIT 1`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, sql.ErrNoRows
	}
	r, err := scanOne(rows)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// SetAction marks a finding as accepted or dismissed. The action arg
// must be exactly "accepted" or "dismissed" — caller validates before
// calling. action_at is set to time.Now() on the writing side (not
// CURRENT_TIMESTAMP) so we can override in tests via a fixed clock if
// ever needed.
func SetAction(s *Store, id int64, action, reason string) error {
	if action != "accepted" && action != "dismissed" {
		return fmt.Errorf("invalid action %q (want accepted|dismissed)", action)
	}
	var reasonArg any
	if reason != "" {
		reasonArg = reason
	}
	res, err := s.db.Exec(
		`UPDATE findings SET user_action = ?, action_at = ?, action_reason = ? WHERE id = ?`,
		action, time.Now().UTC(), reasonArg, id,
	)
	if err != nil {
		return fmt.Errorf("update action: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// TupleStats aggregates counts per (provider, persona, preset) for a
// single codebase. Pending = user_action IS NULL.
type TupleStats struct {
	Provider  string
	Persona   string
	Preset    string
	Accepted  int
	Dismissed int
	Pending   int
}

// Actioned returns accepted+dismissed — the value the weight algorithm
// gates on (cold/bootstrap/mature thresholds).
func (t TupleStats) Actioned() int { return t.Accepted + t.Dismissed }

// Precision returns accepted/(accepted+dismissed) over actioned rows.
// Returns 0 when no actioned rows so callers must check Actioned()
// before treating this as a meaningful weight.
func (t TupleStats) Precision() float64 {
	a := t.Actioned()
	if a == 0 {
		return 0
	}
	return float64(t.Accepted) / float64(a)
}

// StatsByTuple returns one row per (provider, persona, preset) tuple
// for the given codebase, sorted by precision desc. Used by `jutsu
// finding stats` to render the per-tuple table. AllCodebases=true
// drops the codebase filter — useful for `--all-codebases` flag.
func StatsByTuple(s *Store, codebaseFP string, allCodebases bool) ([]TupleStats, error) {
	q := `SELECT provider, persona, preset,
	             SUM(CASE WHEN user_action='accepted' THEN 1 ELSE 0 END)  AS accepted,
	             SUM(CASE WHEN user_action='dismissed' THEN 1 ELSE 0 END) AS dismissed,
	             SUM(CASE WHEN user_action IS NULL    THEN 1 ELSE 0 END) AS pending
	      FROM findings`
	args := []any{}
	if !allCodebases {
		q += " WHERE codebase_fp = ?"
		args = append(args, codebaseFP)
	}
	q += " GROUP BY provider, persona, preset"

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []TupleStats{}
	for rows.Next() {
		var t TupleStats
		if err := rows.Scan(&t.Provider, &t.Persona, &t.Preset,
			&t.Accepted, &t.Dismissed, &t.Pending); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// ClearOpts targets rows for `jutsu finding clear`. All filters AND
// together. CodebaseFP empty + OlderThan zero = delete EVERYTHING
// (the clear command refuses without --yes, so this isn't a stealth
// nuclear button).
type ClearOpts struct {
	CodebaseFP string
	OlderThan  time.Duration // 0 = no age filter
}

// CountClearable returns the number of rows that ClearOpts WOULD
// delete. Used by `jutsu finding clear --dry-run` (the default unless
// --yes is passed).
func CountClearable(s *Store, opts ClearOpts) (int, error) {
	q, args := buildClearWhere("SELECT COUNT(*) FROM findings", opts)
	var n int
	if err := s.db.QueryRow(q, args...).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

// Clear deletes rows matching ClearOpts and runs VACUUM to reclaim
// disk. Returns rows-deleted; runs the delete in a single transaction
// so a partial failure rolls back. VACUUM cannot run inside a tx (it
// requires exclusive lock on the entire DB), so we commit then VACUUM.
func Clear(s *Store, opts ClearOpts) (int, error) {
	q, args := buildClearWhere("DELETE FROM findings", opts)

	tx, err := s.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin: %w", err)
	}
	res, err := tx.Exec(q, args...)
	if err != nil {
		_ = tx.Rollback()
		return 0, fmt.Errorf("delete: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		_ = tx.Rollback()
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit: %w", err)
	}

	// VACUUM after commit — required by SQLite (cannot run in tx).
	// Failing here is non-fatal: the rows are gone, the disk just
	// isn't reclaimed yet. Return the count + error so caller can warn.
	if _, err := s.db.Exec(`VACUUM`); err != nil {
		return int(n), fmt.Errorf("vacuum: %w (rows already deleted)", err)
	}
	return int(n), nil
}

func buildClearWhere(prefix string, opts ClearOpts) (string, []any) {
	clauses := []string{}
	args := []any{}
	if opts.CodebaseFP != "" {
		clauses = append(clauses, "codebase_fp = ?")
		args = append(args, opts.CodebaseFP)
	}
	if opts.OlderThan > 0 {
		cutoff := time.Now().UTC().Add(-opts.OlderThan)
		clauses = append(clauses, "created_at < ?")
		args = append(args, cutoff)
	}
	q := prefix
	if len(clauses) > 0 {
		q += " WHERE " + joinAnd(clauses)
	}
	return q, args
}

// AllForCodebase returns every row for codebase_fp, ordered by id
// asc. Used by `jutsu finding export`. No limit — export is meant to
// be a complete dump.
func AllForCodebase(s *Store, codebaseFP string, allCodebases bool) ([]Row, error) {
	q := `SELECT id, run_id, codebase_fp, preset, provider, persona,
	             severity, file, line_range, summary, reasoning,
	             confidence, created_at, user_action, action_at,
	             action_reason
	      FROM findings`
	args := []any{}
	if !allCodebases {
		q += " WHERE codebase_fp = ?"
		args = append(args, codebaseFP)
	}
	q += " ORDER BY id ASC"
	return scanRows(s.db.Query(q, args...))
}

// --- internals ---

func joinAnd(parts []string) string {
	out := parts[0]
	for _, p := range parts[1:] {
		out += " AND " + p
	}
	return out
}

func scanRows(rows *sql.Rows, err error) ([]Row, error) {
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Row{}
	for rows.Next() {
		r, err := scanOne(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func scanOne(rows *sql.Rows) (Row, error) {
	var r Row
	var (
		reasoning, userAction, actionReason sql.NullString
		confidence                          sql.NullFloat64
		actionAt                            sql.NullTime
	)
	if err := rows.Scan(
		&r.ID, &r.RunID, &r.CodebaseFP, &r.Preset, &r.Provider, &r.Persona,
		&r.Severity, &r.File, &r.LineRange, &r.Summary,
		&reasoning, &confidence, &r.CreatedAt,
		&userAction, &actionAt, &actionReason,
	); err != nil {
		return Row{}, err
	}
	if reasoning.Valid {
		r.Reasoning = &reasoning.String
	}
	if confidence.Valid {
		r.Confidence = &confidence.Float64
	}
	if userAction.Valid {
		r.UserAction = &userAction.String
	}
	if actionAt.Valid {
		t := actionAt.Time
		r.ActionAt = &t
	}
	if actionReason.Valid {
		r.ActionReason = &actionReason.String
	}
	return r, nil
}
