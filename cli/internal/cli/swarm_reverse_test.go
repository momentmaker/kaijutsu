package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

// TestSwarmReverse_RequiresSpec verifies the cobra-layer guard:
// missing --spec produces a clear error before any agent dispatch.
func TestSwarmReverse_RequiresSpec(t *testing.T) {
	root := NewRootCmd()
	root.SetArgs([]string{"swarm", "reverse"})
	stderr := &bytes.Buffer{}
	root.SetOut(&bytes.Buffer{})
	root.SetErr(stderr)
	err := root.Execute()
	if err == nil {
		t.Fatal("expected --spec to be required")
	}
	if !strings.Contains(err.Error(), "--spec") {
		t.Errorf("error should mention --spec; got: %v", err)
	}
}

// TestSwarmReverse_RejectsMissingSpecFile covers the file-read error
// path: --spec points at a nonexistent file → wrapped error from
// os.ReadFile.
func TestSwarmReverse_RejectsMissingSpecFile(t *testing.T) {
	tmp := t.TempDir()
	root := NewRootCmd()
	root.SetArgs([]string{"swarm", "reverse", "--spec", filepath.Join(tmp, "no-such-file.md")})
	stderr := &bytes.Buffer{}
	root.SetOut(&bytes.Buffer{})
	root.SetErr(stderr)
	err := root.Execute()
	if err == nil {
		t.Fatal("expected missing spec file to error")
	}
	if !strings.Contains(err.Error(), "read spec") {
		t.Errorf("error should mention 'read spec'; got: %v", err)
	}
}

// TestLieToThemFromArg covers the v0.9 reverse flag parsing: maps
// "on"/"off" surface name to the underlying strict bool that the
// v0.7 sycophancy filter consumes.
func TestLieToThemFromArg(t *testing.T) {
	cases := map[string]bool{
		"on":      true,
		"true":    true,
		"1":       true,
		"off":     false,
		"false":   false,
		"":        false,
		"unknown": false, // unknown values default to OFF (reverse default)
	}
	for in, want := range cases {
		if got := lieToThemFromArg(in); got != want {
			t.Errorf("lieToThemFromArg(%q) = %v, want %v", in, got, want)
		}
	}
}
