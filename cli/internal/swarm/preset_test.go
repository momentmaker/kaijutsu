package swarm

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPromptFor_ExactAgentMatchWins(t *testing.T) {
	p := &Preset{
		Name: "test",
		PerAgent: map[AgentName]string{
			AgentClaude: "claude-template",
			AgentCodex:  "codex-template",
		},
	}
	got, ok := p.PromptFor("codex")
	if !ok || got != "codex-template" {
		t.Errorf("got %q ok=%v, want codex-template true", got, ok)
	}
}

func TestPromptFor_FallsBackToDefaultPrompt(t *testing.T) {
	p := &Preset{
		Name: "test",
		PerAgent: map[AgentName]string{
			AgentClaude: "claude-template",
		},
		DefaultPrompt: "default-template",
	}
	got, ok := p.PromptFor("deepseek")
	if !ok || got != "default-template" {
		t.Errorf("got %q ok=%v, want default-template true", got, ok)
	}
}

func TestPromptFor_FallsBackToClaudeWhenNoDefault(t *testing.T) {
	p := &Preset{
		Name: "test",
		PerAgent: map[AgentName]string{
			AgentClaude: "claude-template",
			AgentAntigravity: "antigravity-template",
		},
	}
	got, ok := p.PromptFor("deepseek")
	if !ok || got != "claude-template" {
		t.Errorf("got %q ok=%v, want claude-template (implicit fallback) true", got, ok)
	}
}

func TestPromptFor_NoMatchReturnsFalse(t *testing.T) {
	p := &Preset{
		Name:     "test",
		PerAgent: map[AgentName]string{AgentCodex: "codex-template"},
	}
	_, ok := p.PromptFor("deepseek")
	if ok {
		t.Error("expected ok=false when no match + no default + no claude template")
	}
}

// TestAntigravityPromptToolsPolicyInSync asserts that the TOOLS POLICY
// paragraph is byte-identical between the in-binary fallback prompt
// (preset.PerAgent[AgentAntigravity] in preset.go) and the skill-shipped
// override (skills/core/<preset>/prompts/antigravity.md).
//
// Background: Phase-2 round-3 review caught us editing one without
// the other. This test fails fast on drift so the next editor
// notices at CI rather than at runtime. Generalized in Stage 7
// polish to cover every preset that ships an antigravity.md in its
// skill — drift surface scales with preset count.
func TestAntigravityPromptToolsPolicyInSync(t *testing.T) {
	const marker = "CRITICAL — TOOLS POLICY"
	cases := []struct {
		preset *Preset
		// skillDir under skills/core/ where the override lives. Some
		// skills' directory name doesn't exactly match preset.Name —
		// none today, but parametrize for future-proofing.
		skillDir string
	}{
		{&prReviewPreset, "pr-review"},
		{&docReviewPreset, "doc-review"},
		{&brainstormPreset, "brainstorm"},
		{&refactorPlanPreset, "refactor-plan"},
		{&securityAuditPreset, "security-audit"},
	}
	for _, tc := range cases {
		t.Run(tc.preset.Name, func(t *testing.T) {
			inBinary := tc.preset.PerAgent[AgentAntigravity]
			if !strings.Contains(inBinary, marker) {
				t.Fatalf("%s: in-binary antigravity prompt missing TOOLS POLICY marker — did the prompt get rewritten?", tc.preset.Name)
			}

			skillPath := skillPromptPathFromTest(t, tc.skillDir, "antigravity.md")
			disk, err := os.ReadFile(skillPath)
			if err != nil {
				t.Skipf("skill prompt not at expected dev-checkout path %s: %v (skipping; CI/dev usually has it)", skillPath, err)
			}
			if !strings.Contains(string(disk), marker) {
				t.Fatalf("%s: disk antigravity prompt missing TOOLS POLICY marker at %s", tc.preset.Name, skillPath)
			}

			inBinaryPolicy := extractParagraphFrom(inBinary, marker)
			diskPolicy := extractParagraphFrom(string(disk), marker)
			if inBinaryPolicy != diskPolicy {
				t.Errorf("%s: TOOLS POLICY paragraphs out of sync between %s and preset.go.\n\n  binary: %q\n\n  disk:   %q\n\nUpdate both to match.", tc.preset.Name, skillPath, inBinaryPolicy, diskPolicy)
			}
		})
	}
}

// skillPromptPathFromTest resolves
// <repo-root>/skills/core/<dir>/prompts/<file> from the test file's
// location at runtime. Avoids hardcoding a relative path that
// breaks if tests are run from elsewhere.
func skillPromptPathFromTest(t *testing.T, skillDir, file string) string {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed; cannot resolve skill prompt path")
	}
	return filepath.Join(filepath.Dir(here), "..", "..", "..", "skills", "core", skillDir, "prompts", file)
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
