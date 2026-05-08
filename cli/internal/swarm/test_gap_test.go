package swarm

import (
	"strings"
	"testing"
)

// TestTestGapPreset_Registers ensures the v0.12 preset is in the
// default registry and findable by name. Catches accidental
// dropped registration in cli/internal/swarm/registry.go::init().
func TestTestGapPreset_Registers(t *testing.T) {
	p, err := defaultRegistry.Find("test-gap")
	if err != nil {
		t.Fatalf("Find(test-gap): %v", err)
	}
	if p.Name != "test-gap" {
		t.Errorf("preset.Name = %q, want test-gap", p.Name)
	}
}

// TestTestGapPreset_DefaultPromptHasInputSlot ensures the default
// (registry-loaded) prompt has exactly one %s slot for the
// code-under-test substitution at dispatch time.
func TestTestGapPreset_DefaultPromptHasInputSlot(t *testing.T) {
	p, err := defaultRegistry.Find("test-gap")
	if err != nil {
		t.Fatal(err)
	}
	count := strings.Count(p.DefaultPrompt, "%s")
	if count != 1 {
		t.Errorf("DefaultPrompt has %d %%s slots, want 1", count)
	}
}

// TestTestGapPreset_SeverityVocab pins the cross-preset severity
// consistency contract from spec Decision (Stage 1: severity vocab).
// All Tier A presets share pr-review/reverse vocab.
func TestTestGapPreset_SeverityVocab(t *testing.T) {
	p, err := defaultRegistry.Find("test-gap")
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

// TestBuildTestGapPrompt_TwoSlotPattern covers the safety net
// against the v0.8 dreamLensWild incident: literal `%` chars in
// tests content escaped to `%%` BEFORE substitution so the
// downstream code-slot fmt.Sprintf doesn't interpret tests-content
// percent signs as format directives.
func TestBuildTestGapPrompt_TwoSlotPattern(t *testing.T) {
	tests := "// expects: 10% improvement under load"
	prompt := BuildTestGapPrompt(tests)
	// {{TESTS_CONTENT}} replaced by escaped tests; one %s slot
	// remains for the code-under-test.
	if strings.Contains(prompt, "{{TESTS_CONTENT}}") {
		t.Error("placeholder not substituted")
	}
	if !strings.Contains(prompt, "10%% improvement") {
		t.Error("literal %% should be escaped to %%%% so downstream fmt.Sprintf is safe")
	}
	if strings.Count(prompt, "%s") != 1 {
		t.Errorf("expected exactly 1 %%s slot for code-substitution; got %d", strings.Count(prompt, "%s"))
	}
}

// TestBuildTestGapPrompt_EmptyTestsFallsBackToDefault covers
// library-caller use where no --tests is provided. Spec: graceful
// degradation, NOT hard fail. Falls back to the registry's
// DefaultPrompt (code-only audit).
func TestBuildTestGapPrompt_EmptyTestsFallsBackToDefault(t *testing.T) {
	got := BuildTestGapPrompt("")
	if got != testGapDefaultPrompt {
		t.Error("empty tests content should fall back to testGapDefaultPrompt")
	}
	got2 := BuildTestGapPrompt("   \n\t  ")
	if got2 != testGapDefaultPrompt {
		t.Error("whitespace-only tests content should fall back to testGapDefaultPrompt")
	}
}

// TestTestGapPreset_SynthesizerHasFindingsSlot ensures the
// synthesizer prompt has a %s slot for per-agent findings JSON
// substitution. Standard contract for all preset synthesizers.
func TestTestGapPreset_SynthesizerHasFindingsSlot(t *testing.T) {
	p, err := defaultRegistry.Find("test-gap")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(p.Synthesizer, "%s") {
		t.Error("synthesizer must have a per-agent-findings JSON substitution slot")
	}
}
