package swarm

import "testing"

// TestIsAgentDisabledByEnv covers the v0.9
// KAIJUTSU_DISABLE_AGENTS=<comma-list> escape hatch. Whitespace-
// tolerant per item; empty env-var = no-op.
func TestIsAgentDisabledByEnv(t *testing.T) {
	cases := []struct {
		env      string
		agent    AgentName
		disabled bool
	}{
		{"", AgentClaude, false},
		{"antigravity", AgentAntigravity, true},
		{"antigravity", AgentClaude, false},
		{"claude,codex", AgentClaude, true},
		{"claude,codex", AgentCodex, true},
		{"claude,codex", AgentAntigravity, false},
		{"  antigravity  ,  codex  ", AgentAntigravity, true}, // whitespace-tolerant
		{"  antigravity  ,  codex  ", AgentClaude, false},
		{"unknown", AgentClaude, false},
	}
	for _, tc := range cases {
		t.Setenv("KAIJUTSU_DISABLE_AGENTS", tc.env)
		got := isAgentDisabledByEnv(tc.agent)
		if got != tc.disabled {
			t.Errorf("env=%q agent=%q: got %v, want %v", tc.env, tc.agent, got, tc.disabled)
		}
	}
}

// TestAvailable_HonorsKaijutsuDisableAgents covers the integration:
// even when an agent CLI is installed (probe would return true),
// the env-var short-circuits Available to false so dispatch + tests
// can simulate "this agent isn't available".
func TestAvailable_HonorsKaijutsuDisableAgents(t *testing.T) {
	t.Setenv("KAIJUTSU_DISABLE_AGENTS", "antigravity")
	if Available(AgentAntigravity) {
		t.Error("Available(antigravity) should respect KAIJUTSU_DISABLE_AGENTS")
	}
}
