package agents

// BuiltinProviders is the vendored catalog of "known-good" providers
// `jutsu agent add <name>` consults for default config. Stage 2 ships
// the three native CLIs; Stage 3 will add deepseek/glm/kimi/ollama
// HTTP entries; Stage 6 will add reference MCP providers.
//
// The map is the *catalog* — entries here are NOT auto-enabled. Users
// opt-in via `jutsu agent add <name>` (Stage 5) or by writing
// agents.yaml manually.
//
// Cost rates (when present) are accurate as of `RateCardDate`. Stale
// rates trigger a `--estimate` warning after 90 days.
func BuiltinProviders() map[string]*Provider {
	return map[string]*Provider{
		"claude": {
			Name:   "claude",
			Driver: DriverCLI,
			Cmd:    "claude",
		},
		"codex": {
			Name:   "codex",
			Driver: DriverCLI,
			Cmd:    "codex",
		},
		"gemini": {
			Name:   "gemini",
			Driver: DriverCLI,
			Cmd:    "gemini",
			Args:   []string{"--approval-mode", "plan"},
		},
	}
}

// LegacyEnabledMix is the v0.5 default provider set. Synthesized as
// `enabled:` when both global and project config are absent — preserves
// v0.5 behavior for users who never write agents.yaml.
func LegacyEnabledMix() []string {
	return []string{"claude", "codex", "gemini"}
}
