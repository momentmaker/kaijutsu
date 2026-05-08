package swarm

import (
	"strings"
	"testing"
)

// TestBugReproPreset_Registers ensures the v0.12 preset is in the
// default registry and findable by name.
func TestBugReproPreset_Registers(t *testing.T) {
	p, err := defaultRegistry.Find("bug-repro")
	if err != nil {
		t.Fatalf("Find(bug-repro): %v", err)
	}
	if p.Name != "bug-repro" {
		t.Errorf("preset.Name = %q, want bug-repro", p.Name)
	}
}

// TestBugReproPreset_DefaultPromptHasInputSlot ensures the default
// (registry-loaded) prompt has exactly one %s slot for the bug
// description substitution at dispatch time.
func TestBugReproPreset_DefaultPromptHasInputSlot(t *testing.T) {
	p, err := defaultRegistry.Find("bug-repro")
	if err != nil {
		t.Fatal(err)
	}
	count := strings.Count(p.DefaultPrompt, "%s")
	if count != 1 {
		t.Errorf("DefaultPrompt has %d %%s slots, want 1", count)
	}
}

// TestBugReproPreset_SeverityVocab pins cross-preset consistency.
// All Tier A presets share pr-review/reverse vocab.
func TestBugReproPreset_SeverityVocab(t *testing.T) {
	p, err := defaultRegistry.Find("bug-repro")
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

// TestBugReproPreset_InputKindIsPrompt pins the dispatch shape.
// Bug description is the primary input (InputPrompt); --files
// content bakes into the prompt via cobra-time substitution.
func TestBugReproPreset_InputKindIsPrompt(t *testing.T) {
	p, err := defaultRegistry.Find("bug-repro")
	if err != nil {
		t.Fatal(err)
	}
	if p.InputKind != InputPrompt {
		t.Errorf("InputKind = %v, want InputPrompt", p.InputKind)
	}
}

// TestBuildBugReproPrompt_TwoSlotPattern covers the safety net
// against the v0.8 dreamLensWild incident: literal `%` chars in
// files content escaped to `%%` BEFORE substitution.
func TestBuildBugReproPrompt_TwoSlotPattern(t *testing.T) {
	files := "// expects: 95% uptime under load"
	prompt := BuildBugReproPrompt(files)
	if strings.Contains(prompt, "{{FILES_CONTENT}}") {
		t.Error("placeholder not substituted")
	}
	if !strings.Contains(prompt, "95%% uptime") {
		t.Error("literal % must be escaped to %% so downstream fmt.Sprintf is safe")
	}
	if strings.Count(prompt, "%s") != 1 {
		t.Errorf("expected exactly 1 %%s slot for bug-substitution; got %d", strings.Count(prompt, "%s"))
	}
}

// TestBuildBugReproPrompt_EmptyFilesFallsBackToDefault: no --files
// → bug-repro still works (no-context fallback). Spec: graceful
// degradation, not hard fail.
func TestBuildBugReproPrompt_EmptyFilesFallsBackToDefault(t *testing.T) {
	got := BuildBugReproPrompt("")
	if got != bugReproDefaultPrompt {
		t.Error("empty files content should fall back to bugReproDefaultPrompt")
	}
	got2 := BuildBugReproPrompt("\n  \t  \n")
	if got2 != bugReproDefaultPrompt {
		t.Error("whitespace-only files content should fall back to bugReproDefaultPrompt")
	}
}

// TestBugReproPreset_SynthesizerHasFindingsSlot ensures the
// synthesizer prompt has a substitution slot for per-agent
// findings JSON.
func TestBugReproPreset_SynthesizerHasFindingsSlot(t *testing.T) {
	p, err := defaultRegistry.Find("bug-repro")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(p.Synthesizer, "%s") {
		t.Error("synthesizer must have a per-agent-findings JSON substitution slot")
	}
}
