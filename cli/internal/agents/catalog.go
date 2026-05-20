package agents

// BuiltinProviders is the vendored catalog of "known-good" providers
// `jutsu agent add <name>` consults for default config. Stage 7 of
// v0.6 extends Stage 2's three native CLIs with reference HTTP
// providers (deepseek, glm, kimi, ollama-local) so users can
// `jutsu agent add deepseek` without writing YAML.
//
// The map is the *catalog* — entries here are NOT auto-enabled. Users
// opt-in via `jutsu agent add <name>` or by writing agents.yaml
// manually.
//
// Cost rates (when present) are accurate as of `RateCardDate`. Stale
// rates trigger a `--estimate` warning after 90 days. Refresh via
// the v0.6.x cron + bench harness work tracked in ROADMAP.
const builtinRateCardDate = "2026-05-06"

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
		"antigravity": {
			Name:   "antigravity",
			Driver: DriverCLI,
			Cmd:    "agy",
		},
		// HTTP providers (opt-in via `jutsu agent add`). Cost rates
		// reflect publicly-listed prices as of builtinRateCardDate;
		// users with negotiated enterprise rates should override
		// per-project via agents.yaml `overrides:` block.
		"deepseek": {
			Name:     "deepseek",
			Driver:   DriverHTTP,
			Protocol: "openai-compat",
			BaseURL:  "https://api.deepseek.com/v1",
			// DeepSeek API surfaces two real model IDs:
			//   - "deepseek-v4-flash" (cheap, fast, no thinking by default)
			//   - "deepseek-v4-pro"   (heavyweight reasoning, ~3-5x output cost)
			// Legacy aliases "deepseek-chat" and "deepseek-reasoner"
			// both route to v4-flash internally and retire 2026-07-24
			// per upstream deprecation. Use the canonical v4-flash here
			// — users who want pro override per-project or per-persona
			// (Persona.Model wired in v0.6.2).
			Model:     "deepseek-v4-flash",
			APIKeyEnv: "DEEPSEEK_API_KEY",
			Cost: &CostRates{
				InputPerMtok:       0.27,
				OutputPerMtok:      1.10,
				CachedInputPerMtok: 0.07,
				RateCardDate:       builtinRateCardDate,
			},
		},
		"glm": {
			Name:      "glm",
			Driver:    DriverHTTP,
			Protocol:  "openai-compat",
			BaseURL:   "https://open.bigmodel.cn/api/paas/v4",
			Model:     "glm-4.6",
			APIKeyEnv: "GLM_API_KEY",
			Cost: &CostRates{
				InputPerMtok:  0.50,
				OutputPerMtok: 1.50,
				RateCardDate:  builtinRateCardDate,
			},
		},
		"kimi": {
			Name:      "kimi",
			Driver:    DriverHTTP,
			Protocol:  "openai-compat",
			BaseURL:   "https://api.moonshot.cn/v1",
			Model:     "moonshot-v1-32k",
			APIKeyEnv: "KIMI_API_KEY",
			Cost: &CostRates{
				InputPerMtok:  3.00,
				OutputPerMtok: 12.00,
				RateCardDate:  builtinRateCardDate,
			},
		},
		"ollama-local": {
			Name:     "ollama-local",
			Driver:   DriverHTTP,
			Protocol: "openai-compat",
			BaseURL:  "http://localhost:11434/v1",
			Model:    "qwen2.5-coder:14b",
			// No APIKeyEnv — Ollama by default has no auth on localhost.
			// No Cost — local inference is free at the API level.
		},
	}
}

// LegacyEnabledMix is the v0.5 default provider set. Synthesized as
// `enabled:` when both global and project config are absent — preserves
// v0.5 behavior for users who never write agents.yaml.
func LegacyEnabledMix() []string {
	return []string{"claude", "codex", "antigravity"}
}
