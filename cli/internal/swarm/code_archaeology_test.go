package swarm

import (
	"strings"
	"testing"
)

func TestCodeArchaeologyPreset_Registers(t *testing.T) {
	p, err := defaultRegistry.Find("code-archaeology")
	if err != nil {
		t.Fatalf("Find(code-archaeology): %v", err)
	}
	if p.Name != "code-archaeology" {
		t.Errorf("preset.Name = %q, want code-archaeology", p.Name)
	}
}

func TestCodeArchaeologyPreset_DefaultPromptHasInputSlot(t *testing.T) {
	p, err := defaultRegistry.Find("code-archaeology")
	if err != nil {
		t.Fatal(err)
	}
	count := strings.Count(p.DefaultPrompt, "%s")
	if count != 1 {
		t.Errorf("DefaultPrompt has %d %%s slots, want 1", count)
	}
}

func TestCodeArchaeologyPreset_SeverityVocab(t *testing.T) {
	p, err := defaultRegistry.Find("code-archaeology")
	if err != nil {
		t.Fatal(err)
	}
	want := []Severity{SeverityBlocker, SeverityIssue, SeverityMinor, SeverityInfo}
	if len(p.SeverityVocab) != len(want) {
		t.Fatalf("SeverityVocab len = %d, want %d", len(p.SeverityVocab), len(want))
	}
	for i, s := range want {
		if p.SeverityVocab[i] != s {
			t.Errorf("SeverityVocab[%d] = %q, want %q", i, p.SeverityVocab[i], s)
		}
	}
}

func TestCodeArchaeologyPreset_InputKindIsFiles(t *testing.T) {
	p, err := defaultRegistry.Find("code-archaeology")
	if err != nil {
		t.Fatal(err)
	}
	if p.InputKind != InputFiles {
		t.Errorf("InputKind = %v, want InputFiles", p.InputKind)
	}
}

// TestBuildCodeArchaeologyPrompt_TwoSlotPattern covers the safety
// net against the v0.8 dreamLensWild incident: literal `%` chars
// in git-log content escaped to `%%` BEFORE substitution. Commit
// messages can contain `100%` / `5% improvement` / etc.
func TestBuildCodeArchaeologyPrompt_TwoSlotPattern(t *testing.T) {
	history := "commit abc123\nAuthor: rubberduck\nDate: 2024-01-01\n\n    fix: 100% slack threshold\n"
	prompt := BuildCodeArchaeologyPrompt(history)
	if strings.Contains(prompt, "{{HISTORY_CONTENT}}") {
		t.Error("placeholder not substituted")
	}
	if !strings.Contains(prompt, "100%% slack") {
		t.Error("literal % must be escaped to %% so downstream fmt.Sprintf is safe")
	}
	if strings.Count(prompt, "%s") != 1 {
		t.Errorf("expected exactly 1 %%s slot for code-substitution; got %d", strings.Count(prompt, "%s"))
	}
}

// TestBuildCodeArchaeologyPrompt_EmptyHistoryFallsBackToDefault
// pins Decision #9: empty git-log content (failure or new repo)
// degrades gracefully to no-history mode, NOT hard fail.
func TestBuildCodeArchaeologyPrompt_EmptyHistoryFallsBackToDefault(t *testing.T) {
	got := BuildCodeArchaeologyPrompt("")
	if got != codeArchaeologyDefaultPrompt {
		t.Error("empty history should fall back to codeArchaeologyDefaultPrompt")
	}
	got2 := BuildCodeArchaeologyPrompt("\n  \t  \n")
	if got2 != codeArchaeologyDefaultPrompt {
		t.Error("whitespace-only history should fall back to codeArchaeologyDefaultPrompt")
	}
}

func TestCodeArchaeologyPreset_SynthesizerHasFindingsSlot(t *testing.T) {
	p, err := defaultRegistry.Find("code-archaeology")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(p.Synthesizer, "%s") {
		t.Error("synthesizer must have a per-agent-findings JSON substitution slot")
	}
}
