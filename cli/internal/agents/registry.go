package agents

import "fmt"

// Stateless cli driver singletons — zero-field structs, safe to share
// across goroutines. Avoids per-dispatch allocation.
var (
	defaultClaude = &claudeDriver{}
	defaultCodex  = &codexDriver{}
	defaultGemini = &geminiDriver{}
)

// For returns the AgentDriver for a known provider name. Returns nil
// if the name doesn't match a registered driver.
//
// Legacy lookup used by the v0.5 swarm pipeline (parallel.go,
// debate.go). Hardcoded to the three native CLI drivers; does NOT
// consult agents.yaml. Stages 3+ pipeline migrations should call
// BuildDriver(*Provider) against a Resolved config instead.
func For(name string) AgentDriver {
	switch name {
	case "claude":
		return defaultClaude
	case "codex":
		return defaultCodex
	case "gemini":
		return defaultGemini
	}
	return nil
}

// BuildDriver constructs the concrete AgentDriver for a resolved
// Provider. Selects implementation by provider.Driver kind:
//   - DriverCLI       → singleton claude/codex/gemini OR generic cliDriver
//                       in Stage 4 (currently rejects unknown CLI cmds).
//   - DriverHTTP      → httpDriver (this stage).
//   - DriverCLICompat → not yet implemented (Stage 4).
//   - DriverMCP       → not yet implemented (Stage 6).
//
// Constructor-only validation lives here; runtime errors (network,
// auth) surface in Result.Err during Invoke.
func BuildDriver(p *Provider) (AgentDriver, error) {
	if p == nil {
		return nil, fmt.Errorf("BuildDriver: nil provider")
	}
	switch p.Driver {
	case DriverCLI:
		// Stage 1-3a: cli driver dispatch is name-based and ignores
		// p.Cmd. Only claude/codex/gemini are mappable; user-declared
		// cli providers with arbitrary cmds (e.g. opencode, aider) are
		// rejected. Stage 4 will introduce a generic cmd-based driver
		// that uses Provider.Cmd + Provider.Args directly.
		if d := For(p.Name); d != nil {
			return d, nil
		}
		return nil, fmt.Errorf("cli driver for provider %q not implemented (Stage 4 adds generic cmd-based cli drivers; for now, name must be claude|codex|gemini)", p.Name)
	case DriverHTTP:
		if p.BaseURL == "" {
			return nil, fmt.Errorf("provider %q (http): base_url is required", p.Name)
		}
		if p.Model == "" {
			return nil, fmt.Errorf("provider %q (http): model is required", p.Name)
		}
		switch p.Protocol {
		case protocolOpenAI, protocolAnthropic:
			return &httpDriver{provider: p}, nil
		case "":
			return nil, fmt.Errorf("provider %q (http): protocol is required (openai-compat | anthropic-compat)", p.Name)
		}
		return nil, fmt.Errorf("provider %q (http): unsupported protocol %q (allowed: openai-compat, anthropic-compat)", p.Name, p.Protocol)
	case DriverCLICompat:
		if p.BaseCLI == "" {
			return nil, fmt.Errorf("provider %q (cli-compat): base_cli is required (claude | codex | gemini)", p.Name)
		}
		base := For(p.BaseCLI)
		if base == nil {
			return nil, fmt.Errorf("provider %q (cli-compat): base_cli %q is not a known cli driver (allowed: claude, codex, gemini)", p.Name, p.BaseCLI)
		}
		return &cliCompatDriver{provider: p, base: base}, nil
	case DriverMCP:
		if p.Transport == "" {
			return nil, fmt.Errorf("provider %q (mcp): transport is required (stdio | http)", p.Name)
		}
		if p.Transport == "stdio" && p.Command == "" {
			return nil, fmt.Errorf("provider %q (mcp stdio): command is required", p.Name)
		}
		if p.Transport == "http" && p.Endpoint == "" {
			return nil, fmt.Errorf("provider %q (mcp http): endpoint is required", p.Name)
		}
		return &mcpDriver{provider: p}, nil
	}
	return nil, fmt.Errorf("provider %q: unknown driver kind %q", p.Name, p.Driver)
}
