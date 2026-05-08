package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSwarmTestGap_HelpListsFlags pins the cobra wiring contract:
// help output mentions --code + --tests so users discover the
// required flags from `--help`.
func TestSwarmTestGap_HelpListsFlags(t *testing.T) {
	cmd := newSwarmTestGapCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("help: %v", err)
	}
	for _, want := range []string{"--code", "--tests", "test-gap"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("help missing %q; got: %s", want, buf.String())
		}
	}
}

// TestSwarmTestGap_RejectsMissingCode pins the validation contract:
// no --code flag → cobra error with the required-flag hint.
func TestSwarmTestGap_RejectsMissingCode(t *testing.T) {
	cmd := newSwarmTestGapCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--tests", "/tmp"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when --code is missing")
	}
	if !strings.Contains(err.Error(), "--code") {
		t.Errorf("error should name --code; got: %v", err)
	}
}

// TestSwarmTestGap_RejectsMissingTests covers the symmetric path:
// --code without --tests → required-flag error.
func TestSwarmTestGap_RejectsMissingTests(t *testing.T) {
	tmp := t.TempDir()
	codePath := filepath.Join(tmp, "code.go")
	if err := os.WriteFile(codePath, []byte("package x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := newSwarmTestGapCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--code", codePath})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when --tests is missing")
	}
	if !strings.Contains(err.Error(), "--tests") {
		t.Errorf("error should name --tests; got: %v", err)
	}
}

// TestSwarmTestGap_RejectsUnreadableTests covers the per-spec
// Decision #8: unreadable / nonexistent paths surface as cobra
// errors, not deeper-pipeline failures.
func TestSwarmTestGap_RejectsUnreadableTests(t *testing.T) {
	tmp := t.TempDir()
	codePath := filepath.Join(tmp, "code.go")
	if err := os.WriteFile(codePath, []byte("package x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := newSwarmTestGapCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--code", codePath, "--tests", "/this/path/does/not/exist"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing --tests path")
	}
}

// TestReadTestsContent_File reads a single file and returns the
// bytes verbatim (no header injection for single-file mode).
func TestReadTestsContent_File(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "x_test.go")
	body := "package x\n\nfunc TestFoo(t *testing.T) {}\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := readTestsContent(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != body {
		t.Errorf("file body mismatch: got %q, want %q", got, body)
	}
}

// TestReadTestsContent_DirectoryWithHeaders covers the recursive
// directory walk: each file gets a `// === <relative-path> ===`
// header so the agent can attribute findings back to specific
// test files.
func TestReadTestsContent_DirectoryWithHeaders(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "a_test.go"), []byte("aaa\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "sub", "b_test.go"), []byte("bbb\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := readTestsContent(tmp)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"// === a_test.go ===",
		"// === sub/b_test.go ===",
		"aaa",
		"bbb",
	} {
		if !strings.Contains(string(got), want) {
			t.Errorf("output missing %q; got %q", want, got)
		}
	}
}

// TestReadTestsContent_SkipsHiddenDirs ensures the walk doesn't
// recurse into .git / .kaijutsu / etc. — those would balloon the
// content cap with project-management noise.
func TestReadTestsContent_SkipsHiddenDirs(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, ".git", "HEAD"), []byte("ref: refs/heads/main"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "real_test.go"), []byte("real\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := readTestsContent(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(got), "ref: refs/heads/main") {
		t.Error(".git content should not appear in output")
	}
	if !strings.Contains(string(got), "real") {
		t.Error("non-hidden file content missing")
	}
}
