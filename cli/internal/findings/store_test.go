package findings

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func statFile(path string) (fs.FileInfo, error) { return os.Stat(path) }

// TestOpen_CreatesFromEmpty covers the cold-start path: no DB file
// exists, parent dir does not exist. Open must create both, apply
// 0001_init.sql, and leave a usable handle.
func TestOpen_CreatesFromEmpty(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "sub", "findings.db")

	store, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close()

	// schema_version row recorded for migration 1.
	var v int
	if err := store.DB().QueryRow(`SELECT version FROM schema_version WHERE version=1`).Scan(&v); err != nil {
		t.Fatalf("schema_version row: %v", err)
	}
	if v != 1 {
		t.Fatalf("schema_version=%d, want 1", v)
	}

	// findings table exists and is empty.
	var n int
	if err := store.DB().QueryRow(`SELECT COUNT(*) FROM findings`).Scan(&n); err != nil {
		t.Fatalf("count findings: %v", err)
	}
	if n != 0 {
		t.Fatalf("fresh DB has %d findings, want 0", n)
	}

	// Indexes exist (sqlite_master is the lookup point).
	for _, idx := range []string{"idx_findings_lookup", "idx_findings_pending"} {
		var name string
		err := store.DB().QueryRow(`SELECT name FROM sqlite_master WHERE type='index' AND name=?`, idx).Scan(&name)
		if err != nil {
			t.Fatalf("index %q missing: %v", idx, err)
		}
	}
}

// TestOpen_ReopenIdempotent re-opens a store that already has the
// schema applied. applyMigrations must be a no-op the second time —
// re-running 0001_init.sql would bomb on the AUTOINCREMENT column.
func TestOpen_ReopenIdempotent(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "findings.db")

	first, err := Open(path)
	if err != nil {
		t.Fatalf("Open #1: %v", err)
	}
	if _, err := first.DB().Exec(
		`INSERT INTO findings(run_id, codebase_fp, preset, provider, persona, severity, file, line_range, summary)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"r1", "fp1", "pr-review", "claude", "default-claude", "high", "main.go", "10-20", "summary"); err != nil {
		t.Fatalf("seed insert: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("close #1: %v", err)
	}

	second, err := Open(path)
	if err != nil {
		t.Fatalf("Open #2: %v", err)
	}
	defer second.Close()

	var n int
	if err := second.DB().QueryRow(`SELECT COUNT(*) FROM findings`).Scan(&n); err != nil {
		t.Fatalf("count after reopen: %v", err)
	}
	if n != 1 {
		t.Fatalf("count=%d after reopen, want 1 (data persisted)", n)
	}

	var versionRows int
	if err := second.DB().QueryRow(`SELECT COUNT(*) FROM schema_version`).Scan(&versionRows); err != nil {
		t.Fatalf("count schema_version: %v", err)
	}
	if versionRows != 1 {
		t.Fatalf("schema_version rows=%d, want 1 (idempotent)", versionRows)
	}
}

// TestOpen_ChmodsTo0600 verifies the file ends up at 0600 even when
// the umask would otherwise produce 0644. Skipped on Windows where
// POSIX mode bits don't apply.
func TestOpen_ChmodsTo0600(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX file mode")
	}
	tmp := t.TempDir()
	path := filepath.Join(tmp, "findings.db")

	store, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close()

	info, err := statFile(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	got := info.Mode().Perm()
	if got != 0o600 {
		t.Fatalf("perm=%o, want 0600", got)
	}
}

// TestDefaultPath_OverrideEnv covers the KAIJUTSU_FINDINGS_DB escape
// hatch the CLI uses for tests and (eventually) the --db flag.
func TestDefaultPath_OverrideEnv(t *testing.T) {
	t.Setenv("KAIJUTSU_FINDINGS_DB", "/tmp/explicit/findings.db")
	got, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath: %v", err)
	}
	if got != "/tmp/explicit/findings.db" {
		t.Fatalf("path=%q, want override", got)
	}
}

// TestDefaultPath_HomeFallback verifies the HOME-based default when no
// env override is set. t.Setenv guarantees both are scoped to this test.
func TestDefaultPath_HomeFallback(t *testing.T) {
	t.Setenv("KAIJUTSU_FINDINGS_DB", "")
	t.Setenv("HOME", "/tmp/fakehome")
	got, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath: %v", err)
	}
	want := filepath.Join("/tmp/fakehome", ".kaijutsu", "findings.db")
	if got != want {
		t.Fatalf("path=%q, want %q", got, want)
	}
}

// TestClose_DoubleSafe — Close on an already-closed Store and on a nil
// Store must both be no-ops; cleanup paths in defer chains call this
// without checking state.
func TestClose_DoubleSafe(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "findings.db")
	store, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close #1: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close #2 (idempotent): %v", err)
	}
	var nilStore *Store
	if err := nilStore.Close(); err != nil {
		t.Fatalf("Close on nil Store: %v", err)
	}
}
