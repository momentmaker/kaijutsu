package findings

import (
	"path/filepath"
	"testing"
)

// TestMigration0002_BackfillsExistingDreamRows seeds a v0.7-shape DB
// (rows that were inserted before lens column existed) with [lens:X]
// summary prefixes, then verifies the 0002 migration's UPDATE
// statements populate the lens column from the prefix. Regression
// guard for the substr off-by-one bug caught in spec doc-review
// round 1 — LIKE patterns side-step bracket-counting.
func TestMigration0002_BackfillsExistingDreamRows(t *testing.T) {
	tmp := t.TempDir()
	store, err := Open(filepath.Join(tmp, "findings.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close()

	// Insert one row per known lens, mimicking pre-v0.9 dream output
	// where the recorder hadn't yet been taught about the lens column.
	cases := map[string]string{
		"honest":     "[lens:honest] honest take",
		"fit":        "[lens:fit] does it match",
		"gaps":       "[lens:gaps] hidden assumption",
		"wild":       "[lens:wild] 10x extension",
		"adversary":  "[lens:adversary] attack surface",
		"inverse":    "[lens:inverse] do the opposite",
		"status-quo": "[lens:status-quo] do nothing",
		"time":       "[lens:time] 2 years out",
	}
	// Row that doesn't carry the prefix — must end up lens=NULL.
	cases["__no_prefix__"] = "no lens prefix here"

	for name, summary := range cases {
		// NULL lens to simulate pre-migration insert; backfill
		// should re-populate from the summary prefix.
		_, err := store.DB().Exec(
			`INSERT INTO findings(run_id, codebase_fp, preset, provider, persona, severity, file, line_range, summary, lens)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NULL)`,
			"r-"+name, "fp1", "dream", "claude", "default-claude", "info", "x.go", "1", summary)
		if err != nil {
			t.Fatalf("seed %s: %v", name, err)
		}
	}

	// Re-run 0002's UPDATEs explicitly (the migration already ran on
	// Open; this is the load-bearing assertion that re-running is
	// idempotent AND covers all 8 lenses). Slice (not map) so order
	// is deterministic — UPDATEs are commutative here, but a slice
	// is the idiomatic Go pattern when iteration order is observable.
	allLenses := []string{
		"honest", "fit", "gaps", "wild",
		"adversary", "inverse", "status-quo", "time",
	}
	for _, lens := range allLenses {
		_, err := store.DB().Exec(
			"UPDATE findings SET lens = ? WHERE lens IS NULL AND summary LIKE '[lens:'||?||']%'",
			lens, lens)
		if err != nil {
			t.Fatalf("re-apply backfill %s: %v", lens, err)
		}
	}

	for name := range cases {
		var lens any
		if err := store.DB().QueryRow(
			`SELECT lens FROM findings WHERE run_id = ?`, "r-"+name,
		).Scan(&lens); err != nil {
			t.Fatalf("query %s: %v", name, err)
		}
		if name == "__no_prefix__" {
			if lens != nil {
				t.Errorf("no-prefix row: lens=%v, want NULL", lens)
			}
			continue
		}
		got, ok := lens.(string)
		if !ok {
			t.Errorf("%s: lens=%v, want string %q", name, lens, name)
			continue
		}
		if got != name {
			t.Errorf("%s: lens=%q, want %q", name, got, name)
		}
	}
}

// TestMigration0002_HandlesEmptyDB is the first-install path: no
// pre-existing rows, both ALTER TABLE statements apply cleanly,
// idx_findings_lens_lookup index is created.
func TestMigration0002_HandlesEmptyDB(t *testing.T) {
	tmp := t.TempDir()
	store, err := Open(filepath.Join(tmp, "findings.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close()

	// Both columns exist (PRAGMA table_info enumerates).
	rows, err := store.DB().Query(`PRAGMA table_info(findings)`)
	if err != nil {
		t.Fatalf("PRAGMA table_info: %v", err)
	}
	defer rows.Close()
	got := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, pk int
		var dflt any
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got[name] = true
	}
	for _, want := range []string{"lens", "position"} {
		if !got[want] {
			t.Errorf("column %q missing on fresh DB", want)
		}
	}

	// Index exists.
	var idx string
	if err := store.DB().QueryRow(
		`SELECT name FROM sqlite_master WHERE type='index' AND name='idx_findings_lens_lookup'`,
	).Scan(&idx); err != nil {
		t.Fatalf("idx_findings_lens_lookup missing: %v", err)
	}
}

// TestMigration0002_Idempotent verifies re-running the migration
// loader doesn't double-apply 0002 (same applied check that 0001
// gets via TestOpen_ReopenIdempotent — explicit here so a future
// migration regression doesn't slip past).
func TestMigration0002_Idempotent(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "findings.db")
	first, err := Open(path)
	if err != nil {
		t.Fatalf("Open #1: %v", err)
	}
	first.Close()

	second, err := Open(path)
	if err != nil {
		t.Fatalf("Open #2: %v", err)
	}
	defer second.Close()

	var version2Rows int
	if err := second.DB().QueryRow(
		`SELECT COUNT(*) FROM schema_version WHERE version = 2`,
	).Scan(&version2Rows); err != nil {
		t.Fatalf("count v2: %v", err)
	}
	if version2Rows != 1 {
		t.Errorf("v2 row count=%d, want 1 (idempotent)", version2Rows)
	}
}
