// stats.go — v0.14.0 preset usage tracking query surface.
//
// Stats counts findings rows over a time window, grouped by an axis
// (preset / persona / provider). Pure data: no source resolution
// (built-in vs user) — that's the cli layer's job (mediated through
// swarm.DefaultRegistry to avoid an import cycle from findings →
// swarm).
//
// Spec: docs/specs/2026-05-09-v0.14.0-persona-browse-and-preset-usage.md
// (Decisions #6, #7, #8).
package findings

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// StatsBy enumerates the group-by axes Stats supports. Mirrors the
// `--by` flag values on `jutsu finding stats`.
type StatsBy string

const (
	StatsByPreset   StatsBy = "preset"
	StatsByPersona  StatsBy = "persona"
	StatsByProvider StatsBy = "provider"
)

// StatsOpts narrows the Stats result set.
//
//   - By:           group-by axis; required (default StatsByPreset).
//   - Since:        only count rows with created_at >= now-Since.
//                   Zero = all-time. Negative = error (rejected by
//                   Stats so a programmatic caller misuse fails fast
//                   rather than silently behaving as all-time).
//   - CodebaseFP:   if non-empty AND !AllCodebases, restrict to rows
//                   from this codebase.
//   - AllCodebases: bypass codebase filter entirely.
type StatsOpts struct {
	By           StatsBy
	Since        time.Duration
	CodebaseFP   string
	AllCodebases bool
}

// StatsRow is one aggregated count: the group label + the count.
// `Group` is the raw column value (NULL coalesces to "(unknown)").
// Source resolution is NOT computed here — cli/finding_stats.go
// enriches by querying swarm.DefaultRegistry post-hoc, so this layer
// stays free of the swarm import + the abstraction-leak the plan
// doc-review flagged.
type StatsRow struct {
	Group string `json:"group"`
	Count int64  `json:"count"`
}

// Stats returns aggregated counts ordered by Count desc, then Group
// asc (deterministic ties for stable diffs). Returns empty slice on
// no matching rows; never nil.
//
// All three group-by columns (preset / persona / provider) are
// NOT NULL in the schema (0001_init.sql) so no COALESCE is needed
// at this layer. If a future migration relaxes the constraint, add
// the COALESCE then.
func Stats(s *Store, opts StatsOpts) ([]StatsRow, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("findings: nil store")
	}
	if opts.Since < 0 {
		return nil, fmt.Errorf("findings: stats Since must be >= 0 (got %v); use 0 for all-time", opts.Since)
	}
	col, err := statsByColumn(opts.By)
	if err != nil {
		return nil, err
	}

	q := fmt.Sprintf(`SELECT %s AS grp, COUNT(*) AS c
	                  FROM findings`, col)
	clauses := []string{}
	args := []any{}

	if !opts.AllCodebases && opts.CodebaseFP != "" {
		clauses = append(clauses, "codebase_fp = ?")
		args = append(args, opts.CodebaseFP)
	}
	if opts.Since > 0 {
		// Stored as TIMESTAMP DEFAULT CURRENT_TIMESTAMP (UTC ISO-8601 in
		// modernc/sqlite). Comparing against a Go time.Time literal works
		// directly via the driver's time-string conversion.
		clauses = append(clauses, "created_at >= ?")
		args = append(args, time.Now().UTC().Add(-opts.Since))
	}
	if len(clauses) > 0 {
		q += " WHERE " + strings.Join(clauses, " AND ")
	}
	q += " GROUP BY grp ORDER BY c DESC, grp ASC"

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("findings: stats query: %w", err)
	}
	defer rows.Close()

	out := []StatsRow{}
	for rows.Next() {
		var r StatsRow
		if err := rows.Scan(&r.Group, &r.Count); err != nil {
			return nil, fmt.Errorf("findings: stats scan: %w", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("findings: stats rows: %w", err)
	}
	return out, nil
}

// statsByColumn maps the StatsBy enum to a literal column name.
// Returns error on unknown axis. Returning the literal (not a `?`
// arg) is intentional — group-by columns can't be parameterized in
// SQLite — but the input is guarded by enum match so the literal
// can never become user input.
func statsByColumn(by StatsBy) (string, error) {
	switch by {
	case StatsByPreset, "":
		return "preset", nil
	case StatsByPersona:
		return "persona", nil
	case StatsByProvider:
		return "provider", nil
	}
	return "", fmt.Errorf("findings: unknown stats axis %q (want preset|persona|provider)", by)
}

// ParseSinceDuration accepts the `--since` shorthand: `<N>d`, `<N>w`,
// `<N>mo`, `<N>y`. Anything else falls through to time.ParseDuration
// (so `30s` / `5m` / `2h` still work).
//
// Empty string → 0 (all-time). Zero / negative → error.
//
//   - 1d  = 24h
//   - 1w  = 7  * 24h
//   - 1mo = 30 * 24h (calendar-month-agnostic; matches typical CLI
//                     ergonomic; finer grain is a v0.14.x option)
//   - 1y  = 365 * 24h
func ParseSinceDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	// Try shorthand suffixes first. Order matters: `mo` must be tried
	// before `m` (Go's time.ParseDuration owns `m` for minutes).
	for _, suf := range []struct {
		s string
		d time.Duration
	}{
		{"mo", 30 * 24 * time.Hour},
		{"d", 24 * time.Hour},
		{"w", 7 * 24 * time.Hour},
		{"y", 365 * 24 * time.Hour},
	} {
		if num, ok := strings.CutSuffix(s, suf.s); ok {
			n, err := strconv.Atoi(strings.TrimSpace(num))
			if err != nil {
				return 0, fmt.Errorf("findings: --since %q: bad count before %q: %w", s, suf.s, err)
			}
			if n <= 0 {
				return 0, fmt.Errorf("findings: --since %q must be positive", s)
			}
			return time.Duration(n) * suf.d, nil
		}
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("findings: --since %q: %w (use 30s | 5m | 2h | 7d | 4w | 3mo | 1y)", s, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("findings: --since %q must be positive", s)
	}
	return d, nil
}
