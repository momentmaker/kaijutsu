package swarm

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestGeminiPromptToolsPolicyInSync asserts that the TOOLS POLICY
// paragraph is byte-identical between the in-binary fallback prompt
// (prReviewPreset.PerAgent[AgentGemini] in preset.go) and the
// skill-shipped override (skills/core/pr-review/prompts/gemini.md).
//
// Background: Phase-2 review caught us editing one without the
// other. This test fails fast on drift so the next editor notices
// at CI rather than at runtime.
func TestGeminiPromptToolsPolicyInSync(t *testing.T) {
	const marker = "CRITICAL — TOOLS POLICY"
	inBinary := prReviewPreset.PerAgent[AgentGemini]
	if !strings.Contains(inBinary, marker) {
		t.Fatal("in-binary gemini prompt missing TOOLS POLICY marker — did the prompt get rewritten?")
	}

	skillPath := skillPromptPathFromTest(t, "gemini.md")
	disk, err := os.ReadFile(skillPath)
	if err != nil {
		t.Skipf("skill prompt not at expected dev-checkout path %s: %v (skipping; CI/dev usually has it)", skillPath, err)
	}
	if !strings.Contains(string(disk), marker) {
		t.Fatalf("disk gemini prompt missing TOOLS POLICY marker at %s", skillPath)
	}

	inBinaryPolicy := extractParagraphFrom(inBinary, marker)
	diskPolicy := extractParagraphFrom(string(disk), marker)
	if inBinaryPolicy != diskPolicy {
		t.Errorf("TOOLS POLICY paragraphs out of sync between %s and preset.go.\n\n  binary: %q\n\n  disk:   %q\n\nUpdate both to match.", skillPath, inBinaryPolicy, diskPolicy)
	}
}

// skillPromptPathFromTest resolves
// <repo-root>/skills/core/pr-review/prompts/<file> from the test
// file's location at runtime. Avoids hardcoding a relative path
// that breaks if tests are run from elsewhere.
func skillPromptPathFromTest(t *testing.T, file string) string {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed; cannot resolve skill prompt path")
	}
	return filepath.Join(filepath.Dir(here), "..", "..", "..", "skills", "core", "pr-review", "prompts", file)
}

// extractParagraphFrom returns the substring starting at marker and
// ending at the first blank line OR the next %s placeholder OR
// end-of-string, whichever comes first. Used to compare a single
// instruction paragraph independent of surrounding scaffolding.
func extractParagraphFrom(s, marker string) string {
	idx := strings.Index(s, marker)
	if idx < 0 {
		return ""
	}
	tail := s[idx:]
	end := len(tail)
	for _, sep := range []string{"\n\n", "%s"} {
		if i := strings.Index(tail, sep); i > 0 && i < end {
			end = i
		}
	}
	return strings.TrimSpace(tail[:end])
}
