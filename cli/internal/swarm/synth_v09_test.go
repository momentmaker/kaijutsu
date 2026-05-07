package swarm

import (
	"strings"
	"testing"
)

// TestBuildSynthPrompt_DreamLensWeightsEmitted covers the v0.9
// adaptive-lens-weighting path: when preset is dream + LensWeights
// non-empty + killswitch off, the prompt prepends a `lens_weights:`
// block + the tier rule.
func TestBuildSynthPrompt_DreamLensWeightsEmitted(t *testing.T) {
	t.Setenv("KAIJUTSU_DREAM_ADAPTIVE_LENS", "")
	preset := &Preset{Name: "dream", Synthesizer: "core:\n%s"}
	body := []byte(`x`)
	opts := SynthOpts{
		LensWeights: map[string]float64{
			"honest": 0.91,
			"gaps":   0.42,
		},
	}
	prompt := buildSynthPrompt(preset, body, opts)
	if !strings.Contains(prompt, "lens_weights ") {
		t.Errorf("dream preset with lens weights should emit lens_weights: section; got:\n%s", prompt)
	}
	if !strings.Contains(prompt, "honest: 0.91") {
		t.Errorf("dream preset prompt should include per-lens weight line; got:\n%s", prompt)
	}
	if !strings.Contains(prompt, "tier rule") {
		t.Errorf("dream preset prompt should include tier rule; got:\n%s", prompt)
	}
}

// TestBuildSynthPrompt_NonDreamSuppressesLensWeights covers the
// preset-gate: lens weights map is honored ONLY when preset.Name ==
// "dream". Other presets ignore it (the lens column is dream-only).
func TestBuildSynthPrompt_NonDreamSuppressesLensWeights(t *testing.T) {
	preset := &Preset{Name: "pr-review", Synthesizer: "core:\n%s"}
	body := []byte(`x`)
	opts := SynthOpts{
		LensWeights: map[string]float64{"honest": 0.91},
	}
	prompt := buildSynthPrompt(preset, body, opts)
	if strings.Contains(prompt, "lens_weights ") {
		t.Errorf("non-dream preset must not emit lens_weights section; got:\n%s", prompt)
	}
}

// TestBuildSynthPrompt_KillswitchSuppressesLensWeights covers the
// KAIJUTSU_DREAM_ADAPTIVE_LENS=off env-var: when set, the lens-
// weights block is suppressed even on the dream preset. Output
// matches v0.8.3 byte-for-byte.
func TestBuildSynthPrompt_KillswitchSuppressesLensWeights(t *testing.T) {
	t.Setenv("KAIJUTSU_DREAM_ADAPTIVE_LENS", "off")
	preset := &Preset{Name: "dream", Synthesizer: "core:\n%s"}
	body := []byte(`x`)
	opts := SynthOpts{
		LensWeights: map[string]float64{"honest": 0.91},
	}
	prompt := buildSynthPrompt(preset, body, opts)
	if strings.Contains(prompt, "lens_weights ") {
		t.Errorf("killswitch=off should suppress lens_weights; got:\n%s", prompt)
	}
}

// TestBuildSynthPrompt_EmptyLensWeightsMatchesV083 verifies the empty
// LensWeights map produces byte-identical output to v0.8.3 for dream.
func TestBuildSynthPrompt_EmptyLensWeightsMatchesV083(t *testing.T) {
	t.Setenv("KAIJUTSU_DREAM_ADAPTIVE_LENS", "")
	preset := &Preset{Name: "dream", Synthesizer: "core:\n%s"}
	body := []byte(`x`)

	with := buildSynthPrompt(preset, body, SynthOpts{})
	without := buildSynthPrompt(preset, body, SynthOpts{LensWeights: map[string]float64{}})
	if with != without {
		t.Errorf("empty vs nil LensWeights produced different prompts:\nempty:\n%s\nnil:\n%s", without, with)
	}
}

