package swarm

import (
	"strings"
	"testing"
)

// TestDreamPreset_Registered verifies the dream preset is in the
// default registry — `jutsu swarm dream` resolution depends on this.
func TestDreamPreset_Registered(t *testing.T) {
	p, err := PresetFor("dream")
	if err != nil {
		t.Fatalf("dream preset not registered: %v", err)
	}
	if p.Name != "dream" {
		t.Fatalf("preset.Name = %q, want dream", p.Name)
	}
	if p.InputKind != InputPrompt {
		t.Errorf("dream InputKind = %v, want InputPrompt", p.InputKind)
	}
}

// TestDreamLensesBase covers the canonical 4-lens default + LEAD
// position. The first element is the LEAD lens for v0.8.0's
// rotation rule (Stage 3); test pins the order.
func TestDreamLensesBase(t *testing.T) {
	want := []string{"honest", "fit", "gaps", "wild"}
	got := DreamLensesBase()
	if len(got) != len(want) {
		t.Fatalf("DreamLensesBase len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("DreamLensesBase[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestDreamLensesAll verifies the 8-lens superset and ordering: base
// 4 first, extras 4 after. Ordering matters for the LEAD-rotation
// rule + for stable prompt assembly.
func TestDreamLensesAll(t *testing.T) {
	want := []string{"honest", "fit", "gaps", "wild", "adversary", "inverse", "status-quo", "time"}
	got := DreamLensesAll()
	if len(got) != len(want) {
		t.Fatalf("DreamLensesAll len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("DreamLensesAll[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestIsValidDreamLens covers the 8 known + a handful of typos.
// `null` is intentionally invalid (was renamed to `status-quo` in
// the doc-review squash to dodge JSON/YAML/Go reserved-word
// collisions).
func TestIsValidDreamLens(t *testing.T) {
	for _, name := range DreamLensesAll() {
		if !IsValidDreamLens(name) {
			t.Errorf("IsValidDreamLens(%q) = false, want true", name)
		}
	}
	for _, bad := range []string{"null", "honesty", "Wild", "", "premortem"} {
		if IsValidDreamLens(bad) {
			t.Errorf("IsValidDreamLens(%q) = true, want false (typo / reserved-word)", bad)
		}
	}
}

// TestBuildDreamPrompt_BaseHasAntiSycophancy guards against accidental
// softening of the anti-sycophancy preamble. The exact phrases live
// in dreamSharedHeader; this test asserts they survive any future
// edit.
func TestBuildDreamPrompt_BaseHasAntiSycophancy(t *testing.T) {
	prompt := BuildDreamPrompt(DreamLensesBase())
	for _, phrase := range []string{
		`"Great question!"`,
		`"You're absolutely right!"`,
		`"This is interesting"`,
		"hedging that means nothing",
	} {
		if !strings.Contains(prompt, phrase) {
			t.Errorf("dream prompt missing anti-sycophancy phrase %q — preamble was softened", phrase)
		}
	}
}

// TestBuildDreamPrompt_ContainsLensFooter verifies the trailing
// fmt.Sprintf placeholder is present so the swarm dispatch path can
// substitute the topic via fmt.Sprintf(template, topic).
func TestBuildDreamPrompt_ContainsLensFooter(t *testing.T) {
	prompt := BuildDreamPrompt(DreamLensesBase())
	if !strings.Contains(prompt, "TOPIC:") {
		t.Errorf("dream prompt missing TOPIC: marker:\n%s", prompt[:min(len(prompt), 200)])
	}
	// Single %s — the topic is substituted exactly once.
	if c := strings.Count(prompt, "%s"); c != 1 {
		t.Errorf("dream prompt has %d %%s placeholders, want exactly 1", c)
	}
}

// TestBuildDreamPrompt_LensSelection verifies that --lenses honest,
// gaps produces a prompt with ONLY those two lenses' content blocks.
func TestBuildDreamPrompt_LensSelection(t *testing.T) {
	prompt := BuildDreamPrompt([]string{"honest", "gaps"})
	if !strings.Contains(prompt, "LENS: honest") {
		t.Error("expected LENS: honest in prompt")
	}
	if !strings.Contains(prompt, "LENS: gaps") {
		t.Error("expected LENS: gaps in prompt")
	}
	for _, missing := range []string{"LENS: fit", "LENS: wild", "LENS: adversary"} {
		if strings.Contains(prompt, missing) {
			t.Errorf("prompt should NOT contain %q for honest+gaps selection", missing)
		}
	}
}

// TestBuildDreamPrompt_AllLensesPresent verifies the all-8 path
// includes every lens content block + the matrix-ready header.
func TestBuildDreamPrompt_AllLensesPresent(t *testing.T) {
	prompt := BuildDreamPrompt(DreamLensesAll())
	for _, lens := range DreamLensesAll() {
		want := "LENS: " + lens
		if !strings.Contains(prompt, want) {
			t.Errorf("all-lenses prompt missing %q", want)
		}
	}
}

// TestDreamPreset_SeverityVocabIsKaijutsuStandard verifies dream uses
// the standard kaijutsu vocabulary (blocker / issue / minor / info)
// rather than inventing a dream-specific severity set. v0.7 findings
// DB schema validation depends on this.
func TestDreamPreset_SeverityVocabIsKaijutsuStandard(t *testing.T) {
	want := map[Severity]bool{
		SeverityBlocker: true,
		SeverityIssue:   true,
		SeverityMinor:   true,
		SeverityInfo:    true,
	}
	got := map[Severity]bool{}
	for _, s := range dreamPreset.SeverityVocab {
		got[s] = true
	}
	if len(want) != len(got) {
		t.Fatalf("dream SeverityVocab len = %d, want %d", len(got), len(want))
	}
	for s := range want {
		if !got[s] {
			t.Errorf("dream SeverityVocab missing %q", s)
		}
	}
}

// TestDreamSynthesizer_ContainsHardStop guards against accidental
// removal of the synthesizer's hard-stop rule that prevents the
// trailing-coda failure mode.
func TestDreamSynthesizer_ContainsHardStop(t *testing.T) {
	if !strings.Contains(dreamSynthesizer, "HARD STOP RULE") {
		t.Error("dream synthesizer missing 'HARD STOP RULE' — coda guardrail softened")
	}
	if !strings.Contains(dreamSynthesizer, "End the output at the last section") {
		t.Error("dream synthesizer missing the end-at-last-section directive")
	}
}
