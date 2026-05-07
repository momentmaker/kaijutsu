// describe.go — `jutsu --describe`: machine-readable command catalog
// for fresh AI agents discovering the CLI surface. Always JSON; this
// command exists FOR agents, not humans (humans pipe through jq if
// curious). Per AGENTS.md design principle.
package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// commandSpec is the JSON shape returned by --describe. Designed for
// agents to read once at session-start and learn the kaijutsu CLI
// surface without trial-and-error help-walking.
type commandSpec struct {
	Name        string         `json:"name"`           // canonical command name (e.g. "jutsu finding accept")
	Use         string         `json:"use"`            // usage line from cobra (e.g. "accept <id>")
	Short       string         `json:"short"`          // one-line summary
	Long        string         `json:"long,omitempty"` // multi-line description (omitted when same as short)
	Aliases     []string       `json:"aliases,omitempty"`
	Hidden      bool           `json:"hidden,omitempty"`
	Flags       []flagSpec     `json:"flags,omitempty"`
	Subcommands []*commandSpec `json:"subcommands,omitempty"`
}

type flagSpec struct {
	Name      string `json:"name"`            // long form (e.g. "format")
	Shorthand string `json:"shorthand,omitempty"`
	Type      string `json:"type"`            // "string" / "bool" / "int" / "duration" / etc.
	Default   string `json:"default,omitempty"`
	Usage     string `json:"usage"`
}

// catalog is the top-level --describe payload. Includes a schema
// version field so future agents can detect format changes without
// breaking on unknown fields.
type catalog struct {
	SchemaVersion int            `json:"schema_version"`
	Tool          string         `json:"tool"`         // "jutsu"
	Version       string         `json:"version"`      // build version
	Description   string         `json:"description"`  // root command Short
	Commands      []*commandSpec `json:"commands"`
}

// newDescribeCmd registers `jutsu --describe`. Hidden from --help
// because it's meant for agents, not humans browsing the CLI surface.
//
// Output is ALWAYS JSON regardless of TTY — this command exists for
// machine consumption. Humans pipe through jq if they want to read
// it.
func newDescribeCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "describe",
		Short:  "Print a machine-readable JSON catalog of all jutsu commands and flags",
		Long: `Returns a JSON document describing the entire jutsu CLI surface — every command, every flag, every description. Designed for fresh AI agents to ingest at session start so they can use the tool without trial-and-error help-walking.

Output is always JSON regardless of TTY. Pipe through jq for human reading.

Schema version 1: {schema_version, tool, version, description, commands[{name, use, short, long?, aliases?, hidden?, flags[{name, shorthand?, type, default?, usage}], subcommands[]}]}.`,
		Hidden: true, // not in human --help; agents discover via convention
		RunE: func(cmd *cobra.Command, args []string) error {
			root := cmd.Root()
			cat := catalog{
				SchemaVersion: 1,
				Tool:          root.Use,
				Version:       Version,
				Description:   root.Short,
				Commands:      describeChildren(root),
			}
			out, err := json.MarshalIndent(cat, "", "  ")
			if err != nil {
				return fmt.Errorf("marshal catalog: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(out))
			return nil
		},
	}
}

// describeChildren walks a cobra command's children and produces the
// JSON spec recursively. Sorts subcommands alphabetically for stable
// output (deterministic — agents diffing two --describe outputs see
// only real changes).
func describeChildren(parent *cobra.Command) []*commandSpec {
	children := parent.Commands()
	specs := make([]*commandSpec, 0, len(children))
	for _, c := range children {
		// Skip cobra's auto-generated `help` and `completion` —
		// agents don't need to know about them, and they bloat the
		// catalog.
		if c.Name() == "help" || c.Name() == "completion" {
			continue
		}
		specs = append(specs, describeCommand(c))
	}
	sort.Slice(specs, func(i, j int) bool { return specs[i].Name < specs[j].Name })
	return specs
}

// describeCommand fills a commandSpec from a cobra.Command. Builds
// the canonical Name from CommandPath() (e.g. "jutsu finding accept"),
// strips the "use" line to just the local invocation (e.g. "accept <id>"),
// and recurses into subcommands.
func describeCommand(c *cobra.Command) *commandSpec {
	spec := &commandSpec{
		Name:        c.CommandPath(),
		Use:         c.Use,
		Short:       c.Short,
		Aliases:     c.Aliases,
		Hidden:      c.Hidden,
		Flags:       describeFlags(c),
		Subcommands: describeChildren(c),
	}
	// Only include Long when it adds info beyond Short (avoid
	// duplicate noise in the JSON).
	if c.Long != "" && c.Long != c.Short {
		spec.Long = strings.TrimSpace(c.Long)
	}
	return spec
}

// describeFlags walks the local flagset (NOT inherited; each command
// surfaces its OWN flags + global flags appear at root). Skips
// cobra-internal "help" flag.
func describeFlags(c *cobra.Command) []flagSpec {
	out := []flagSpec{}
	c.LocalFlags().VisitAll(func(f *pflag.Flag) {
		if f.Name == "help" {
			return
		}
		out = append(out, flagSpec{
			Name:      f.Name,
			Shorthand: f.Shorthand,
			Type:      f.Value.Type(),
			Default:   f.DefValue,
			Usage:     f.Usage,
		})
	})
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
