package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

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
	cmd.AddCommand(newAgentRemoveCmd())
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
		Short: "Audit agents.yaml resolved config for env-var presence + driver-kind health",
		Long: `Per-driver-kind health probe across every enabled provider:
- cli:        '<cmd> --version' returns 0 within 2s
- cli-compat: same as cli for the BaseCLI binary, plus api_key_env check
- http:       api_key_env set in env AND base_url GET /models is reachable
- mcp:        deferred to Stage 6 (probe will be a list-tools handshake)

Exits 0 only if every enabled provider passes its driver's checks.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()

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
				rows := doctorProbe(ctx, p)
				for _, row := range rows {
					if !row.pass {
						fail = true
					}
					status := "✓"
					if !row.pass {
						status = "✗"
					}
					fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", p.Name, p.Driver, row.check, status, row.detail)
				}
			}
			_ = tw.Flush()
			if fail {
				return fmt.Errorf("one or more enabled providers failed driver-kind health checks (see table)")
			}
			return nil
		},
	}
	return cmd
}

type doctorRow struct {
	check  string
	pass   bool
	detail string
}

// doctorProbe runs the per-driver-kind health checks for one provider.
// All probes are bounded by ctx (default 30s aggregate set in the
// doctor command). Returns one or more rows for the table.
func doctorProbe(ctx context.Context, p *agents.Provider) []doctorRow {
	switch p.Driver {
	case agents.DriverCLI:
		return []doctorRow{probeCLIVersion(ctx, p.Cmd, "binary")}
	case agents.DriverCLICompat:
		bin := p.BaseCLI
		if bin == "" {
			bin = p.Name
		}
		return []doctorRow{
			probeCLIVersion(ctx, bin, "base-cli"),
			probeAPIKeyEnv(p),
		}
	case agents.DriverHTTP:
		return []doctorRow{
			probeAPIKeyEnv(p),
			probeHTTPModels(ctx, p),
		}
	case agents.DriverMCP:
		return []doctorRow{{check: "mcp", pass: false, detail: "mcp probe not implemented (Stage 6)"}}
	}
	return []doctorRow{{check: "driver", pass: false, detail: fmt.Sprintf("unknown driver kind %q", p.Driver)}}
}

func probeCLIVersion(ctx context.Context, bin, label string) doctorRow {
	if bin == "" {
		return doctorRow{check: label, pass: false, detail: "cmd not set"}
	}
	subCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(subCtx, bin, "--version").CombinedOutput()
	if err != nil {
		return doctorRow{check: label, pass: false, detail: fmt.Sprintf("%s --version failed: %v", bin, err)}
	}
	first := strings.SplitN(strings.TrimSpace(string(out)), "\n", 2)[0]
	return doctorRow{check: label, pass: true, detail: truncateRunes(first, 60)}
}

func probeAPIKeyEnv(p *agents.Provider) doctorRow {
	if p.APIKeyEnv == "" {
		return doctorRow{check: "env", pass: true, detail: "(no api_key_env declared)"}
	}
	if os.Getenv(p.APIKeyEnv) == "" {
		return doctorRow{check: "env", pass: false, detail: fmt.Sprintf("%s not set", p.APIKeyEnv)}
	}
	return doctorRow{check: "env", pass: true, detail: fmt.Sprintf("%s set", p.APIKeyEnv)}
}

func probeHTTPModels(ctx context.Context, p *agents.Provider) doctorRow {
	if p.BaseURL == "" {
		return doctorRow{check: "reach", pass: false, detail: "base_url not set"}
	}
	subCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	url := strings.TrimRight(p.BaseURL, "/") + "/models"
	req, err := http.NewRequestWithContext(subCtx, "GET", url, nil)
	if err != nil {
		return doctorRow{check: "reach", pass: false, detail: err.Error()}
	}
	if k := os.Getenv(p.APIKeyEnv); k != "" {
		switch p.Protocol {
		case "anthropic-compat":
			req.Header.Set("x-api-key", k)
			req.Header.Set("anthropic-version", "2023-06-01")
		default:
			req.Header.Set("authorization", "Bearer "+k)
		}
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return doctorRow{check: "reach", pass: false, detail: fmt.Sprintf("GET %s: %v", url, err)}
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return doctorRow{check: "reach", pass: true, detail: fmt.Sprintf("GET /models %d", resp.StatusCode)}
	}
	return doctorRow{check: "reach", pass: false, detail: fmt.Sprintf("GET /models %d (provider may not support /models — agent test will fall back to a 1-token completion)", resp.StatusCode)}
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
