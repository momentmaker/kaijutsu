// Package eval implements the v0.10 `jutsu eval` runner. It parses
// agentskills.io-format evals.json files (compat with darkrishabh/
// agent-skills-eval upstream), runs target + judge models via the v0.6
// driver layer, and writes iteration-N artifact trees with a static
// HTML report.
//
// Stage 1: schema + single-skill eval (parity with upstream).
// Stage 2: kaijutsu.{swarm,personas,presets} extension blocks.
// Stage 3: CI integration + stateful --strict via --baseline-from.
//
// Spec: docs/specs/2026-05-08-v0.10.0-eval-runner.md
// Plan: IMPLEMENTATION_PLAN.md
package eval

// Suite is one parsed evals.json file. The top-level shape mirrors
// agent-skills-eval upstream verbatim so kaijutsu reads upstream files
// unchanged. Kaijutsu-specific extensions live under Suite.Kaijutsu;
// upstream parsers ignore that field via JSON-permissive parsing.
type Suite struct {
	SkillName string             `json:"skill_name"`
	Evals     []Eval             `json:"evals"`
	Kaijutsu  *KaijutsuExtension `json:"kaijutsu,omitempty"`
}

// Eval is one eval case in the upstream-shape evals[] array.
//
// Required: ID, Name, Prompt.
// Optional:
//   - Files — file paths attached to the prompt (relative to the
//     evals.json directory).
//   - ExpectedOutput — description of desired behavior. Auto-promoted
//     to a single assertion via the literal wrap
//     `"Output should satisfy expected behavior: " + ExpectedOutput`
//     when Assertions is empty.
//   - Assertions — explicit grading criteria. Each assertion gets ONE
//     judge call; pass = AND of all assertions.
type Eval struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Prompt         string   `json:"prompt"`
	Files          []string `json:"files,omitempty"`
	ExpectedOutput string   `json:"expected_output,omitempty"`
	Assertions     []string `json:"assertions,omitempty"`
}

// KaijutsuExtension carries the v0.10 Stage-2 swarm-shape eval
// blocks. Unrecognized fields are ignored by upstream parsers per
// the forward-compat contract (verified by Stage 2's
// TestKaijutsuExtensionBlock_IgnoredByUpstreamParser). Stage 1
// declares the type but leaves the slices empty + unused.
type KaijutsuExtension struct {
	Swarm    []SwarmCase    `json:"swarm,omitempty"`
	Personas []PersonaCase  `json:"personas,omitempty"`
	Presets  []PresetCase   `json:"presets,omitempty"`
}

// SwarmCase tests a (skill, swarm preset) combination — does the
// skill loaded into a swarm preset's pipeline outperform the same
// preset without the skill? Baseline + Challenger name the side
// directory labels in the artifact tree (e.g.
// without_skill_in_swarm/, with_skill_in_swarm/).
//
// Stage 2 wires this; Stage 1 ships the type signature only.
type SwarmCase struct {
	ID         string   `json:"id"`
	Preset     string   `json:"preset"`
	Diff       string   `json:"diff,omitempty"`
	Files      []string `json:"files,omitempty"`
	Prompt     string   `json:"prompt,omitempty"`
	Baseline   string   `json:"baseline"`   // e.g. "without_skill"
	Challenger string   `json:"challenger"` // e.g. "with_skill"
	Assertions []string `json:"assertions"`
}

// PersonaCase compares two personas head-to-head on the same prompt.
// Stage 2 wires this; Stage 1 ships the type signature only.
type PersonaCase struct {
	ID         string   `json:"id"`
	Baseline   string   `json:"baseline"`   // persona name
	Challenger string   `json:"challenger"` // persona name
	Prompt     string   `json:"prompt"`
	Files      []string `json:"files,omitempty"`
	Assertions []string `json:"assertions"`
}

// PresetCase compares two preset modes (e.g. quick vs full) on the
// same prompt. Stage 2 wires this; Stage 1 ships the type signature
// only.
type PresetCase struct {
	ID         string   `json:"id"`
	Preset     string   `json:"preset"`
	Baseline   string   `json:"baseline"`   // mode name
	Challenger string   `json:"challenger"` // mode name
	Prompt     string   `json:"prompt"`
	Files      []string `json:"files,omitempty"`
	Assertions []string `json:"assertions"`
}
