package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/momentmaker/kaijutsu/cli/internal/agents"
)

func TestAgentPersonaBrowse_HelpListsFlags(t *testing.T) {
	cmd := newAgentPersonaBrowseCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"--tag", "--provider", "--source", "--json", "--yaml"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("help missing %q", want)
		}
	}
}

func TestAgentPersonaBrowse_PipeRendersJSON(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp) // isolate from user's real ~/.kaijutsu/agents.yaml
	prev, _ := os.Getwd()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(prev)

	cmd := newAgentPersonaBrowseCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	// Force JSON via flag (out is *bytes.Buffer, isStdoutTTY returns false → JSON anyway).
	cmd.SetArgs([]string{"--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var rows []agents.BrowseRow
	if err := json.Unmarshal(stdout.Bytes(), &rows); err != nil {
		t.Fatalf("output is not valid JSON: %v\nbody: %s", err, stdout.String())
	}
	if len(rows) == 0 {
		t.Error("expected at least built-in personas; got 0")
	}
}

func TestAgentPersonaBrowse_JSONHasFullSystemPrompt(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	prev, _ := os.Getwd()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(prev)

	cmd := newAgentPersonaBrowseCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"--json", "--source", "built-in"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var rows []agents.BrowseRow
	if err := json.Unmarshal(stdout.Bytes(), &rows); err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.Name == "paranoid-security-claude" && len(row.SystemPrompt) < 100 {
			t.Errorf("system_prompt got truncated in JSON output (len=%d); want full text", len(row.SystemPrompt))
		}
	}
}

func TestAgentPersonaBrowse_YamlFlagRendersPasteReadyFormat(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	prev, _ := os.Getwd()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(prev)

	cmd := newAgentPersonaBrowseCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"--yaml", "--source", "built-in"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(strings.TrimSpace(stdout.String()), "personas:") {
		t.Errorf("yaml should start with 'personas:'; got %q", stdout.String()[:60])
	}
}

func TestAgentPersonaBrowse_YamlRoundTripsThroughLoadGlobalConfig(t *testing.T) {
	// Run browse --yaml; write output to a synthetic ~/.kaijutsu/agents.yaml;
	// load via agents.LoadGlobalConfig and verify built-in personas
	// round-trip with name + provider + system_prompt + tags.
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	prev, _ := os.Getwd()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(prev)

	cmd := newAgentPersonaBrowseCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"--yaml", "--source", "built-in"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	// agents.yaml requires `version: 1` + `providers:` block; the
	// browse output is just the personas: subtree. Wrap it.
	yamlBody := "version: 1\nproviders: {}\n" + stdout.String()
	cfgPath := filepath.Join(tmp, ".kaijutsu", "agents.yaml")
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, []byte(yamlBody), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := agents.LoadGlobalConfig()
	if err != nil {
		t.Fatalf("LoadGlobalConfig: %v\nbody:\n%s", err, yamlBody)
	}
	// Pick a known built-in to spot-check the round-trip.
	p, ok := cfg.Personas["paranoid-security-claude"]
	if !ok {
		t.Fatal("paranoid-security-claude not round-tripped")
	}
	if p.Provider != "claude" {
		t.Errorf("provider = %q; want claude", p.Provider)
	}
	if !strings.Contains(p.SystemPrompt, "paranoid security reviewer") {
		t.Errorf("system_prompt round-trip incomplete; got %q", p.SystemPrompt)
	}
}

func TestAgentPersonaBrowse_JSONAndYamlMutuallyExclusive(t *testing.T) {
	cmd := newAgentPersonaBrowseCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"--json", "--yaml"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for --json + --yaml together")
	}
	combined := err.Error() + stdout.String()
	if !strings.Contains(combined, "json") || !strings.Contains(combined, "yaml") {
		t.Errorf("error should name both flags; got: %v / %s", err, stdout.String())
	}
	// Cobra's MarkFlagsMutuallyExclusive wording is "if any flags in the group [json yaml] are set"
	if !strings.Contains(combined, "[json yaml]") && !strings.Contains(combined, "mutually exclusive") {
		t.Errorf("error should indicate mutual-exclusion; got: %v / %s", err, stdout.String())
	}
}

func TestAgentPersonaBrowse_TTYTruncatesSystemPrompt(t *testing.T) {
	// Direct render-table call to bypass cobra's --json auto-flip.
	rows := []agents.BrowseRow{
		{
			Name:         "long-prompt-test",
			Provider:     "claude",
			Source:       "built-in",
			SystemPrompt: strings.Repeat("a", 200),
		},
	}
	var buf bytes.Buffer
	renderPersonasTable(&buf, rows)
	out := buf.String()
	// Expect ellipsis (truncated to 60 runes including the "..." suffix
	// per truncateRunes implementation).
	if !strings.Contains(out, "...") {
		t.Errorf("expected truncation ellipsis in TTY output; got %q", out)
	}
	if strings.Contains(out, strings.Repeat("a", 100)) {
		t.Error("TTY output should NOT contain the full long system_prompt (should be truncated)")
	}
}

func TestRenderPersonasYaml_Deterministic(t *testing.T) {
	// Stable iteration order means yaml output is byte-equal across runs.
	rows := []agents.BrowseRow{
		{Name: "z-last", Provider: "claude"},
		{Name: "a-first", Provider: "antigravity"},
		{Name: "m-mid", Provider: "codex"},
	}
	var buf1, buf2 bytes.Buffer
	if err := renderPersonasYaml(&buf1, rows); err != nil {
		t.Fatal(err)
	}
	if err := renderPersonasYaml(&buf2, rows); err != nil {
		t.Fatal(err)
	}
	if buf1.String() != buf2.String() {
		t.Errorf("yaml output not deterministic across runs:\n%s\n---\n%s", buf1.String(), buf2.String())
	}
	// Verify it parses
	var wrapper struct {
		Personas map[string]map[string]any `yaml:"personas"`
	}
	if err := yaml.Unmarshal(buf1.Bytes(), &wrapper); err != nil {
		t.Fatal(err)
	}
	if len(wrapper.Personas) != 3 {
		t.Errorf("expected 3 personas; got %d", len(wrapper.Personas))
	}
}
