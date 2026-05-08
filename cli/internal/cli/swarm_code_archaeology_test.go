package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSwarmCodeArchaeology_HelpListsFlags(t *testing.T) {
	cmd := newSwarmCodeArchaeologyCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("help: %v", err)
	}
	for _, want := range []string{"code-archaeology", "--code", "--git-log", "workaround"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("help missing %q; got: %s", want, buf.String())
		}
	}
}

func TestSwarmCodeArchaeology_RejectsMissingCode(t *testing.T) {
	cmd := newSwarmCodeArchaeologyCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when --code is missing")
	}
	if !strings.Contains(err.Error(), "--code") {
		t.Errorf("error should name --code; got: %v", err)
	}
}

func TestSwarmCodeArchaeology_RejectsUnreadableCode(t *testing.T) {
	cmd := newSwarmCodeArchaeologyCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--code", "/this/path/does/not/exist"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing --code path")
	}
}

// TestFetchGitLog_GracefulDegradation pins Decision #9: a non-git
// directory yields empty string + stderr warning, NOT hard error.
// Test by running fetchGitLog from a temp dir that's not a git repo.
func TestFetchGitLog_GracefulDegradation(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "x.go"), []byte("package x"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	got := fetchGitLog(tmp, filepath.Join(tmp, "x.go"), "", &stderr)
	if got != "" {
		t.Errorf("non-git-repo should yield empty git-log; got %q", got)
	}
	if !strings.Contains(stderr.String(), "warning: code-archaeology git-log fetch failed") {
		t.Errorf("expected stderr warning; got: %q", stderr.String())
	}
}

// TestFetchGitLog_RealRepoReturnsContent: from inside an actual
// git repo (the kaijutsu repo itself), fetchGitLog returns
// non-empty content for a tracked path. Sanity-checks the happy
// path without mocking exec.
func TestFetchGitLog_RealRepoReturnsContent(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	// Find the project root by walking up from cwd looking for .git.
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := wd
	for {
		if _, err := os.Stat(filepath.Join(root, ".git")); err == nil {
			break
		}
		parent := filepath.Dir(root)
		if parent == root {
			t.Skip("not running inside a git repo; skipping")
		}
		root = parent
	}

	// Pick a path that definitely has commits (this test file's
	// own directory).
	codePath := filepath.Join(root, "cli", "internal", "cli")
	var stderr bytes.Buffer
	got := fetchGitLog(root, codePath, "", &stderr)
	if got == "" {
		t.Errorf("expected non-empty git-log for known-tracked path; stderr=%q", stderr.String())
	}
	if !strings.Contains(got, "commit") {
		t.Errorf("git-log output should contain 'commit'; got prefix %q", got[:min(200, len(got))])
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
