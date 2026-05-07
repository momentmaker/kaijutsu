package eval

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestLock_AcquireAndRelease covers the happy path: lock acquired
// successfully, release function clears the lock file.
func TestLock_AcquireAndRelease(t *testing.T) {
	tmp := t.TempDir()
	stderr := &bytes.Buffer{}
	release, err := AcquireLock(tmp, stderr)
	if err != nil {
		t.Fatalf("AcquireLock: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmp, LockFileName)); err != nil {
		t.Errorf("lock file should exist after acquire: %v", err)
	}
	release()
	if _, err := os.Stat(filepath.Join(tmp, LockFileName)); !os.IsNotExist(err) {
		t.Errorf("lock file should be removed after release; got: %v", err)
	}
}

// TestLock_ConcurrentInvocationErrors covers the EEXIST path: a
// second AcquireLock against the same workspace while the first
// holds errors with "another eval is running".
func TestLock_ConcurrentInvocationErrors(t *testing.T) {
	tmp := t.TempDir()
	stderr := &bytes.Buffer{}
	release1, err := AcquireLock(tmp, stderr)
	if err != nil {
		t.Fatalf("first AcquireLock: %v", err)
	}
	defer release1()

	_, err = AcquireLock(tmp, stderr)
	if err == nil {
		t.Fatal("expected second acquire to error")
	}
	if !strings.Contains(err.Error(), "another eval is running") {
		t.Errorf("error should mention 'another eval is running'; got: %v", err)
	}
}

// TestLock_StaleClearOnDeadPID covers the PID-not-alive path. We
// write a lock file with a PID guaranteed to not exist (PID 1 is
// always alive, so we use a high invented PID and cross fingers; on
// macOS/Linux, picking an unlikely-running PID is safer than 0).
func TestLock_StaleClearOnDeadPID(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, LockFileName)
	// PID 999999 is virtually guaranteed not to exist on a test
	// machine. macOS PID_MAX defaults to 99999; Linux defaults to
	// 32768 unless raised. Either way 999999 is well above.
	body := fmt.Sprintf("999999\n%d\n", time.Now().Unix())
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("seed lock: %v", err)
	}

	stderr := &bytes.Buffer{}
	release, err := AcquireLock(tmp, stderr)
	if err != nil {
		t.Fatalf("AcquireLock should clear stale dead-PID lock: %v", err)
	}
	defer release()
	if !strings.Contains(stderr.String(), "not alive") {
		t.Errorf("expected dead-PID warning; got stderr: %q", stderr.String())
	}
}

// TestLock_BlockedBy1h59mLock covers the boundary: lock with mtime
// 1h59m old should still BLOCK (not clear). Pins the 2h threshold
// against regression-flipping the constant downward.
func TestLock_BlockedBy1h59mLock(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, LockFileName)
	// Use the current PID so PID-liveness check passes (mtime-only
	// staleness path under test).
	body := fmt.Sprintf("%d\n%d\n", os.Getpid(), time.Now().Unix())
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	old := time.Now().Add(-(1*time.Hour + 59*time.Minute))
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	stderr := &bytes.Buffer{}
	_, err := AcquireLock(tmp, stderr)
	if err == nil {
		t.Fatal("expected acquire to block on 1h59m-old + live-PID lock")
	}
	if !strings.Contains(err.Error(), "another eval is running") {
		t.Errorf("error should be the 'another eval' message; got: %v", err)
	}
}

// TestLock_ClearedBy2h01mLock covers the boundary: lock with mtime
// 2h01m old SHOULD clear regardless of PID liveness (PID could
// have been recycled across the 2h gap).
func TestLock_ClearedBy2h01mLock(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, LockFileName)
	// Use the current PID — even live PID gets cleared past the 2h
	// threshold. This is the spec contract: mtime check fires
	// independent of PID-liveness.
	body := fmt.Sprintf("%d\n%d\n", os.Getpid(), time.Now().Unix())
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	old := time.Now().Add(-(2*time.Hour + 1*time.Minute))
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	stderr := &bytes.Buffer{}
	release, err := AcquireLock(tmp, stderr)
	if err != nil {
		t.Fatalf("AcquireLock should clear 2h01m-old lock: %v", err)
	}
	defer release()
	if !strings.Contains(stderr.String(), "stale lock") {
		t.Errorf("expected stale-lock warning; got stderr: %q", stderr.String())
	}
}

// TestLock_UnparseableBodyTreatedAsStale covers the corrupted-state
// recovery: a lock file with garbage contents shouldn't permanently
// block the workspace. Treat as stale + clear.
func TestLock_UnparseableBodyTreatedAsStale(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, LockFileName)
	if err := os.WriteFile(path, []byte("not a pid\nnot a timestamp\n"), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	stderr := &bytes.Buffer{}
	release, err := AcquireLock(tmp, stderr)
	if err != nil {
		t.Fatalf("unparseable lock should clear: %v", err)
	}
	defer release()
	if !strings.Contains(stderr.String(), "unparseable") {
		t.Errorf("expected 'unparseable' warning; got: %q", stderr.String())
	}
}
