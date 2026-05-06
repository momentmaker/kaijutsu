// Package findings owns the local SQLite store that backs v0.7 quality
// fingerprinting. Every swarm finding is recorded here so the synthesizer
// can compute per-tuple (provider, persona, preset, codebase) precision
// and downweight noisy combinations on the next run.
//
// Design pointers:
//   - Spec:  docs/specs/2026-05-06-v0.7.0-quality-fingerprinting.md
//   - ADR:   docs/decisions/2026-05-06-quality-fingerprinting.md
//
// The store is local-only by design; nothing in this package opens a
// network socket. Driver is modernc.org/sqlite (pure Go, no CGo) so the
// release binary stays cross-compilable.
package findings

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Store is the handle for the findings database. Safe for concurrent use
// by multiple goroutines — modernc.org/sqlite serializes writes through
// its own mutex while WAL mode keeps reads non-blocking.
type Store struct {
	db   *sql.DB
	path string
}

// DefaultPath returns the canonical findings.db location:
// $HOME/.kaijutsu/findings.db. Override with KAIJUTSU_FINDINGS_DB for
// tests; honoring HOME (which testing.T.Setenv supports) is what makes
// the table-driven tests deterministic.
func DefaultPath() (string, error) {
	if override := os.Getenv("KAIJUTSU_FINDINGS_DB"); override != "" {
		return override, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, ".kaijutsu", "findings.db"), nil
}

// Open returns a ready-to-use Store backed by the file at path. The
// parent directory is created with 0700 if missing; the DB file is
// chmod'd to 0600 on every Open (re-asserted in case the user copied
// the file in from elsewhere with looser perms — findings text may
// contain quoted source from private repos).
//
// Pending migrations are applied before Open returns; callers can
// assume the schema matches the current build.
func Open(path string) (*Store, error) {
	if path == "" {
		return nil, errors.New("findings: empty path")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create findings dir: %w", err)
	}

	// _journal=WAL gives concurrent readers w/ a single writer — fine
	// for the swarm pattern (write batch after fan-out, read at the
	// start of the next dispatch).
	// _busy_timeout=5000ms covers the rare overlap when two `jutsu`
	// processes touch the DB simultaneously (e.g. a long-running swarm
	// alongside `jutsu finding stats`).
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %q: %w", path, err)
	}
	// modernc.org/sqlite supports concurrent reads, but only one
	// writer at a time. Keeping max-open low avoids busy errors that
	// would otherwise look transient under load.
	db.SetMaxOpenConns(4)

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite %q: %w", path, err)
	}

	if err := applyMigrations(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("apply migrations: %w", err)
	}

	// Re-assert restrictive perms after first-time creation. SQLite
	// creates the file via the standard libc open() with the process
	// umask, which on many shells is 0022 → 0644. Force 0600.
	if err := os.Chmod(path, 0o600); err != nil && !os.IsNotExist(err) {
		_ = db.Close()
		return nil, fmt.Errorf("chmod findings.db: %w", err)
	}

	return &Store{db: db, path: path}, nil
}

// Close releases the underlying connection pool. Safe to call multiple
// times; subsequent calls return nil.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	err := s.db.Close()
	s.db = nil
	return err
}

// DB exposes the underlying *sql.DB for sibling files in this package
// (recorder, weighter). Not exported beyond package boundary.
func (s *Store) DB() *sql.DB { return s.db }

// Path returns the resolved on-disk location. Useful for stats output
// ("findings stored at: …") and for diagnostics.
func (s *Store) Path() string { return s.path }

// migrationName matches NNNN_<slug>.sql where NNNN is zero-padded. The
// loader sorts by NNNN to derive apply order, so a file named
// "0010_foo.sql" applies after "0001_init.sql" regardless of FS walk
// order. Anchored at start; the trailing ".sql" is checked separately.
var migrationName = regexp.MustCompile(`^(\d{4})_[a-z0-9_-]+\.sql$`)

// applyMigrations is forward-only: every NNNN_*.sql whose version is
// not yet recorded in schema_version runs, in NNNN order, inside a
// single transaction per file. Failure rolls back that file's tx and
// leaves prior migrations applied — the next Open will retry the
// failed file.
//
// schema_version is bootstrapped via 0001_init.sql so the first call
// must allow the migrations table not to exist yet.
func applyMigrations(db *sql.DB) error {
	files, err := loadMigrationFiles()
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return errors.New("no migration files embedded — build is broken")
	}

	applied, err := loadAppliedVersions(db)
	if err != nil {
		return err
	}

	for _, m := range files {
		if applied[m.version] {
			continue
		}
		if err := runMigration(db, m); err != nil {
			return fmt.Errorf("migration %04d (%s): %w", m.version, m.name, err)
		}
	}
	return nil
}

type migration struct {
	version int
	name    string
	sql     string
}

func loadMigrationFiles() ([]migration, error) {
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return nil, fmt.Errorf("read embedded migrations: %w", err)
	}
	out := make([]migration, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		m := migrationName.FindStringSubmatch(e.Name())
		if m == nil {
			return nil, fmt.Errorf("migration %q: filename must match NNNN_<slug>.sql", e.Name())
		}
		ver, err := strconv.Atoi(m[1])
		if err != nil {
			return nil, fmt.Errorf("migration %q: parse version: %w", e.Name(), err)
		}
		body, err := fs.ReadFile(migrationsFS, filepath.ToSlash(filepath.Join("migrations", e.Name())))
		if err != nil {
			return nil, fmt.Errorf("read migration %q: %w", e.Name(), err)
		}
		out = append(out, migration{version: ver, name: e.Name(), sql: string(body)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].version < out[j].version })

	// Refuse to run if two files share a version — that's almost
	// certainly a copy-paste mistake during a v0.x stage and will
	// cause undefined ordering on filesystems whose ReadDir is not
	// lexical.
	for i := 1; i < len(out); i++ {
		if out[i].version == out[i-1].version {
			return nil, fmt.Errorf("duplicate migration version %04d: %s and %s", out[i].version, out[i-1].name, out[i].name)
		}
	}
	return out, nil
}

func loadAppliedVersions(db *sql.DB) (map[int]bool, error) {
	applied := map[int]bool{}
	rows, err := db.Query(`SELECT version FROM schema_version`)
	if err != nil {
		// First-run: the table itself is created by 0001_init.sql, so
		// "no such table" means nothing has been applied yet.
		if isNoSuchTable(err) {
			return applied, nil
		}
		return nil, fmt.Errorf("query schema_version: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, fmt.Errorf("scan schema_version: %w", err)
		}
		applied[v] = true
	}
	return applied, rows.Err()
}

func runMigration(db *sql.DB, m migration) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	if _, err := tx.Exec(m.sql); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("exec sql: %w", err)
	}
	if _, err := tx.Exec(`INSERT INTO schema_version(version) VALUES (?)`, m.version); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("record version: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

// isNoSuchTable matches modernc.org/sqlite's error string for a missing
// table. The driver does not expose a typed sentinel for this, and the
// generic sqlite3 error code (1) is shared with too many other failures
// to be useful. String-match is the documented pattern in the driver's
// own examples.
func isNoSuchTable(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "no such table")
}
