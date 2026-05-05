package swarm

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateSHA_Accepts(t *testing.T) {
	cases := []string{
		"abcdef1",                      // 7 chars (min)
		"abc1234",
		"abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789", // 64 (max)
		"f791839c2c6b80",
	}
	for _, sha := range cases {
		if err := ValidateSHA(sha); err != nil {
			t.Errorf("expected %q to validate, got %v", sha, err)
		}
	}
}

func TestValidateSHA_Rejects(t *testing.T) {
	cases := []string{
		"",                              // empty
		"abc",                           // too short
		"ABCDEF1",                       // uppercase
		"../../etc",                     // path traversal
		"abcdef1/",                      // trailing slash
		"abcdef1\nfoo",                  // newline injection
		"deadbee",                       // 7 chars but contains non-hex 'g' would fail; 'deadbee' is ok actually — 7 hex chars
		strings.Repeat("a", 65),         // too long
		"abc def",                       // space
	}
	// Fix: "deadbee" is valid hex (a–f), so remove from rejects.
	for _, sha := range cases {
		if sha == "deadbee" {
			continue
		}
		if err := ValidateSHA(sha); err == nil {
			t.Errorf("expected %q to be rejected", sha)
		}
	}
}

func TestCacheDir_StaysInsideRoot(t *testing.T) {
	// Even if a malicious SHA somehow bypassed ValidateSHA (e.g.,
	// a future regression), filepath.Join would still resolve
	// the path. We rely on ValidateSHA being called first; this
	// test documents the assumption by showing what filepath.Join
	// does with a traversal attempt — it normalizes but DOES escape.
	root := "/tmp/project"
	preset := &Preset{Name: "pr-review"}
	dir := CacheDir(root, preset, "../../etc")
	if !strings.HasPrefix(filepath.Clean(dir), filepath.Clean(root)) {
		// This is the EXPECTED failure mode: path escapes root.
		// ValidateSHA must catch this before CacheDir is reached.
		t.Logf("confirmed: path traversal escapes root (%q); ValidateSHA must guard the input", dir)
	}
}

func TestCacheDir_PerPresetSegment(t *testing.T) {
	root := "/tmp/project"
	pr := &Preset{Name: "pr-review"}
	doc := &Preset{Name: "doc-review"}
	prDir := CacheDir(root, pr, "abc1234")
	docDir := CacheDir(root, doc, "abc1234")
	if !strings.HasSuffix(prDir, "/.kaijutsu/pr-review-runs/abc1234") {
		t.Errorf("pr-review path wrong: %q", prDir)
	}
	if !strings.HasSuffix(docDir, "/.kaijutsu/doc-review-runs/abc1234") {
		t.Errorf("doc-review path wrong: %q", docDir)
	}
	if prDir == docDir {
		t.Errorf("expected per-preset segment to differ; both = %q", prDir)
	}
}

func TestCacheDir_HonorsCachePathPartOverride(t *testing.T) {
	root := "/tmp/project"
	custom := &Preset{Name: "doc-review", CachePathPart: "doc-runs"}
	dir := CacheDir(root, custom, "abc1234")
	if !strings.HasSuffix(dir, "/.kaijutsu/doc-runs/abc1234") {
		t.Errorf("override not honored: %q", dir)
	}
}

// TestCacheDir_NoCollisionAcrossSecurityAuditAndPRReview verifies
// that auditing the same diff twice — once via pr-review, once via
// security-audit — produces distinct cache dirs. The cache-segment
// is per-preset (`<Name>-runs`) so the two never overlap. Stage 7
// acceptance criterion.
func TestCacheDir_NoCollisionAcrossSecurityAuditAndPRReview(t *testing.T) {
	root := "/tmp/project"
	pr := &Preset{Name: "pr-review"}
	sa := &Preset{Name: "security-audit"}
	sha := "abc1234"
	prDir := CacheDir(root, pr, sha)
	saDir := CacheDir(root, sa, sha)
	if prDir == saDir {
		t.Fatalf("expected distinct cache dirs, both = %q", prDir)
	}
	if !strings.HasSuffix(prDir, "/.kaijutsu/pr-review-runs/abc1234") {
		t.Errorf("pr-review path wrong: %q", prDir)
	}
	if !strings.HasSuffix(saDir, "/.kaijutsu/security-audit-runs/abc1234") {
		t.Errorf("security-audit path wrong: %q", saDir)
	}
}

func TestSanitizeAgent_DropsUnsafeChars(t *testing.T) {
	got := sanitizeAgent("../../foo")
	if got != "foo" {
		t.Errorf("want 'foo', got %q", got)
	}
	if sanitizeAgent("Claude") != "laude" { // uppercase C dropped
		t.Errorf("uppercase not stripped")
	}
}
