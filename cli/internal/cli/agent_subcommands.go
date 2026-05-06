package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/momentmaker/kaijutsu/cli/internal/agents"
	"github.com/momentmaker/kaijutsu/cli/internal/manifest"
)

// --- agent add --------------------------------------------------------

func newAgentAddCmd() *cobra.Command {
	var (
		global    bool
		force     bool
		driver    string
		cmd2      string
		argsArg   []string
		protocol  string
		baseURL   string
		model     string
		apiKeyEnv string
		baseCLI   string
		envPairs  []string
		envKeyP   []string
	)
	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add a provider to the local agents.yaml (project default; --global for ~/)",
		Long: `Add a provider entry to .kaijutsu/agents.yaml.

Two paths:
- catalog: 'jutsu agent add deepseek' — uses the vendored default
  config for the named provider (currently claude/codex/gemini).
- non-catalog: 'jutsu agent add <name> --driver http --protocol ...'
  — fully user-supplied. All driver-specific flags are optional but
  the loader will reject the resulting entry if required fields for
  the chosen driver are missing.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			projectRoot, _ := os.Getwd()

			provider, err := buildProviderFromFlags(name, driver, cmd2, argsArg, protocol, baseURL, model, apiKeyEnv, baseCLI, envPairs, envKeyP)
			if err != nil {
				return err
			}

			if global {
				return addToGlobal(provider, force)
			}
			return addToProject(projectRoot, provider, force)
		},
	}
	cmd.Flags().BoolVar(&global, "global", false, "write to ~/.kaijutsu/agents.yaml instead of <repo>/.kaijutsu/agents.yaml")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite an existing entry without prompting")
	cmd.Flags().StringVar(&driver, "driver", "", "driver kind (cli | http | cli-compat | mcp). Required when name is not in the catalog.")
	cmd.Flags().StringVar(&cmd2, "cmd", "", "cli driver: command name (defaults to provider name)")
	cmd.Flags().StringSliceVar(&argsArg, "args", nil, "cli driver: extra args (repeatable, comma-separated)")
	cmd.Flags().StringVar(&protocol, "protocol", "", "http driver: openai-compat | anthropic-compat")
	cmd.Flags().StringVar(&baseURL, "base-url", "", "http driver: API base URL")
	cmd.Flags().StringVar(&model, "model", "", "http driver: model name")
	cmd.Flags().StringVar(&apiKeyEnv, "api-key-env", "", "http / cli-compat driver: env-var name holding the API key")
	cmd.Flags().StringVar(&baseCLI, "base-cli", "", "cli-compat driver: underlying CLI to wrap (claude | codex | gemini)")
	cmd.Flags().StringSliceVar(&envPairs, "env", nil, "cli-compat driver: literal env override KEY=VALUE (repeatable)")
	cmd.Flags().StringSliceVar(&envKeyP, "env-key", nil, "cli-compat driver: indirect env VAR=ENV_NAME — VAR is set on child to value of ENV_NAME at invoke (repeatable)")
	return cmd
}

func buildProviderFromFlags(name, driver, cmd2 string, argsArg []string, protocol, baseURL, model, apiKeyEnv, baseCLI string, envPairs, envKeyP []string) (*agents.Provider, error) {
	// Catalog path: name in built-ins + no driver flag → use defaults.
	if driver == "" {
		if p, ok := agents.BuiltinProviders()[name]; ok {
			cp := *p
			cp.Name = name
			return &cp, nil
		}
		return nil, fmt.Errorf("agent add %q: not in vendored catalog and no --driver flag set; pass --driver cli|http|cli-compat|mcp", name)
	}
	p := &agents.Provider{Name: name, Driver: agents.DriverKind(driver)}
	switch p.Driver {
	case agents.DriverCLI:
		p.Cmd = cmd2
		if p.Cmd == "" {
			p.Cmd = name
		}
		p.Args = argsArg
	case agents.DriverHTTP:
		if protocol == "" || baseURL == "" || model == "" {
			return nil, errors.New("http driver requires --protocol, --base-url, --model")
		}
		p.Protocol = protocol
		p.BaseURL = baseURL
		p.Model = model
		p.APIKeyEnv = apiKeyEnv
	case agents.DriverCLICompat:
		if baseCLI == "" {
			return nil, errors.New("cli-compat driver requires --base-cli")
		}
		p.BaseCLI = baseCLI
		p.Env = parseKeyValPairs(envPairs)
		p.EnvKey = parseKeyValPairs(envKeyP)
		p.APIKeyEnv = apiKeyEnv
	case agents.DriverMCP:
		return nil, errors.New("mcp driver lands in Stage 6")
	default:
		return nil, fmt.Errorf("unknown driver kind %q", driver)
	}
	return p, nil
}

func parseKeyValPairs(pairs []string) map[string]string {
	if len(pairs) == 0 {
		return nil
	}
	out := make(map[string]string, len(pairs))
	for _, p := range pairs {
		eq := strings.IndexByte(p, '=')
		if eq <= 0 {
			continue
		}
		out[p[:eq]] = p[eq+1:]
	}
	return out
}

func addToGlobal(p *agents.Provider, force bool) error {
	c, err := agents.LoadGlobalConfig()
	if err != nil {
		return err
	}
	if c.Providers == nil {
		c.Providers = map[string]*agents.Provider{}
	}
	if _, exists := c.Providers[p.Name]; exists && !force {
		return fmt.Errorf("provider %q already exists in global agents.yaml; pass --force to overwrite", p.Name)
	}
	c.Providers[p.Name] = p
	if err := agents.SaveGlobalConfig(c); err != nil {
		return err
	}
	fmt.Printf("added %q (driver=%s) to ~/.kaijutsu/agents.yaml\n", p.Name, p.Driver)
	return nil
}

func addToProject(root string, p *agents.Provider, force bool) error {
	c, err := agents.LoadProjectConfig(root)
	if err != nil {
		return err
	}
	if c.Providers == nil {
		c.Providers = map[string]*agents.Provider{}
	}
	if _, exists := c.Providers[p.Name]; exists && !force {
		return fmt.Errorf("provider %q already exists in project agents.yaml; pass --force to overwrite", p.Name)
	}
	c.Providers[p.Name] = p
	if err := agents.SaveProjectConfig(root, c); err != nil {
		return err
	}
	fmt.Printf("added %q (driver=%s) to <repo>/.kaijutsu/agents.yaml\n", p.Name, p.Driver)
	return nil
}

// --- agent enable / disable ------------------------------------------

func newAgentEnableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "enable <name>",
		Short: "Add a provider to the project's enabled list",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return toggleEnabled(args[0], true)
		},
	}
}

func newAgentDisableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "disable <name>",
		Short: "Remove a provider from the project's enabled list (config preserved)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return toggleEnabled(args[0], false)
		},
	}
}

func toggleEnabled(name string, enable bool) error {
	root, _ := os.Getwd()
	c, err := agents.LoadProjectConfig(root)
	if err != nil {
		return err
	}
	idx := -1
	for i, n := range c.Enabled {
		if n == name {
			idx = i
			break
		}
	}
	if enable {
		if idx >= 0 {
			fmt.Printf("provider %q already enabled\n", name)
			return nil
		}
		c.Enabled = append(c.Enabled, name)
	} else {
		if idx < 0 {
			fmt.Printf("provider %q not in enabled list (no-op)\n", name)
			return nil
		}
		c.Enabled = append(c.Enabled[:idx], c.Enabled[idx+1:]...)
	}
	if err := agents.SaveProjectConfig(root, c); err != nil {
		return err
	}
	verb := "enabled"
	if !enable {
		verb = "disabled"
	}
	fmt.Printf("%s %q in <repo>/.kaijutsu/agents.yaml\n", verb, name)
	return nil
}

// --- agent test ------------------------------------------------------

func newAgentTestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "test <name>",
		Short: "Probe a provider for reachability + auth (no swarm dispatch)",
		Long: `Driver-aware probe:
- cli / cli-compat: '<cmd> --version' (existence; auth not verified —
  the first real swarm dispatch surfaces auth failures)
- http: GET <base_url>/models when supported, fallback to a 1-token
  /chat/completions or /v1/messages call
- mcp: not implemented yet (Stage 6)

Exits 0 on success, non-zero with the underlying error on failure.
Does not write to swarm cache.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			root, _ := os.Getwd()
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
			provider, ok := resolved.Providers[name]
			if !ok {
				return fmt.Errorf("agent test %q: not enabled. Run `jutsu agent enable %s` first", name, name)
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
			defer cancel()
			return runAgentTest(ctx, provider, cmd.OutOrStdout())
		},
	}
	return cmd
}

