package findings

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

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
// Returns (written, skipped, error):
//   - written: rows successfully INSERTed
//   - skipped: rows excluded because they failed dream-preset lens-
//     prefix validation (dream-only — non-dream presets always have
//     skipped == 0)
//   - error: tx-level failure
//
// Skips AgentResult entries with Err set (errored agents have nothing
// useful to record) and empty Findings slices. For dream preset:
// also skips findings whose summary doesn't start with [lens:<known>]
// or whose reasoning doesn't start with load_bearing: true|false.
// Malformed dream rows would pollute the v0.9 schema migration source
// data; better to drop them at recorder time + surface the skip count
// to the caller for stderr logging.
//
// Best-effort caller pattern: the swarm pipeline wraps this in a
// log-and-continue so a bad DB never blocks the markdown render.
func RecordRun(s *Store, meta RunMeta, results []swarm.AgentResult) (int, int, error) {
	if s == nil || s.db == nil {
		return 0, 0, errors.New("recorder: nil store")
	}
	if meta.RunID == "" {
		return 0, 0, errors.New("recorder: RunID required")
	}
	if meta.CodebaseFP == "" {
		return 0, 0, errors.New("recorder: CodebaseFP required")
	}
	if meta.Preset == "" {
		return 0, 0, errors.New("recorder: Preset required")
	}

	tx, err := s.db.Begin()
	if err != nil {
		return 0, 0, fmt.Errorf("begin: %w", err)
	}
	stmt, err := tx.Prepare(`
		INSERT INTO findings(
			run_id, codebase_fp, preset, provider, persona,
			severity, file, line_range, summary, reasoning, confidence
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		_ = tx.Rollback()
		return 0, 0, fmt.Errorf("prepare: %w", err)
	}
	defer stmt.Close()

	isDream := meta.Preset == "dream"
	written, skipped := 0, 0
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
			// Dream-preset rows MUST follow the lens-in-summary
			// encoding: summary starts with [lens:<known>] and
			// reasoning starts with load_bearing: true|false. Drop
			// malformed rows so the v0.9 schema migration source
			// data stays clean. Skipped count surfaces to the
			// caller for stderr logging.
			if isDream && !ValidDreamFinding(f.Summary, f.Reasoning) {
				skipped++
				continue
			}
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
				return 0, 0, fmt.Errorf("insert finding (%s/%s): %w", r.Agent, f.Summary, err)
			}
			written++
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, 0, fmt.Errorf("commit: %w", err)
	}
	return written, skipped, nil
}

// dreamLensWhitelist matches the 8 canonical lens names. Closed set
// in v0.8.0+. ValidDreamFinding rejects anything outside this list
// so the lens-in-summary encoding stays parseable for the v0.9
// schema migration.
var dreamLensWhitelist = map[string]bool{
	"honest": true, "fit": true, "gaps": true, "wild": true,
	"adversary": true, "inverse": true, "status-quo": true, "time": true,
}

// dreamSummaryPattern matches the [lens:<name>] prefix at start of
// summary. Whitespace after the closing bracket is tolerated:
// 0+ whitespace chars then any non-whitespace. Trailing-content
// requirement keeps "[lens:gaps]" alone (no thought) rejected.
// Whitelist enforcement happens after capture.
var dreamSummaryPattern = regexp.MustCompile(`^\[lens:([a-z][a-z-]+[a-z])\]\s*\S`)

// ValidDreamFinding reports whether a dream-preset finding's summary
// + reasoning fields conform to the lens-in-summary encoding.
//
//   - Summary must start with [lens:<name>] where <name> is in the
//     8-lens whitelist.
//   - Reasoning must start with "load_bearing: true" or
//     "load_bearing: false" (case-insensitive on the bool).
//
// Empty reasoning is ALLOWED (the recorder converts to NULL); only
// non-empty reasoning is checked for the load_bearing prefix. This
// matches early dream-preset behavior where some agents omit
// reasoning entirely.
func ValidDreamFinding(summary, reasoning string) bool {
	m := dreamSummaryPattern.FindStringSubmatch(summary)
	if m == nil {
		return false
	}
	if !dreamLensWhitelist[m[1]] {
		return false
	}
	if reasoning == "" {
		return true
	}
	// Tolerate variable whitespace around the colon — models emit
	// "load_bearing:true" / "load_bearing:  true" / "load_bearing: TRUE"
	// indistinguishably. Strip prefix shape, then check the bool word
	// case-insensitively. Anything past the bool is the actual reason
	// text (free-form).
	lower := strings.ToLower(strings.TrimSpace(reasoning))
	if !strings.HasPrefix(lower, "load_bearing") {
		return false
	}
	rest := strings.TrimSpace(strings.TrimPrefix(lower, "load_bearing"))
	if !strings.HasPrefix(rest, ":") {
		return false
	}
	rest = strings.TrimSpace(strings.TrimPrefix(rest, ":"))
	return strings.HasPrefix(rest, "true") || strings.HasPrefix(rest, "false")
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
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM findings WHERE run_id = ?`, runID).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}
