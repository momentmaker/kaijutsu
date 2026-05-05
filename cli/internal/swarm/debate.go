package swarm

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Debate runs a Pass-2 round-robin critique. Each agent receives its
// own Pass-1 findings + every peer's Pass-1 findings, and returns a
// revised list with [agreed]/[disputes]/[new] markers in reasoning.
//
// Returns the Pass-2 results in the same agent order as the input.
// Agents that errored in Pass 1 are skipped (they can't critique
// what they didn't produce).
func Debate(ctx context.Context, pass1 []AgentResult, preset *Preset, budget float64, perAgentTimeout time.Duration) []AgentResult {
	jobs := make([]Job, 0, len(pass1))
	indexByAgent := make(map[string]int, len(pass1))
	for i, r := range pass1 {
		indexByAgent[r.Agent] = i
		if r.Err != "" && len(r.Findings) == 0 {
			continue
		}
		agent := AgentFor(AgentName(r.Agent))
		if agent == nil {
			continue
		}
		ownJSON, _ := json.MarshalIndent(r.Findings, "", "  ")
		peersJSON, _ := json.MarshalIndent(peerFindings(pass1, r.Agent), "", "  ")
		prompt := fmt.Sprintf(preset.Debate, string(ownJSON), string(peersJSON))
		jobs = append(jobs, Job{Agent: agent, Prompt: prompt})
	}
	if len(jobs) == 0 {
		return nil
	}
	pass2 := FanOut(ctx, jobs, budget, perAgentTimeout)
	// Re-align pass2 to the same input order as pass1 so callers can
	// zip Pass-1 and Pass-2 results by index.
	out := make([]AgentResult, len(pass1))
	for i, r := range pass1 {
		out[i] = AgentResult{Agent: r.Agent, Err: "skipped (Pass 1 had no usable findings)"}
		_ = indexByAgent
		for _, p2 := range pass2 {
			if p2.Agent == r.Agent {
				out[i] = p2
				break
			}
		}
	}
	return out
}

func peerFindings(all []AgentResult, self string) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(all)-1)
	for _, r := range all {
		if r.Agent == self || r.Err != "" {
			continue
		}
		out = append(out, map[string]interface{}{
			"agent":    r.Agent,
			"findings": r.Findings,
		})
	}
	return out
}

// SynthesizeWithDebate is the --full counterpart to Synthesize. It
// merges Pass-1 + Pass-2 findings before clustering so contested vs
// durable findings can be distinguished in the table.
func SynthesizeWithDebate(ctx context.Context, pass1, pass2 []AgentResult, synth Agent, preset *Preset, budget float64, perAgentTimeout time.Duration) (*Synthesis, error) {
	combined := mergePasses(pass1, pass2)
	return Synthesize(ctx, combined, synth, preset, budget, perAgentTimeout)
}

// mergePasses replaces each Pass-1 result with its Pass-2 revision
// when the Pass-2 result succeeded; otherwise keeps Pass-1 intact.
// The reasoning field of each finding now carries [agreed]/[disputes]/
// [new] markers (per the Debate prompt template), so downstream
// rendering benefits automatically.
func mergePasses(pass1, pass2 []AgentResult) []AgentResult {
	if len(pass2) == 0 {
		return pass1
	}
	out := make([]AgentResult, len(pass1))
	for i, p1 := range pass1 {
		out[i] = p1
		for _, p2 := range pass2 {
			if p2.Agent == p1.Agent && p2.Err == "" && len(p2.Findings) > 0 {
				out[i] = p2
				break
			}
		}
	}
	return out
}

// LieToThem post-filters a synthesis draft to remove sycophancy,
// fluff, and over-hedging. Stage 3 --strict mode runs this as one
// extra synthesizer call after the main synthesis. Returns the
// trimmed markdown; on failure returns the original draft unchanged.
func LieToThem(ctx context.Context, draft string, synth Agent, budget float64, perAgentTimeout time.Duration) (string, float64, error) {
	prompt := lieToThemPrompt + "\n\nDRAFT:\n" + draft
	subCtx, cancel := context.WithTimeout(ctx, perAgentTimeout)
	defer cancel()
	raw, err := synth.Run(subCtx, prompt, budget)
	if err != nil {
		return draft, 0, err
	}
	cost := EstimateCostUSD(synth.Name(), EstimateTokens(prompt), EstimateTokens(raw))
	return raw, cost, nil
}

const lieToThemPrompt = `You are filtering a code-review markdown draft for sycophancy and fluff.

Rules:
- Cut greetings, validation, hedging ("might be worth considering", "this is a great PR but"), apologies.
- Cut findings whose reasoning amounts to "this could in theory be a problem" without a concrete failure mode.
- Keep blocker- and issue-level findings VERBATIM unless they are clearly fluff. Be conservative on cutting.
- Preserve the markdown structure (headings, table, footer) exactly.
- Return ONLY the trimmed markdown. No commentary, no preamble.`
