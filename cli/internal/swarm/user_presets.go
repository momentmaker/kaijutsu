// user_presets.go — v0.13.0 Preset SDK loader.
//
// Reads `.kaijutsu/swarm.yaml` (per-project) + `~/.kaijutsu/swarm.yaml`
// (per-user), validates each entry against the same field contracts
// the JSON schema enforces, and returns the merged map. Project
// entries shadow home entries on cross-file collision; intra-file
// duplicate names reject the WHOLE file with a citation.
//
// Spec: docs/specs/2026-05-08-v0.13.0-preset-sdk.md.
package swarm

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// userPresetNamePattern matches the schema's name pattern. Lowercase,
// alphanumeric + hyphens; can't start or end with hyphen.
var userPresetNamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]*[a-z0-9]$`)

// UserPresetSource indicates where a user preset came from. Used
// only for help-text rendering (Decision #8). Never affects
// dispatch behavior.
type UserPresetSource string

const (
	SourceProject UserPresetSource = "user:project" // .kaijutsu/swarm.yaml
	SourceHome    UserPresetSource = "user:home"    // ~/.kaijutsu/swarm.yaml
)

// UserPreset wraps a built-in Preset value with its provenance +
// dispatch-time defaults that aren't fields on Preset itself.
//
// Mode / Personas / ConfidenceFloor are flag values on
// commonSwarmFlags / PipelineOpts at dispatch time, NOT preset-
// baked fields. The cobra layer reads UserPreset.{Mode,Personas,
// ConfidenceFloor} when constructing the subcommand and uses
// them as the DEFAULT flag values (overridable per-invocation
// via --mode / --personas / --confidence-threshold flags).
type UserPreset struct {
	Preset
	Source          UserPresetSource
	Mode            string   // "quick" | "full"; default flag value for --mode
	Personas        []string // default flag value for --personas
	ConfidenceFloor float64  // default flag value for --confidence-threshold
}

// builtinPresetNames is the set of names a user preset MUST NOT
// shadow per spec Decision #2. Kept in sync with registry.go's
// init() registrations.
var builtinPresetNames = map[string]bool{
	"pr-review":        true,
	"doc-review":       true,
	"brainstorm":       true,
	"refactor-plan":    true,
	"security-audit":   true,
	"dream":            true,
	"reverse":          true,
	"test-gap":         true,
	"bug-repro":        true,
	"code-archaeology": true,
}

// validSeverityValues is the predefined severity set per spec
// Decision #4. Kept in sync with the schema's enum.
var validSeverityValues = map[string]bool{
	"blocker": true, "issue": true, "minor": true, "info": true,
	"critical": true, "high": true, "medium": true, "low": true, "informational": true,
	"recommended": true, "alternative": true, "risky": true, "speculative": true,
}

// userPresetYAML mirrors the yaml shape expected on disk. Field
// names match the JSON schema (camelCase via yaml tags).
type userPresetYAML struct {
	Name            string   `yaml:"name"`
	Description     string   `yaml:"description"`
	InputKind       string   `yaml:"inputKind"`
	DefaultPrompt   string   `yaml:"defaultPrompt"`
	Synthesizer     string   `yaml:"synthesizer"`
	SeverityVocab   []string `yaml:"severityVocab"`
	Mode            string   `yaml:"mode,omitempty"`
	Personas        []string `yaml:"personas,omitempty"`
	ConfidenceFloor float64  `yaml:"confidenceFloor,omitempty"`
}

// LoadUserPresets reads project + home swarm.yaml files, validates
// each entry against the field contracts (schema + name-collision
// vs built-ins), and returns the merged map.
//
// Both directory paths are explicit args (no os.UserHomeDir() call
// inside) so tests can inject any tmp dir. The cli wrapper resolves
// homeDir once via os.UserHomeDir() before calling.
//
// Returns:
//   - presets: map[name]*UserPreset of validated, non-shadowing entries
//   - warnings: per-entry validation issues; entry SKIPPED but other
//     entries in the same file still load EXCEPT for intra-file
//     duplicate-name errors which reject the WHOLE file
//   - err: fatal yaml-parse-level errors (file unreadable, malformed
//     YAML; not validation errors)
//
// Project entries shadow home entries with the same name.
func LoadUserPresets(projectRoot, homeDir string) (map[string]*UserPreset, []error, error) {
	out := map[string]*UserPreset{}
	var allWarnings []error

	// Home first; project shadows it.
	homePath := filepath.Join(homeDir, ".kaijutsu", "swarm.yaml")
	projectPath := filepath.Join(projectRoot, ".kaijutsu", "swarm.yaml")

	// Same-dir guard: if homeDir == projectRoot, both paths point at
	// the same file. Load once + tag with SourceProject so the
	// project-shadows-home rule is visually preserved (project
	// takes precedence in single-source mode, matching what a real
	// project-only setup looks like).
	if homePath == projectPath {
		entries, warnings, err := loadUserPresetsFile(projectPath, SourceProject)
		if err != nil {
			return nil, nil, fmt.Errorf("load %s: %w", projectPath, err)
		}
		return entries, warnings, nil
	}

	homeEntries, homeWarnings, homeErr := loadUserPresetsFile(homePath, SourceHome)
	if homeErr != nil {
		return nil, nil, fmt.Errorf("load %s: %w", homePath, homeErr)
	}
	allWarnings = append(allWarnings, homeWarnings...)
	for name, p := range homeEntries {
		out[name] = p
	}

	projectEntries, projectWarnings, projectErr := loadUserPresetsFile(projectPath, SourceProject)
	if projectErr != nil {
		return nil, nil, fmt.Errorf("load %s: %w", projectPath, projectErr)
	}
	allWarnings = append(allWarnings, projectWarnings...)
	// Project shadows home.
	for name, p := range projectEntries {
		out[name] = p
	}

	return out, allWarnings, nil
}

// loadUserPresetsFile reads + validates a single swarm.yaml. Missing
// file → empty map + nil error (not a problem; the file is optional).
// Malformed yaml → fatal error. Intra-file duplicate name → REJECT
// THE WHOLE FILE with a single warning citing both occurrences.
// Per-entry validation issues skip the offending entry but other
// entries in the same file still load.
func loadUserPresetsFile(path string, source UserPresetSource) (map[string]*UserPreset, []error, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]*UserPreset{}, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("read: %w", err)
	}

	// Strict decoder: yaml.v3's KnownFields(true) rejects unknown
	// keys so typos like `inputkind:` (vs `inputKind:`) or
	// `confidencFloor:` (vs `confidenceFloor:`) surface as parse
	// errors instead of silently mapping to zero-value. Matches
	// the schema's `additionalProperties: false` contract.
	var entries []userPresetYAML
	dec := yaml.NewDecoder(strings.NewReader(string(data)))
	dec.KnownFields(true)
	if err := dec.Decode(&entries); err != nil {
		return nil, nil, fmt.Errorf("parse yaml: %w", err)
	}

	// Detect intra-file duplicates BEFORE validating individual entries.
	// Per Decision #6: reject the whole file with a citation; other
	// yaml file (project vs home) still loads independently.
	seen := map[string]int{}
	for i, e := range entries {
		if e.Name == "" {
			continue // empty-name issue caught in per-entry validation below
		}
		if first, dup := seen[e.Name]; dup {
			return nil,
				[]error{fmt.Errorf("%s: intra-file duplicate name %q (entries %d and %d); WHOLE FILE rejected — pick unique names", path, e.Name, first, i)},
				nil
		}
		seen[e.Name] = i
	}

	out := map[string]*UserPreset{}
	var warnings []error
	for i, e := range entries {
		preset, vErr := validateUserPresetEntry(e, source)
		if vErr != nil {
			warnings = append(warnings, fmt.Errorf("%s entry %d (%q): %w", path, i, e.Name, vErr))
			continue
		}
		out[e.Name] = preset
	}
	return out, warnings, nil
}

// validateUserPresetEntry maps a parsed yaml entry → UserPreset
// after enforcing every field contract from spec Decision #4 + the
// schema. Returns a single concatenated error for any failure so
// callers get the full picture in one warning.
func validateUserPresetEntry(e userPresetYAML, source UserPresetSource) (*UserPreset, error) {
	if e.Name == "" {
		return nil, errors.New("name is required")
	}
	if !userPresetNamePattern.MatchString(e.Name) {
		return nil, fmt.Errorf("name %q must match %s", e.Name, userPresetNamePattern.String())
	}
	if builtinPresetNames[e.Name] {
		return nil, fmt.Errorf("name %q collides with built-in preset (built-ins: %s); pick a different name", e.Name, strings.Join(builtinPresetNamesList(), ", "))
	}
	if strings.TrimSpace(e.Description) == "" {
		return nil, errors.New("description is required")
	}
	var inputKind InputKind
	switch e.InputKind {
	case "diff":
		inputKind = InputDiff
	case "files":
		inputKind = InputFiles
	case "prompt":
		inputKind = InputPrompt
	default:
		return nil, fmt.Errorf("inputKind %q must be one of: diff, files, prompt", e.InputKind)
	}
	if strings.TrimSpace(e.DefaultPrompt) == "" {
		return nil, errors.New("defaultPrompt is required")
	}
	if c := countFormatSlot(e.DefaultPrompt); c != 1 {
		return nil, fmt.Errorf("defaultPrompt has %d %%s slot(s); must have exactly 1 (input substituted at dispatch time)", c)
	}
	if strings.TrimSpace(e.Synthesizer) == "" {
		return nil, errors.New("synthesizer is required")
	}
	if c := countFormatSlot(e.Synthesizer); c != 1 {
		return nil, fmt.Errorf("synthesizer has %d %%s slot(s); must have exactly 1 (per-agent findings substituted at synthesis time)", c)
	}
	if len(e.SeverityVocab) == 0 {
		return nil, errors.New("severityVocab must be non-empty")
	}
	severityVocab := make([]Severity, 0, len(e.SeverityVocab))
	for _, s := range e.SeverityVocab {
		if !validSeverityValues[s] {
			return nil, fmt.Errorf("severityVocab entry %q is not a recognized severity; valid: %s", s, validSeverityValuesList())
		}
		severityVocab = append(severityVocab, Severity(s))
	}
	mode := e.Mode
	if mode == "" {
		mode = "quick"
	}
	if mode != "quick" && mode != "full" {
		return nil, fmt.Errorf("mode %q must be quick or full", e.Mode)
	}
	if e.ConfidenceFloor < 0.0 || e.ConfidenceFloor > 1.0 {
		return nil, fmt.Errorf("confidenceFloor %f must be in [0.0, 1.0]", e.ConfidenceFloor)
	}

	preset := Preset{
		Name:          e.Name,
		Description:   e.Description,
		InputKind:     inputKind,
		DefaultPrompt: e.DefaultPrompt,
		Synthesizer:   e.Synthesizer,
		SeverityVocab: severityVocab,
		// User presets reuse the artifact-agnostic genericDebateTemplate
		// from v0.12 — custom debate templates aren't yaml-configurable
		// in v0.13 (deferred to v0.14+ per spec Out-of-scope).
		Debate: genericDebateTemplate,
	}
	return &UserPreset{
		Preset:          preset,
		Source:          source,
		Mode:            mode,
		Personas:        e.Personas,
		ConfidenceFloor: e.ConfidenceFloor,
	}, nil
}

// countFormatSlot counts unescaped `%s` occurrences. Treats `%%s`
// as an escaped literal (the user wants `%s` to appear in the
// rendered prompt as text, not be substituted), so `%%s` does NOT
// count toward the slot total. Used by validation to enforce the
// "exactly one substitution slot" contract from spec Decision #4.
//
// Implementation: strip every `%%` first (yields the verbatim
// percent literal), then count `%s` in the residue.
func countFormatSlot(s string) int {
	stripped := strings.ReplaceAll(s, "%%", "")
	return strings.Count(stripped, "%s")
}

func builtinPresetNamesList() []string {
	names := make([]string, 0, len(builtinPresetNames))
	for n := range builtinPresetNames {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func validSeverityValuesList() string {
	names := make([]string, 0, len(validSeverityValues))
	for n := range validSeverityValues {
		names = append(names, n)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}
