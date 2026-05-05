package swarm

// EstimateTokens returns a rough token count for a string. Uses
// 4-chars-per-token heuristic which is close enough for Claude/
// GPT/Gemini tokenizers in practice.
func EstimateTokens(s string) int {
	if s == "" {
		return 0
	}
	return (len(s) + 3) / 4
}

// EstimateCostUSD multiplies token count by a per-1k-token rate.
// Stage 1 uses a single rate per agent (input+output averaged) — Stage
// 2 may split input/output and incorporate the ACTUAL output length
// post-call instead of pre-estimating.
func EstimateCostUSD(agent AgentName, promptTokens, outputTokensEst int) float64 {
	rate := perKTokenRate(agent)
	return float64(promptTokens+outputTokensEst) * rate / 1000.0
}

func perKTokenRate(agent AgentName) float64 {
	// Conservative ballpark blended rates as of early 2026.
	// Update when models change. We deliberately overestimate so
	// --max-cost is a safety belt rather than precise.
	switch agent {
	case AgentClaude:
		return 0.005 // ~$5 / 1M tokens, blended sonnet-ish
	case AgentCodex:
		return 0.005 // ~$5 / 1M tokens, blended
	case AgentGemini:
		return 0.002 // ~$2 / 1M tokens, blended flash-ish
	}
	return 0.005
}
