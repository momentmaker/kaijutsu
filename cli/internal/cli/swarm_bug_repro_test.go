package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSwarmBugRepro_HelpListsFlags pins the cobra wiring contract:
// help mentions --files + describes the bug-description positional.
func TestSwarmBugRepro_HelpListsFlags(t *testing.T) {
	cmd := newSwarmBugReproCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("help: %v", err)
	}
	for _, want := range []string{"bug-repro", "bug description", "--files", "category"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("help missing %q; got: %s", want, buf.String())
		}
	}
}

// TestSwarmBugRepro_RequiresBugDescription pins validation:
// 0 positional args → cobra error with the description-required
// hint. Allows the --replay / --grant-consent short-circuit paths.
func TestSwarmBugRepro_RequiresBugDescription(t *testing.T) {
	cmd := newSwarmBugReproCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when no bug description is provided")
	}
	if !strings.Contains(err.Error(), "bug description") {
		t.Errorf("error should mention 'bug description'; got: %v", err)
	}
}

// TestSwarmBugRepro_RejectsEmptyAfterCanonicalize covers the
// whitespace-only edge case: positional arg present but empty after
// trim → reject. Otherwise InputPrompt's canonicalizer returns
// "" and the deeper pipeline errors with a less-clear message.
func TestSwarmBugRepro_RejectsEmptyAfterCanonicalize(t *testing.T) {
	cmd := newSwarmBugReproCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"   "})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for whitespace-only bug description")
	}
}

// TestSwarmBugRepro_RejectsUnreadableFile covers Decision #8:
// nonexistent --files path surfaces as a cobra error.
func TestSwarmBugRepro_RejectsUnreadableFile(t *testing.T) {
	cmd := newSwarmBugReproCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"a real bug", "--files", "/this/path/does/not/exist"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for nonexistent --files path")
	}
}

// TestReadBugReproFiles_NoFilesReturnsEmpty pins the no-context
// path: zero --files = empty string (not error). Preset works
// without code context (falls back to bugReproDefaultPrompt).
func TestReadBugReproFiles_NoFilesReturnsEmpty(t *testing.T) {
	body, err := readBugReproFiles(nil)
	if err != nil {
		t.Fatal(err)
	}
	if body != "" {
		t.Errorf("nil files should return empty body; got %q", body)
	}
	body, err = readBugReproFiles([]string{})
	if err != nil {
		t.Fatal(err)
	}
	if body != "" {
		t.Errorf("empty files should return empty body; got %q", body)
	}
}

// TestReadBugReproFiles_SingleFile reads one file with a
// path-header prefix (so the agent can attribute findings).
func TestReadBugReproFiles_SingleFile(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "x.go")
	body := "package x\nfunc Foo() {}\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := readBugReproFiles([]string{path})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "// === "+path+" ===") {
		t.Error("missing path header")
	}
	if !strings.Contains(got, "func Foo()") {
		t.Error("missing file body")
	}
}

// TestSwarmBugRepro_RejectsOversizeFiles pins the 200 KB cap on
// --files content (Decision #8). Final-pr-review caught this
// missing.
func TestSwarmBugRepro_RejectsOversizeFiles(t *testing.T) {
	tmp := t.TempDir()
	bigBody := strings.Repeat("a", MaxBugReproFilesBytes+1024)
	bigPath := filepath.Join(tmp, "big.go")
	if err := os.WriteFile(bigPath, []byte(bigBody), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := newSwarmBugReproCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"a real bug", "--files", bigPath})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for oversize --files content")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Errorf("error should mention the size cap; got: %v", err)
	}
}

// TestReadBugReproFiles_DirectoryRecursive walks a directory +
// concatenates all regular files with relative-path headers.
func TestReadBugReproFiles_DirectoryRecursive(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "a.go"), []byte("aaa"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "sub", "b.go"), []byte("bbb"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := readBugReproFiles([]string{tmp})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "aaa") || !strings.Contains(got, "bbb") {
		t.Error("recursive walk missed file content")
	}
}
