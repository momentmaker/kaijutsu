package cli

import (
	"fmt"
	"io"
	"os"
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/momentmaker/kaijutsu/cli/internal/agents"
)

// newAgentCmd builds the `jutsu agent` subcommand group. v0.6 Stage 2
// ships `list` (read-only) and `doctor` (env-var audit). Stage 5
// rounds out add/enable/disable/remove/test/migrate.
func newAgentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Inspect and configure swarm provider/persona setup",
		Long: `Manage the .kaijutsu/agents.yaml provider catalog and persona declarations.

Subcommands cover the full v0.6 surface: list, doctor, add, enable,
disable, test, migrate. (remove + cross-repo scan land in Stage 5b.)`,
	}
	cmd.AddCommand(newAgentListCmd())
	cmd.AddCommand(newAgentDoctorCmd())
	cmd.AddCommand(newAgentAddCmd())
	cmd.AddCommand(newAgentEnableCmd())
	cmd.AddCommand(newAgentDisableCmd())
	cmd.AddCommand(newAgentTestCmd())
	cmd.AddCommand(newAgentMigrateCmd())
	return cmd
}

func newAgentListCmd() *cobra.Command {
	var showPersonas bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Show resolved providers (and optionally personas)",
		Long: `Print the resolved provider catalog (project ⊕ global ⊕ built-ins).
Pass --personas to also list resolved personas.

Secrets are never printed; api_key_env shows the env-var name only.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			projectRoot, err := os.Getwd()
			if err != nil {
				return err
			}
			global, err := agents.LoadGlobalConfig()
			if err != nil {
				return err
			}
			project, err := agents.LoadProjectConfig(projectRoot)
			if err != nil {
				return err
			}
			resolved, err := agents.Resolve(global, project)
			if err != nil {
				return err
			}
			printProviders(cmd.OutOrStdout(), resolved)
			if showPersonas {
				fmt.Fprintln(cmd.OutOrStdout())
				printPersonas(cmd.OutOrStdout(), resolved)
			}
			if len(resolved.MissingKeys) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "\nwarn: %d enabled provider(s) missing API key env var: %v\n", len(resolved.MissingKeys), resolved.MissingKeys)
				fmt.Fprintln(cmd.ErrOrStderr(), "      run `jutsu agent doctor` for details, or `export <ENV_NAME>=...` to fix")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&showPersonas, "personas", false, "also list resolved personas")
	return cmd
}

func newAgentDoctorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Audit agents.yaml resolved config for missing env vars",
		Long: `Walks every enabled provider and reports per-provider health:
- env-var presence (api_key_env / env_key indirection)

v0.6 Stage 2 ships env-var auditing only. Stage 5 adds binary
existence (cli driver), endpoint reachability (http driver), and
MCP server handshake (mcp driver).

Exits 0 only if every enabled provider's required env vars are set.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			projectRoot, err := os.Getwd()
			if err != nil {
				return err
			}
			global, err := agents.LoadGlobalConfig()
			if err != nil {
				return err
			}
			project, err := agents.LoadProjectConfig(projectRoot)
			if err != nil {
				return err
			}
			resolved, err := agents.Resolve(global, project)
			if err != nil {
				return err
			}
			fail := false
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "PROVIDER\tDRIVER\tCHECK\tSTATUS\tDETAIL")
			names := make([]string, 0, len(resolved.Providers))
			for n := range resolved.Providers {
				names = append(names, n)
			}
			sort.Strings(names)
			for _, name := range names {
				p := resolved.Providers[name]
				if p.APIKeyEnv == "" {
					fmt.Fprintf(tw, "%s\t%s\tenv\t✓\t(no api_key_env declared)\n", p.Name, p.Driver)
					continue
				}
				if os.Getenv(p.APIKeyEnv) == "" {
					fmt.Fprintf(tw, "%s\t%s\tenv\t✗\t%s not set\n", p.Name, p.Driver, p.APIKeyEnv)
					fail = true
					continue
				}
				fmt.Fprintf(tw, "%s\t%s\tenv\t✓\t%s set\n", p.Name, p.Driver, p.APIKeyEnv)
			}
			_ = tw.Flush()
			if fail {
				return fmt.Errorf("one or more enabled providers have missing env vars (see table)")
			}
			return nil
		},
	}
	return cmd
}

func printProviders(w io.Writer, r *agents.Resolved) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "PROVIDER\tDRIVER\tDETAIL")
	names := make([]string, 0, len(r.Providers))
	for n := range r.Providers {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, name := range names {
		p := r.Providers[name]
		detail := providerDetail(p)
		fmt.Fprintf(tw, "%s\t%s\t%s\n", p.Name, p.Driver, detail)
	}
	_ = tw.Flush()
	fmt.Fprintf(w, "\n%d enabled provider(s): %v\n", len(r.Enabled), r.Enabled)
}

func providerDetail(p *agents.Provider) string {
	switch p.Driver {
	case agents.DriverCLI:
		if len(p.Args) > 0 {
			return fmt.Sprintf("cmd=%s args=%v", p.Cmd, p.Args)
		}
		return fmt.Sprintf("cmd=%s", p.Cmd)
	case agents.DriverHTTP:
		return fmt.Sprintf("%s base_url=%s model=%s key=$%s", p.Protocol, p.BaseURL, p.Model, p.APIKeyEnv)
	case agents.DriverCLICompat:
		return fmt.Sprintf("base_cli=%s envs=%d", p.BaseCLI, len(p.Env)+len(p.EnvKey))
	case agents.DriverMCP:
		return fmt.Sprintf("transport=%s tool=%s", p.Transport, p.ToolName)
	}
	return fmt.Sprintf("(unknown driver kind: %q)", p.Driver)
}

// truncateRunes returns s if it has at most max runes; otherwise the
// first (max-3) runes followed by "...". Rune-aware so non-ASCII
// system_prompts (Japanese, emoji, etc.) don't get sliced mid-codepoint.
func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max < 4 {
		return string(r[:max])
	}
	return string(r[:max-3]) + "..."
}

func printPersonas(w io.Writer, r *agents.Resolved) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "PERSONA\tPROVIDER\tTAGS\tSYSTEM_PROMPT")
	names := make([]string, 0, len(r.Personas))
	for n := range r.Personas {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, name := range names {
		p := r.Personas[name]
		sp := truncateRunes(p.SystemPrompt, 60)
		if sp == "" {
			sp = "(empty)"
		}
		fmt.Fprintf(tw, "%s\t%s\t%v\t%s\n", p.Name, p.Provider, p.Tags, sp)
	}
	_ = tw.Flush()
	fmt.Fprintf(w, "\n%d persona(s)\n", len(r.Personas))
}
