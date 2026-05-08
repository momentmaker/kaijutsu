package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestWriteAgentsFragment_CreatesWhenMissing covers cold-start: no
// AGENTS.md exists, init writes one with just the fragment.
func TestWriteAgentsFragment_CreatesWhenMissing(t *testing.T) {
	tmp := t.TempDir()
	action, err := writeAgentsFragment(tmp)
	if err != nil {
		t.Fatalf("writeAgentsFragment: %v", err)
	}
	if action != "created" {
		t.Errorf("action = %q, want created", action)
	}
	body := mustRead(t, filepath.Join(tmp, "AGENTS.md"))
	if !strings.Contains(body, agentsFragmentMarkerStart) {
		t.Error("written file missing start marker")
	}
	if !strings.Contains(body, agentsFragmentMarkerEnd) {
		t.Error("written file missing end marker")
	}
}

// TestWriteAgentsFragment_AppendsToExisting covers the case where
// AGENTS.md exists with no kaijutsu markers — fragment appends at
// end, existing content untouched.
func TestWriteAgentsFragment_AppendsToExisting(t *testing.T) {
	tmp := t.TempDir()
	preexisting := "# My Project\n\nUser-authored agent rules go here.\n"
	mustWrite(t, filepath.Join(tmp, "AGENTS.md"), preexisting)

	action, err := writeAgentsFragment(tmp)
	if err != nil {
		t.Fatalf("writeAgentsFragment: %v", err)
	}
	if action != "appended" {
		t.Errorf("action = %q, want appended", action)
	}
	body := mustRead(t, filepath.Join(tmp, "AGENTS.md"))
	if !strings.HasPrefix(body, preexisting) {
		t.Error("preexisting content lost")
	}
	if !strings.Contains(body, agentsFragmentMarkerStart) {
		t.Error("fragment not appended")
	}
}

// TestWriteAgentsFragment_ReplacesInPlace covers idempotent re-run:
// markers present, content between gets replaced. Surrounding user
// content preserved verbatim.
func TestWriteAgentsFragment_ReplacesInPlace(t *testing.T) {
	tmp := t.TempDir()
	preexisting := "# Project\n\nMy notes.\n\n" +
		agentsFragmentMarkerStart + "\nstale content from previous version\n" + agentsFragmentMarkerEnd +
		"\n\n## Other section the user wrote\n\nMore user notes.\n"
	mustWrite(t, filepath.Join(tmp, "AGENTS.md"), preexisting)

	action, err := writeAgentsFragment(tmp)
	if err != nil {
		t.Fatalf("writeAgentsFragment: %v", err)
	}
	if action != "replaced" {
		t.Errorf("action = %q, want replaced", action)
	}
	body := mustRead(t, filepath.Join(tmp, "AGENTS.md"))
	if strings.Contains(body, "stale content from previous version") {
		t.Error("stale content not replaced")
	}
	if !strings.Contains(body, "## Other section the user wrote") {
		t.Error("user content after the fragment lost")
	}
	if !strings.Contains(body, "My notes.") {
		t.Error("user content before the fragment lost")
	}
}

// TestWriteAgentsFragment_RefusesCorruptMarkers covers the safety
// net: only one marker = corrupt state. Refuse to write rather than
// guess what the user intended.
func TestWriteAgentsFragment_RefusesCorruptMarkers(t *testing.T) {
	tmp := t.TempDir()
	corrupt := "# Project\n\n" + agentsFragmentMarkerStart + "\n(no end marker, file is truncated)\n"
	mustWrite(t, filepath.Join(tmp, "AGENTS.md"), corrupt)

	_, err := writeAgentsFragment(tmp)
	if err == nil {
		t.Fatal("expected error for corrupt marker state")
	}
	if !strings.Contains(err.Error(), "marker") {
		t.Errorf("error should mention marker repair; got: %v", err)
	}
	// File should be untouched.
	after := mustRead(t, filepath.Join(tmp, "AGENTS.md"))
	if after != corrupt {
		t.Error("file was modified despite error — should be untouched")
	}
}

// TestWriteAgentsFragment_UpgradesAcrossVersions covers the regression
// flagged by the v0.8.2 adversarial review: a v0.8.2 user upgrading
// to a future jutsu version (different version pin in the start
// marker) must still get an idempotent in-place replace, not a
// corrupt-state error.
func TestWriteAgentsFragment_UpgradesAcrossVersions(t *testing.T) {
	tmp := t.TempDir()
	// Simulate a marker block written by an older jutsu version.
	older := "# Project\n\n" +
		"<!-- kaijutsu:start name=jutsu-cli version=0.5.0 -->\nold body from v0.5.0\n" +
		agentsFragmentMarkerEnd + "\n\n## User content\n"
	mustWrite(t, filepath.Join(tmp, "AGENTS.md"), older)

	action, err := writeAgentsFragment(tmp)
	if err != nil {
		t.Fatalf("upgrade scenario errored (should idempotent-replace): %v", err)
	}
	if action != "replaced" {
		t.Errorf("action = %q, want replaced", action)
	}
	body := mustRead(t, filepath.Join(tmp, "AGENTS.md"))
	if strings.Contains(body, "version=0.5.0") {
		t.Error("old version marker not replaced — upgrade path broken")
	}
	if !strings.Contains(body, "version=0.12.0") {
		t.Error("new version marker missing")
	}
	if strings.Contains(body, "old body from v0.5.0") {
		t.Error("old body not replaced")
	}
	if !strings.Contains(body, "## User content") {
		t.Error("user content after the block was lost during upgrade")
	}
}

// TestWriteAgentsFragment_Idempotent covers running writeAgentsFragment
// twice in succession — second call should report 'unchanged' (or
// 'replaced' depending on byte-equality), and content stays the same.
func TestWriteAgentsFragment_Idempotent(t *testing.T) {
	tmp := t.TempDir()
	if _, err := writeAgentsFragment(tmp); err != nil {
		t.Fatalf("first call: %v", err)
	}
	first := mustRead(t, filepath.Join(tmp, "AGENTS.md"))

	action, err := writeAgentsFragment(tmp)
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if action != "replaced" && action != "unchanged" {
		t.Errorf("second call action = %q, want replaced or unchanged", action)
	}
	second := mustRead(t, filepath.Join(tmp, "AGENTS.md"))
	if first != second {
		t.Errorf("idempotency broken: file content changed between identical calls\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

// TestWriteAgentsFragment_BodyContent verifies the fragment body
// includes the key agent-discovery commands. Guards against
// accidentally scrubbing the discoverability content from the
// template.
func TestWriteAgentsFragment_BodyContent(t *testing.T) {
	tmp := t.TempDir()
	if _, err := writeAgentsFragment(tmp); err != nil {
		t.Fatalf("write: %v", err)
	}
	body := mustRead(t, filepath.Join(tmp, "AGENTS.md"))
	for _, must := range []string{
		"jutsu describe",
		"jutsu suggest",
		"jutsu install",
		"jutsu swarm",
		"jutsu finding list",
		"agent-first",
	} {
		if !strings.Contains(body, must) {
			t.Errorf("fragment missing key reference %q", must)
		}
	}
}

// --- helpers ---

func mustRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

func mustWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
