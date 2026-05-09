package cli

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/momentmaker/kaijutsu/cli/internal/swarm"
)

func writeUserPresetYaml(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, ".kaijutsu"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".kaijutsu", "swarm.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

const validUserPresetYaml = `
- name: u-tight-review
  description: shorter pr-review
  inputKind: diff
  defaultPrompt: "review %s"
  synthesizer: "synthesize %s"
  severityVocab: [blocker, issue, minor, info]
  mode: quick
`

const validFilesPresetYaml = `
- name: u-doc-audit
  description: shorter doc-review
  inputKind: files
  defaultPrompt: "audit %s"
  synthesizer: "synthesize %s"
  severityVocab: [issue, minor]
`

const validPromptPresetYaml = `
- name: u-quick-brainstorm
  description: shorter brainstorm
  inputKind: prompt
  defaultPrompt: "ideate %s"
  synthesizer: "synthesize %s"
  severityVocab: [recommended, alternative]
`

// freshParent returns a cobra.Command we can attach user preset
// subcommands to without polluting the production swarm cmd.
func freshParent() *cobra.Command {
	return &cobra.Command{Use: "swarm"}
}

func TestAddUserPresetSubcommands_RegistersFromValidYaml(t *testing.T) {
	tmp := t.TempDir()
	homeDir := filepath.Join(tmp, "home")
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	projectRoot := filepath.Join(tmp, "project")
	writeUserPresetYaml(t, projectRoot, validUserPresetYaml)

	parent := freshParent()
	registry := swarm.NewPresetRegistry()
	var stderr bytes.Buffer
	addUserPresetSubcommands(parent, registry, projectRoot, homeDir, &stderr)

	found := false
	for _, sub := range parent.Commands() {
		if sub.Name() == "u-tight-review" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("u-tight-review subcommand not registered; stderr=%s", stderr.String())
	}
}

func TestAddUserPresetSubcommands_TagsSourceInShort(t *testing.T) {
	tmp := t.TempDir()
	homeDir := filepath.Join(tmp, "home")
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	projectRoot := filepath.Join(tmp, "project")
	writeUserPresetYaml(t, projectRoot, validUserPresetYaml)

	parent := freshParent()
	registry := swarm.NewPresetRegistry()
	addUserPresetSubcommands(parent, registry, projectRoot, homeDir, io.Discard)

	for _, sub := range parent.Commands() {
		if sub.Name() == "u-tight-review" {
			if !strings.HasSuffix(sub.Short, "[user:project]") {
				t.Errorf("Short = %q, want suffix '[user:project]'", sub.Short)
			}
			return
		}
	}
	t.Fatal("u-tight-review not found")
}

func TestAddUserPresetSubcommands_HomeSourceTag(t *testing.T) {
	tmp := t.TempDir()
	homeDir := filepath.Join(tmp, "home")
	writeUserPresetYaml(t, homeDir, validUserPresetYaml)
	projectRoot := filepath.Join(tmp, "project")
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	parent := freshParent()
	registry := swarm.NewPresetRegistry()
	addUserPresetSubcommands(parent, registry, projectRoot, homeDir, io.Discard)

	for _, sub := range parent.Commands() {
		if sub.Name() == "u-tight-review" {
			if !strings.HasSuffix(sub.Short, "[user:home]") {
				t.Errorf("Short = %q, want suffix '[user:home]'", sub.Short)
			}
			return
		}
	}
	t.Fatal("u-tight-review not found")
}

func TestAddUserPresetSubcommands_BrokenYamlDoesNotBreakRoot(t *testing.T) {
	tmp := t.TempDir()
	homeDir := filepath.Join(tmp, "home")
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	projectRoot := filepath.Join(tmp, "project")
	writeUserPresetYaml(t, projectRoot, "this is: very broken yaml: lol")

	parent := freshParent()
	registry := swarm.NewPresetRegistry()
	var stderr bytes.Buffer
	addUserPresetSubcommands(parent, registry, projectRoot, homeDir, &stderr)

	if len(parent.Commands()) != 0 {
		t.Errorf("broken yaml should add zero subcommands; got %d", len(parent.Commands()))
	}
	if !strings.Contains(stderr.String(), "warning") {
		t.Errorf("expected warning on stderr; got %q", stderr.String())
	}
}

func TestAddUserPresetSubcommands_BuiltinShadowSurfacesAsCollision(t *testing.T) {
	tmp := t.TempDir()
	homeDir := filepath.Join(tmp, "home")
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	projectRoot := filepath.Join(tmp, "project")
	// Built-in shadow is rejected at LoadUserPresets validation
	// (struct-level), so the warning surfaces from the load path.
	shadowYaml := `
- name: pr-review
  description: shadows builtin
  inputKind: diff
  defaultPrompt: "%s"
  synthesizer: "%s"
  severityVocab: [issue]
`
	writeUserPresetYaml(t, projectRoot, shadowYaml)

	parent := freshParent()
	registry := swarm.NewPresetRegistry()
	registry.Register(&swarm.Preset{Name: "pr-review"})
	var stderr bytes.Buffer
	addUserPresetSubcommands(parent, registry, projectRoot, homeDir, &stderr)

	for _, sub := range parent.Commands() {
		if sub.Name() == "pr-review" {
			t.Error("user preset shadowing built-in should NOT be registered as subcommand")
		}
	}
	if !strings.Contains(stderr.String(), "pr-review") {
		t.Errorf("stderr should cite the conflicting name; got %q", stderr.String())
	}
}

func TestNewUserPresetSubcommand_FlagSetForInputDiff(t *testing.T) {
	up := &swarm.UserPreset{
		Preset: swarm.Preset{
			Name:      "test-diff",
			InputKind: swarm.InputDiff,
		},
		Source: swarm.SourceProject,
	}
	cmd := newUserPresetSubcommand("test-diff", up)
	if cmd.Flags().Lookup("pr") == nil {
		t.Error("InputDiff preset should expose --pr flag")
	}
	if cmd.Flags().Lookup("diff-from-branch") == nil {
		t.Error("InputDiff preset should expose --diff-from-branch flag")
	}
}

func TestNewUserPresetSubcommand_FlagSetForInputFiles(t *testing.T) {
	up := &swarm.UserPreset{
		Preset: swarm.Preset{
			Name:      "test-files",
			InputKind: swarm.InputFiles,
		},
		Source: swarm.SourceProject,
	}
	cmd := newUserPresetSubcommand("test-files", up)
	if cmd.Flags().Lookup("pr") != nil {
		t.Error("InputFiles preset should NOT expose --pr flag")
	}
	if cmd.Args == nil {
		t.Error("InputFiles preset should accept positional args")
	}
}

func TestNewUserPresetSubcommand_FlagSetForInputPrompt(t *testing.T) {
	up := &swarm.UserPreset{
		Preset: swarm.Preset{
			Name:      "test-prompt",
			InputKind: swarm.InputPrompt,
		},
		Source: swarm.SourceProject,
	}
	cmd := newUserPresetSubcommand("test-prompt", up)
	if cmd.Args == nil {
		t.Error("InputPrompt preset should accept positional args")
	}
}

func TestUserPresetEndToEnd_RegistryLookupSucceedsAtRunE(t *testing.T) {
	// Pin: registration must complete before any subcommand's RunE
	// fires. Plan-doc-review caught the original ordering rationale
	// as misleading — this test pins the end-to-end contract.
	tmp := t.TempDir()
	homeDir := filepath.Join(tmp, "home")
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	projectRoot := filepath.Join(tmp, "project")
	writeUserPresetYaml(t, projectRoot, validUserPresetYaml)

	parent := freshParent()
	registry := swarm.NewPresetRegistry()
	addUserPresetSubcommands(parent, registry, projectRoot, homeDir, io.Discard)

	// After registration, looking up the preset by name in the registry
	// must succeed — that's what the RunE will do at command time via
	// LoadPresetWithSkillOverrides → DefaultRegistry → Find.
	p, err := registry.Find("u-tight-review")
	if err != nil {
		t.Fatalf("registry.Find(u-tight-review) failed after registration: %v", err)
	}
	if p.Name != "u-tight-review" {
		t.Errorf("Name = %q, want u-tight-review", p.Name)
	}
	if registry.UserSource("u-tight-review") != swarm.SourceProject {
		t.Errorf("UserSource = %q, want SourceProject", registry.UserSource("u-tight-review"))
	}
}

func TestSwarmHelp_DifferentiatesUserFromBuiltin(t *testing.T) {
	tmp := t.TempDir()
	homeDir := filepath.Join(tmp, "home")
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	projectRoot := filepath.Join(tmp, "project")
	writeUserPresetYaml(t, projectRoot, validUserPresetYaml)

	parent := freshParent()
	parent.AddCommand(&cobra.Command{Use: "pr-review", Short: "Multi-agent PR review"})
	registry := swarm.NewPresetRegistry()
	addUserPresetSubcommands(parent, registry, projectRoot, homeDir, io.Discard)

	var help bytes.Buffer
	parent.SetOut(&help)
	parent.SetErr(&help)
	parent.SetArgs([]string{"--help"})
	if err := parent.Execute(); err != nil {
		t.Fatalf("help: %v", err)
	}

	out := help.String()
	if !strings.Contains(out, "pr-review") {
		t.Error("help missing built-in pr-review")
	}
	if !strings.Contains(out, "u-tight-review") {
		t.Error("help missing user preset u-tight-review")
	}
	if !strings.Contains(out, "[user:project]") {
		t.Error("help missing [user:project] tag")
	}
}

// TestNewUserPresetSubcommand_InvalidInputKindStubErrors covers the
// defense-in-depth path: validateUserPresetEntry rejects invalid
// InputKind at load time, but if an unknown kind ever leaks through
// (e.g. future enum extension), the cobra subcommand returns a
// helpful error rather than panicking.
func TestNewUserPresetSubcommand_InvalidInputKindStubErrors(t *testing.T) {
	up := &swarm.UserPreset{
		Preset: swarm.Preset{
			Name:      "test-bad-kind",
			InputKind: swarm.InputKind(999), // synthetic invalid value
		},
		Source: swarm.SourceProject,
	}
	cmd := newUserPresetSubcommand("test-bad-kind", up)
	if !strings.Contains(cmd.Short, "DISABLED") {
		t.Errorf("Short should flag DISABLED; got %q", cmd.Short)
	}
	cmd.SetArgs([]string{})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("invalid InputKind subcommand should error on invocation")
	}
	if !strings.Contains(err.Error(), "invalid InputKind") {
		t.Errorf("error should mention invalid InputKind; got %v", err)
	}
}

// Compile-time guard: ensure errors.New is in the test imports.
var _ = errors.New
