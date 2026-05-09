package swarm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeYaml(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

const validProjectYaml = `
- name: my-tight-review
  description: shorter pr-review
  inputKind: diff
  defaultPrompt: "review %s"
  synthesizer: "synthesize %s"
  severityVocab: [blocker, issue, minor, info]
  mode: quick
  personas: [paranoid-security-claude]
  confidenceFloor: 0.55
`

const validHomeYaml = `
- name: my-personal-quick-audit
  description: personal lens
  inputKind: prompt
  defaultPrompt: "audit %s"
  synthesizer: "synthesize %s"
  severityVocab: [issue, minor]
- name: my-tight-review
  description: home version (will be shadowed by project)
  inputKind: diff
  defaultPrompt: "home review %s"
  synthesizer: "home synthesize %s"
  severityVocab: [issue]
`

func TestLoadUserPresets_ProjectShadowsHome(t *testing.T) {
	tmp := t.TempDir()
	homeDir := filepath.Join(tmp, "home")
	projectRoot := filepath.Join(tmp, "project")

	writeYaml(t, filepath.Join(homeDir, ".kaijutsu", "swarm.yaml"), validHomeYaml)
	writeYaml(t, filepath.Join(projectRoot, ".kaijutsu", "swarm.yaml"), validProjectYaml)

	presets, warnings, err := LoadUserPresets(projectRoot, homeDir)
	if err != nil {
		t.Fatalf("LoadUserPresets: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
	// my-tight-review from project shadows home.
	if p, ok := presets["my-tight-review"]; !ok {
		t.Fatal("my-tight-review missing from merged presets")
	} else if p.Source != SourceProject {
		t.Errorf("my-tight-review Source = %q, want %q (project should shadow home)", p.Source, SourceProject)
	} else if strings.Contains(p.DefaultPrompt, "home review") {
		t.Errorf("my-tight-review.DefaultPrompt should be project's (which has 'review %%s'), not home's (which has 'home review %%s'); got %q", p.DefaultPrompt)
	} else if !strings.HasPrefix(p.DefaultPrompt, "review ") {
		t.Errorf("my-tight-review.DefaultPrompt should start with 'review '; got %q", p.DefaultPrompt)
	}
	// Field round-trip: yaml-set values for Mode / Personas /
	// ConfidenceFloor must populate the UserPreset (not silently drop).
	// Final-pr-review pass caught these missing.
	p := presets["my-tight-review"]
	if p.Mode != "quick" {
		t.Errorf("Mode = %q, want quick", p.Mode)
	}
	if len(p.Personas) != 1 || p.Personas[0] != "paranoid-security-claude" {
		t.Errorf("Personas = %v, want [paranoid-security-claude]", p.Personas)
	}
	if p.ConfidenceFloor != 0.55 {
		t.Errorf("ConfidenceFloor = %v, want 0.55", p.ConfidenceFloor)
	}
	// my-personal-quick-audit only in home.
	if p, ok := presets["my-personal-quick-audit"]; !ok {
		t.Fatal("my-personal-quick-audit missing")
	} else if p.Source != SourceHome {
		t.Errorf("my-personal-quick-audit Source = %q, want %q", p.Source, SourceHome)
	}
}

func TestLoadUserPresets_HomeOnlyLoads(t *testing.T) {
	tmp := t.TempDir()
	homeDir := filepath.Join(tmp, "home")
	projectRoot := filepath.Join(tmp, "project") // exists but no .kaijutsu dir

	writeYaml(t, filepath.Join(homeDir, ".kaijutsu", "swarm.yaml"), validHomeYaml)
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	presets, warnings, err := LoadUserPresets(projectRoot, homeDir)
	if err != nil {
		t.Fatalf("LoadUserPresets: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
	if len(presets) == 0 {
		t.Error("home-only yaml should load 2 presets; got 0")
	}
	for _, p := range presets {
		if p.Source != SourceHome {
			t.Errorf("preset %q has Source = %q, want %q", p.Name, p.Source, SourceHome)
		}
	}
}

func TestLoadUserPresets_ProjectOnlyLoads(t *testing.T) {
	tmp := t.TempDir()
	homeDir := filepath.Join(tmp, "home") // exists but no .kaijutsu dir
	projectRoot := filepath.Join(tmp, "project")
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeYaml(t, filepath.Join(projectRoot, ".kaijutsu", "swarm.yaml"), validProjectYaml)

	presets, _, err := LoadUserPresets(projectRoot, homeDir)
	if err != nil {
		t.Fatal(err)
	}
	if p, ok := presets["my-tight-review"]; !ok {
		t.Fatal("project-only preset missing")
	} else if p.Source != SourceProject {
		t.Errorf("Source = %q, want %q", p.Source, SourceProject)
	}
}

func TestLoadUserPresets_MissingFilesNoError(t *testing.T) {
	tmp := t.TempDir()
	homeDir := filepath.Join(tmp, "home")
	projectRoot := filepath.Join(tmp, "project")
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	presets, warnings, err := LoadUserPresets(projectRoot, homeDir)
	if err != nil {
		t.Fatalf("missing files should not error; got %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
	if len(presets) != 0 {
		t.Errorf("missing files should yield empty map; got %d entries", len(presets))
	}
}

func TestLoadUserPresets_MalformedYamlSurfacesAsError(t *testing.T) {
	tmp := t.TempDir()
	homeDir := filepath.Join(tmp, "home")
	projectRoot := filepath.Join(tmp, "project")
	writeYaml(t, filepath.Join(projectRoot, ".kaijutsu", "swarm.yaml"), "this is: not yaml: and: also: not")

	_, _, err := LoadUserPresets(projectRoot, homeDir)
	if err == nil {
		t.Fatal("expected fatal error for malformed yaml")
	}
}

func TestLoadUserPresets_IntraFileDuplicateRejectsFile(t *testing.T) {
	tmp := t.TempDir()
	homeDir := filepath.Join(tmp, "home")
	projectRoot := filepath.Join(tmp, "project")
	dupYaml := `
- name: dup-name
  description: first
  inputKind: prompt
  defaultPrompt: "%s"
  synthesizer: "%s"
  severityVocab: [issue]
- name: dup-name
  description: second
  inputKind: prompt
  defaultPrompt: "%s"
  synthesizer: "%s"
  severityVocab: [minor]
`
	writeYaml(t, filepath.Join(projectRoot, ".kaijutsu", "swarm.yaml"), dupYaml)
	// Home file has its own valid entry — must still load.
	writeYaml(t, filepath.Join(homeDir, ".kaijutsu", "swarm.yaml"), validHomeYaml)

	presets, warnings, err := LoadUserPresets(projectRoot, homeDir)
	if err != nil {
		t.Fatal(err)
	}
	// Project rejected; home survives.
	if _, ok := presets["dup-name"]; ok {
		t.Error("dup-name should be rejected (intra-file duplicate)")
	}
	if _, ok := presets["my-personal-quick-audit"]; !ok {
		t.Error("home preset should still load when project file rejected")
	}
	foundDupCitation := false
	for _, w := range warnings {
		if strings.Contains(w.Error(), "intra-file duplicate") {
			foundDupCitation = true
		}
	}
	if !foundDupCitation {
		t.Errorf("expected duplicate-name citation in warnings; got %v", warnings)
	}
}

func TestLoadUserPresets_InvalidEntrySkipped(t *testing.T) {
	tmp := t.TempDir()
	homeDir := filepath.Join(tmp, "home")
	projectRoot := filepath.Join(tmp, "project")
	mixedYaml := `
- name: valid-one
  description: ok
  inputKind: prompt
  defaultPrompt: "%s"
  synthesizer: "%s"
  severityVocab: [issue]
- name: pr-review
  description: shadows built-in
  inputKind: diff
  defaultPrompt: "%s"
  synthesizer: "%s"
  severityVocab: [issue]
`
	writeYaml(t, filepath.Join(projectRoot, ".kaijutsu", "swarm.yaml"), mixedYaml)

	presets, warnings, err := LoadUserPresets(projectRoot, homeDir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := presets["valid-one"]; !ok {
		t.Error("valid entry should load alongside invalid one")
	}
	if _, ok := presets["pr-review"]; ok {
		t.Error("built-in shadow should be rejected (validation skips entry)")
	}
	if len(warnings) != 1 {
		t.Errorf("expected 1 warning for built-in shadow; got %d: %v", len(warnings), warnings)
	}
}

func TestLoadUserPresets_RequiresOneSlotInDefaultPrompt(t *testing.T) {
	tmp := t.TempDir()
	homeDir := filepath.Join(tmp, "home")
	projectRoot := filepath.Join(tmp, "project")
	yamlBody := `
- name: zero-slots
  description: bad
  inputKind: prompt
  defaultPrompt: "no slot here"
  synthesizer: "%s"
  severityVocab: [issue]
- name: two-slots
  description: also bad
  inputKind: prompt
  defaultPrompt: "two %s and %s slots"
  synthesizer: "%s"
  severityVocab: [issue]
`
	writeYaml(t, filepath.Join(projectRoot, ".kaijutsu", "swarm.yaml"), yamlBody)
	presets, warnings, err := LoadUserPresets(projectRoot, homeDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(presets) != 0 {
		t.Errorf("both entries should fail validation; got %d valid", len(presets))
	}
	if len(warnings) != 2 {
		t.Errorf("expected 2 slot-count warnings; got %d", len(warnings))
	}
}

func TestLoadUserPresets_RequiresOneSlotInSynthesizer(t *testing.T) {
	tmp := t.TempDir()
	homeDir := filepath.Join(tmp, "home")
	projectRoot := filepath.Join(tmp, "project")
	yamlBody := `
- name: zero-synth-slots
  description: bad synth
  inputKind: prompt
  defaultPrompt: "%s"
  synthesizer: "no slot here"
  severityVocab: [issue]
`
	writeYaml(t, filepath.Join(projectRoot, ".kaijutsu", "swarm.yaml"), yamlBody)
	presets, warnings, err := LoadUserPresets(projectRoot, homeDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(presets) != 0 {
		t.Error("missing-synthesizer-slot should fail validation")
	}
	if len(warnings) != 1 {
		t.Errorf("expected 1 warning; got %d", len(warnings))
	}
}

// TestBuiltinPresetNames_StaysInSyncWithRegistryInit guards against
// a new built-in preset landing in registry.go::init() without
// being added to builtinPresetNames. If the two drift, user
// presets could shadow the new built-in silently — defeating
// spec Decision #2's hard-shadow-rejection contract.
func TestBuiltinPresetNames_StaysInSyncWithRegistryInit(t *testing.T) {
	for name := range builtinPresetNames {
		if _, err := defaultRegistry.Find(name); err != nil {
			t.Errorf("builtinPresetNames lists %q but defaultRegistry doesn't have it; user_presets.go is out of sync", name)
		}
	}
	for _, name := range defaultRegistry.Names() {
		// Skip names registered as user presets in test runs.
		if defaultRegistry.UserSource(name) != "" {
			continue
		}
		if !builtinPresetNames[name] {
			t.Errorf("defaultRegistry has built-in %q but builtinPresetNames doesn't list it; user_presets.go::builtinPresetNames is out of sync", name)
		}
	}
}

// TestLoadUserPresets_StrictYamlRejectsUnknownFields pins the
// contract from final pr-review: typos in yaml field names surface
// as parse errors instead of silently mapping to zero-value.
func TestLoadUserPresets_StrictYamlRejectsUnknownFields(t *testing.T) {
	tmp := t.TempDir()
	homeDir := filepath.Join(tmp, "home")
	projectRoot := filepath.Join(tmp, "project")
	typoYaml := `
- name: my-typo
  description: oops
  inputKind: prompt
  defaultPrompt: "%s"
  synthesizer: "%s"
  severityVocab: [issue]
  confidencFloor: 0.5
`
	writeYaml(t, filepath.Join(projectRoot, ".kaijutsu", "swarm.yaml"), typoYaml)

	_, _, err := LoadUserPresets(projectRoot, homeDir)
	if err == nil {
		t.Fatal("expected fatal parse error for unknown field 'confidencFloor'")
	}
	if !strings.Contains(err.Error(), "confidencFloor") {
		t.Errorf("error should name the typo field; got %v", err)
	}
}

// TestCountFormatSlot_HandlesEscapedPercent pins the v0.13 final-
// pr-review fix: `%%s` is an escaped literal (renders as `%s` in
// the output) and must NOT count toward the substitution-slot
// total.
func TestCountFormatSlot_HandlesEscapedPercent(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"plain %s", 1},
		{"escaped %%s only", 0},
		{"mix %%s and %s", 1},
		{"two real %s and %s", 2},
		{"empty string", 0},
		{"escaped at end %%s", 0},
	}
	for _, tc := range cases {
		if got := countFormatSlot(tc.in); got != tc.want {
			t.Errorf("countFormatSlot(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

// TestLoadUserPresets_AcceptsEscapedPercentInPrompt pins the
// behavior change: a user prompt that wants to embed a literal
// `%s` in the rendered prompt uses `%%s` and the validator no
// longer trips on the slot count.
func TestLoadUserPresets_AcceptsEscapedPercentInPrompt(t *testing.T) {
	tmp := t.TempDir()
	homeDir := filepath.Join(tmp, "home")
	projectRoot := filepath.Join(tmp, "project")
	escapedYaml := `
- name: literal-percent-s
  description: prompt with literal %s embed
  inputKind: prompt
  defaultPrompt: "use %%s in your output. body: %s"
  synthesizer: "%s"
  severityVocab: [issue]
`
	writeYaml(t, filepath.Join(projectRoot, ".kaijutsu", "swarm.yaml"), escapedYaml)

	presets, warnings, err := LoadUserPresets(projectRoot, homeDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 {
		t.Errorf("escaped %%s should not produce warnings; got %v", warnings)
	}
	if _, ok := presets["literal-percent-s"]; !ok {
		t.Error("preset with escaped percent-s should load successfully")
	}
}
