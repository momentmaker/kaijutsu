package eval

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// LockFileName is the workspace-level lock file. v0.10 single-skill
// invariant: only one eval run per workspace at a time. Concurrent
// invocations error so partial-iteration-N artifacts never get
// overwritten by a parallel runner.
const LockFileName = ".eval-lock"

// StaleLockMaxAge is the upper bound on lock-file mtime before the
// next runner considers the lock abandoned. CI runners that crash
// mid-eval leave a stale lock; the next run picks up cleanly after
// this window. Pinned at 2 hours to comfortably outlast the longest
// expected eval suite (10 evals × 2 sides × 5 assertions ≈ minutes,
// not hours; 2h gives 30× margin for slow models or large suites).
const StaleLockMaxAge = 2 * time.Hour

// AcquireLock writes the workspace lock file with the current PID +
// unix timestamp. Returns an error if a non-stale lock already
// holds the workspace.
//
// Stale-lock policy (spec §2):
//  1. PID in lock file no longer alive → stale, auto-clear
//  2. Lock file mtime > StaleLockMaxAge ago → stale, auto-clear
//  3. Otherwise → blocked
//
// Stale-clear is not silent: warning is written to stderrW so the
// operator sees that a prior crash left state to recover.
func AcquireLock(workspace string, stderrW Writer) (release func(), err error) {
	if err := os.MkdirAll(workspace, 0o700); err != nil {
		return nil, fmt.Errorf("create workspace: %w", err)
	}
	path := filepath.Join(workspace, LockFileName)
	if err := tryClearStaleLock(path, stderrW); err != nil {
		return nil, err
	}
	body := fmt.Sprintf("%d\n%d\n", os.Getpid(), time.Now().Unix())
	// O_EXCL|O_CREATE is the atomic-claim primitive. Second concurrent
	// invocation gets EEXIST and surfaces the "another eval is
	// running" message.
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		if os.IsExist(err) {
			return nil, fmt.Errorf("another eval is running in this workspace (lock at %s)", path)
		}
		return nil, fmt.Errorf("create lock: %w", err)
	}
	if _, err := f.WriteString(body); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return nil, fmt.Errorf("write lock body: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(path)
		return nil, fmt.Errorf("close lock: %w", err)
	}
	return func() {
		_ = os.Remove(path)
	}, nil
}

// Writer is a minimal interface satisfied by *os.File (for stderr)
// and *bytes.Buffer (for tests). Avoids pulling io into the eval
// package's primary surface for a single warning channel.
type Writer interface {
	Write(p []byte) (int, error)
}

// tryClearStaleLock checks an existing lock for staleness (per the
// PID + mtime policy) and removes it if abandoned. Non-stale locks
// stay in place; the caller will hit O_EXCL conflict and surface
// the "another eval is running" message.
func tryClearStaleLock(path string, stderrW Writer) error {
	stat, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // no lock — nothing to clear
		}
		return fmt.Errorf("stat lock: %w", err)
	}
	// Mtime check first — cheap. If lock is older than 2h it's
	// stale regardless of PID liveness (long-dead process whose PID
	// got recycled by the OS would erroneously appear alive
	// otherwise).
	if time.Since(stat.ModTime()) > StaleLockMaxAge {
		fmt.Fprintf(stderrW, "warning: stale lock at %s (mtime %s ago); clearing\n", path, time.Since(stat.ModTime()).Truncate(time.Second))
		return os.Remove(path)
	}
	// PID liveness check.
	pid, err := readLockPID(path)
	if err != nil {
		// Unparseable lock body — treat as stale to avoid permanent
		// blocks from corrupted state.
		fmt.Fprintf(stderrW, "warning: lock file %s unparseable; clearing\n", path)
		return os.Remove(path)
	}
	if !pidAlive(pid) {
		fmt.Fprintf(stderrW, "warning: stale lock at %s (pid %d not alive); clearing\n", path, pid)
		return os.Remove(path)
	}
	// Live PID + recent mtime — genuine conflict; let O_EXCL fire.
	return nil
}

// readLockPID parses the first line of the lock file as a PID.
// Format: `<pid>\n<unix-ts>\n`. Returns 0 + error on malformed body.
func readLockPID(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	lines := strings.SplitN(string(data), "\n", 2)
	if len(lines) == 0 || strings.TrimSpace(lines[0]) == "" {
		return 0, fmt.Errorf("empty lock body")
	}
	pid, err := strconv.Atoi(strings.TrimSpace(lines[0]))
	if err != nil {
		return 0, fmt.Errorf("parse pid: %w", err)
	}
	return pid, nil
}

// pidAlive reports whether a PID corresponds to a running process.
// On unix, sending signal 0 to a PID checks existence without
// affecting the target. Returns false for pid <= 0 (sentinel /
// invalid input).
func pidAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// Signal 0 = "test if delivery is possible." err == nil means
	// the process exists; ESRCH means it doesn't; EPERM means it
	// exists but we can't signal (still alive for our purposes).
	err = proc.Signal(syscall.Signal(0))
	if err == nil {
		return true
	}
	if err == syscall.EPERM {
		return true
	}
	return false
}