func runAgentTest(ctx context.Context, p *agents.Provider, out interface{ Write(p []byte) (int, error) }) error {
	switch p.Driver {
	case agents.DriverCLI, agents.DriverCLICompat:
		bin := p.Cmd
		if p.Driver == agents.DriverCLICompat {
			bin = p.BaseCLI
		}
		if bin == "" {
			bin = p.Name
		}
		cmd := exec.CommandContext(ctx, bin, "--version")
		raw, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("agent test %q: %s --version failed: %v: %s", p.Name, bin, err, strings.TrimSpace(string(raw)))
		}
		fmt.Fprintf(out, "✓ %s --version: %s\n", bin, strings.TrimSpace(string(raw)))
		return nil
	case agents.DriverHTTP:
		return runHTTPTest(ctx, p, out)
	case agents.DriverMCP:
		return errors.New("mcp driver test lands in Stage 6")
	}
	return fmt.Errorf("agent test %q: unknown driver kind %q", p.Name, p.Driver)
}

func runHTTPTest(ctx context.Context, p *agents.Provider, out interface{ Write(p []byte) (int, error) }) error {
	apiKey := os.Getenv(p.APIKeyEnv)
	if p.APIKeyEnv != "" && apiKey == "" {
		return fmt.Errorf("agent test %q: %s not set in environment", p.Name, p.APIKeyEnv)
	}
	url := strings.TrimRight(p.BaseURL, "/") + "/models"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	if apiKey != "" {
		switch p.Protocol {
		case "anthropic-compat":
			req.Header.Set("x-api-key", apiKey)
			req.Header.Set("anthropic-version", "2023-06-01")
		default:
			req.Header.Set("authorization", "Bearer "+apiKey)
		}
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		fmt.Fprintf(out, "✓ %s GET /models: HTTP %d\n", p.Name, resp.StatusCode)
		return nil
	}
	fmt.Fprintf(out, "× %s GET /models: HTTP %d (provider may not support /models — fallback completion not implemented in this stage)\n", p.Name, resp.StatusCode)
	return fmt.Errorf("HTTP %d from /models", resp.StatusCode)
}

