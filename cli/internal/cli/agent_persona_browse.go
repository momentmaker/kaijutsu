// agent_persona_browse.go — v0.14.0 read-only persona discovery.
// `jutsu agent persona browse` lists every resolved persona
// (built-in + user:home + user:project) with provenance tags.
//
// Output:
//   - TTY: aligned table with system_prompt truncated to 60 chars
//   - Pipe: JSON (auto-flip per existing agent-first convention)
//   - --yaml: paste-into-agents.yaml-ready yaml block
//   - --json: forces JSON regardless of pipe detection
//
// Filters: --tag, --provider, --source (each single-value; AND combine).
// Spec: docs/specs/2026-05-09-v0.14.0-persona-browse-and-preset-usage.md
package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/momentmaker/kaijutsu/cli/internal/agents"
)

// systemPromptDisplayMaxRunes caps the system_prompt column in TTY
// table output. JSON + yaml output preserve full text.
const systemPromptDisplayMaxRunes = 60

// newAgentPersonaCmd is the parent for `jutsu agent persona <verb>`.
// v0.14.0 ships browse only; test + new are deferred per spec.
func newAgentPersonaCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "persona",
		Short: "Inspect personas (browse). v0.14: read-only.",
	}
	cmd.AddCommand(newAgentPersonaBrowseCmd())
	return cmd
}

func newAgentPersonaBrowseCmd() *cobra.Command {
	var (
		tag        string
		provider   string
		source     string
		jsonOut    bool
		yamlOut    bool
	)
	cmd := &cobra.Command{
		Use:   "browse",
		Short: "List resolved personas (built-in + user:home + user:project) with filters and provenance tags",
		Long: `List every resolved persona with source tags.

Sources:
  built-in       shipped with kaijutsu (BuiltinPersonas())
  user:home      from ~/.kaijutsu/agents.yaml
  user:project   from .kaijutsu/agents.yaml in the current project

Multi-config collisions: project shadows home; home shadows built-in
(matches agents.Resolve overlay semantics). Browse surfaces the
winning version + tags it with the winning source.

Output auto-flips to JSON when stdout is a pipe; force with --json.
--yaml emits paste-into-agents.yaml-ready blocks. --json and --yaml
are mutually exclusive.

Filters (single-value, combine with AND):
  --tag <name>          exact match against persona.tags
  --provider <name>     exact match against persona.provider
  --source <source>     built-in | user:home | user:project`,
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := projectRoot()
			if err != nil {
				return err
			}
			global, err := agents.LoadGlobalConfig()
			if err != nil {
				return err
			}
			project, err := agents.LoadProjectConfig(root)
			if err != nil {
				return err
			}
			resolved, err := agents.Resolve(global, project)
			if err != nil {
				return err
			}

			rows := agents.BrowsePersonas(resolved, global, project, agents.BrowseFilters{
				Tag:      tag,
				Provider: provider,
				Source:   source,
			})

			out := cmd.OutOrStdout()
			switch {
			case yamlOut:
				return renderPersonasYaml(out, rows)
			case jsonOut || !isStdoutTTY(out):
				return renderPersonasJSON(out, rows)
			default:
				renderPersonasTable(out, rows)
				return nil
			}
		},
	}
	cmd.Flags().StringVar(&tag, "tag", "", "filter by persona tag (exact match)")
	cmd.Flags().StringVar(&provider, "provider", "", "filter by provider name (exact match)")
	cmd.Flags().StringVar(&source, "source", "", "filter by source: built-in | user:home | user:project")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "force JSON output regardless of pipe detection")
	cmd.Flags().BoolVar(&yamlOut, "yaml", false, "emit paste-into-agents.yaml-ready yaml block")
	cmd.MarkFlagsMutuallyExclusive("json", "yaml")
	return cmd
}

// isStdoutTTY returns true when out is the actual stdout AND it's a
// terminal. Used for auto-flip JSON-when-piped behavior.
func isStdoutTTY(out io.Writer) bool {
	f, ok := out.(*os.File)
	if !ok {
		return false
	}
	stat, err := f.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) != 0
}

func renderPersonasTable(out io.Writer, rows []agents.BrowseRow) {
	if len(rows) == 0 {
		fmt.Fprintln(out, "(no personas match the filter)")
		return
	}
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "PERSONA\tPROVIDER\tSOURCE\tTAGS\tSYSTEM_PROMPT")
	for _, r := range rows {
		sp := truncateRunes(r.SystemPrompt, systemPromptDisplayMaxRunes)
		if sp == "" {
			sp = "(empty)"
		}
		tags := "-"
		if len(r.Tags) > 0 {
			tags = fmt.Sprintf("%v", r.Tags)
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", r.Name, r.Provider, r.Source, tags, sp)
	}
	_ = tw.Flush()
	fmt.Fprintf(out, "\n%d persona(s)\n", len(rows))
}

func renderPersonasJSON(out io.Writer, rows []agents.BrowseRow) error {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(rows)
}

// renderPersonasYaml emits a paste-into-agents.yaml-ready block.
// Top-level `personas:` map keyed by name; sub-keys mirror
// agents.yaml schema (provider, system_prompt, tags, model).
//
// IMPORTANT: this output is a FRAGMENT (just the `personas:`
// subtree), not a standalone agents.yaml. Users paste this into
// an existing agents.yaml that already has `version:` + `providers:`
// blocks. Tests that round-trip through agents.LoadGlobalConfig
// must wrap the output with those required fields. Real-world
// usage flow: user runs `jutsu agent persona browse --yaml`,
// pastes the output into the `personas:` section of their existing
// `~/.kaijutsu/agents.yaml`.
func renderPersonasYaml(out io.Writer, rows []agents.BrowseRow) error {
	type personaYaml struct {
		Provider     string   `yaml:"provider"`
		SystemPrompt string   `yaml:"system_prompt,omitempty"`
		Model        string   `yaml:"model,omitempty"`
		Tags         []string `yaml:"tags,omitempty"`
	}
	wrapper := struct {
		Personas map[string]personaYaml `yaml:"personas"`
	}{
		Personas: make(map[string]personaYaml, len(rows)),
	}
	for _, r := range rows {
		wrapper.Personas[r.Name] = personaYaml{
			Provider:     r.Provider,
			SystemPrompt: r.SystemPrompt,
			Model:        r.Model,
			Tags:         r.Tags,
		}
	}
	// yaml.v3 sorts map[string]T keys alphabetically, so output is
	// deterministic across runs (verified by RenderPersonasYaml_Deterministic).
	enc := yaml.NewEncoder(out)
	enc.SetIndent(2)
	defer enc.Close()
	return enc.Encode(wrapper)
}