// TestBuildSynthPrompt_LensWeightsCanonicalOrder pins the determinism
// criterion: lens weights are emitted in canonical 8-lens cycle order
// regardless of map iteration order.
func TestBuildSynthPrompt_LensWeightsCanonicalOrder(t *testing.T) {
	t.Setenv("KAIJUTSU_DREAM_ADAPTIVE_LENS", "")
	preset := &Preset{Name: "dream", Synthesizer: "%s"}
	body := []byte(``)
	// Insert in non-canonical order; expect canonical emission.
	opts := SynthOpts{
		LensWeights: map[string]float64{
			"time":   0.5,
			"honest": 0.9,
			"gaps":   0.3,
			"fit":    0.7,
		},
	}
	prompt := buildSynthPrompt(preset, body, opts)
	// Find positions of each lens line in the prompt.
	posHonest := strings.Index(prompt, "- honest:")
	posFit := strings.Index(prompt, "- fit:")
	posGaps := strings.Index(prompt, "- gaps:")
	posTime := strings.Index(prompt, "- time:")
	if posHonest < 0 || posFit < 0 || posGaps < 0 || posTime < 0 {
		t.Fatalf("missing lens line; prompt:\n%s", prompt)
	}
	if !(posHonest < posFit && posFit < posGaps && posGaps < posTime) {
		t.Errorf("canonical order violated: honest=%d fit=%d gaps=%d time=%d", posHonest, posFit, posGaps, posTime)
	}
}

// TestApplyConfidenceFloor_DropsBelowFloor verifies the v0.9 floor:
// findings with non-zero confidence below the floor are dropped;
// confidence == 0 (agent didn't emit) and confidence >= floor are
// kept. Empty floor (0) is a no-op.
func TestApplyConfidenceFloor_DropsBelowFloor(t *testing.T) {
	results := []AgentResult{
		{Agent: "a", Findings: []Finding{
			{Summary: "high", Confidence: 0.9},
			{Summary: "mid", Confidence: 0.5},
			{Summary: "low", Confidence: 0.2},
			{Summary: "no-conf", Confidence: 0}, // agent didn't emit
		}},
	}

	// Floor 0 = no-op.
	got := applyConfidenceFloor(results, 0)
	if len(got[0].Findings) != 4 {
		t.Errorf("floor=0 should be no-op; kept %d, want 4", len(got[0].Findings))
	}

	// Floor 0.3 drops "low" (0.2), keeps "no-conf" (0.0 = unset).
	got = applyConfidenceFloor(results, 0.3)
	if len(got[0].Findings) != 3 {
		t.Errorf("floor=0.3 should keep 3; got %d", len(got[0].Findings))
	}
	for _, f := range got[0].Findings {
		if f.Summary == "low" {
			t.Errorf("'low' (0.2) should have been dropped")
		}
	}

	// Floor 1.0 drops everything with non-zero confidence.
	got = applyConfidenceFloor(results, 1.0)
	if len(got[0].Findings) != 1 {
		t.Errorf("floor=1.0 should keep only 'no-conf'; got %d", len(got[0].Findings))
	}
	if got[0].Findings[0].Summary != "no-conf" {
		t.Errorf("floor=1.0 should keep 'no-conf'; got %q", got[0].Findings[0].Summary)
	}
}

// TestBuildSynthPrompt_AgentWeightsAndLensWeightsBothEmit covers the
// composition path: warm v0.7 agent weights + warm v0.9 lens weights
// produce TWO weights sections in the prompt (agent first, lens
// second), in deterministic order.
func TestBuildSynthPrompt_AgentWeightsAndLensWeightsBothEmit(t *testing.T) {
	t.Setenv("KAIJUTSU_DREAM_ADAPTIVE_LENS", "")
	preset := &Preset{Name: "dream", Synthesizer: "core:\n%s"}
	body := []byte(`x`)
	opts := SynthOpts{
		Weights:     map[string]float64{"claude": 0.85},
		LensWeights: map[string]float64{"honest": 0.91},
	}
	prompt := buildSynthPrompt(preset, body, opts)
	posAgent := strings.Index(prompt, "weights ")
	posLens := strings.Index(prompt, "lens_weights ")
	if posAgent < 0 {
		t.Errorf("agent weights section missing; got:\n%s", prompt)
	}
	if posLens < 0 {
		t.Errorf("lens weights section missing; got:\n%s", prompt)
	}
	if posAgent > posLens {
		t.Errorf("agent weights should appear BEFORE lens weights; agent=%d lens=%d", posAgent, posLens)
	}
}