// --- agent migrate ---------------------------------------------------

func newAgentMigrateCmd() *cobra.Command {
	var prefer string
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Migrate kaijutsu.json's deprecated 'agents' field to .kaijutsu/agents.yaml",
		Long: `Reads kaijutsu.json's deprecated 'agents' field and writes the
equivalent 'enabled' list to <repo>/.kaijutsu/agents.yaml. Idempotent.

Conflict resolution when both sources exist with diverging lists:
- --prefer legacy: replace yaml's enabled with the kaijutsu.json list
- --prefer yaml:   keep yaml's enabled (drops legacy entries)
- --prefer merge:  yaml's enabled becomes union of both
- (no flag):       refuse, print diff, exit 1.

In all three --prefer modes the legacy 'agents' field is removed
from kaijutsu.json. Personas / overrides blocks in agents.yaml are
preserved unchanged.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, _ := os.Getwd()
			return runMigrate(root, prefer, cmd.OutOrStdout())
		},
	}
	cmd.Flags().StringVar(&prefer, "prefer", "", "conflict-resolution mode: legacy | yaml | merge")
	return cmd
}

func runMigrate(root, prefer string, out interface{ Write(p []byte) (int, error) }) error {
	mfPath := root + "/kaijutsu.json"
	mf, err := manifest.Load(mfPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintln(out, "no kaijutsu.json present; nothing to migrate")
			return nil
		}
		return err
	}
	if len(mf.Agents) == 0 {
		fmt.Fprintln(out, "no kaijutsu.json.agents field found; nothing to migrate")
		return nil
	}

	yamlCfg, err := agents.LoadProjectConfig(root)
	if err != nil {
		return err
	}

	if listsEqual(mf.Agents, yamlCfg.Enabled) {
		// Lists already in sync. Strip legacy field and exit.
		mf.Agents = nil
		if err := mf.Save(mfPath); err != nil {
			return err
		}
		fmt.Fprintln(out, "kaijutsu.json.agents already matches agents.yaml enabled; legacy field removed")
		return nil
	}

	if prefer == "" {
		fmt.Fprintf(out, "kaijutsu.json.agents = %v\n", mf.Agents)
		fmt.Fprintf(out, "agents.yaml enabled = %v\n", yamlCfg.Enabled)
		return errors.New("kaijutsu.json.agents and agents.yaml.enabled diverge; pass --prefer legacy|yaml|merge to resolve")
	}

	switch prefer {
	case "legacy":
		yamlCfg.Enabled = append([]string(nil), mf.Agents...)
	case "yaml":
		// Keep yamlCfg.Enabled as-is.
	case "merge":
		yamlCfg.Enabled = unionStrings(yamlCfg.Enabled, mf.Agents)
	default:
		return fmt.Errorf("--prefer must be legacy | yaml | merge (got %q)", prefer)
	}
	if err := agents.SaveProjectConfig(root, yamlCfg); err != nil {
		return err
	}
	mf.Agents = nil
	if err := mf.Save(mfPath); err != nil {
		return err
	}
	fmt.Fprintf(out, "migrated: agents.yaml enabled = %v; legacy field removed from kaijutsu.json\n", yamlCfg.Enabled)
	return nil
}

func listsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	as := append([]string(nil), a...)
	bs := append([]string(nil), b...)
	sort.Strings(as)
	sort.Strings(bs)
	for i := range as {
		if as[i] != bs[i] {
			return false
		}
	}
	return true
}

func unionStrings(a, b []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, s := range a {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	for _, s := range b {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

