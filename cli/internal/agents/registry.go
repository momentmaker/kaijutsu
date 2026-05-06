package agents

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
// In v0.6 Stage 1 only the cli driver is wired up. Stages 3-6 add
// http / cli-compat / mcp drivers; Stage 2 introduces YAML-driven
// provider configuration that supersedes this hardcoded switch.
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
