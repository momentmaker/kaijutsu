package swarm

import (
	"context"
	"sync"
	"time"
)

// Job pairs one agent with its tailored prompt. Each job is run in
// parallel by FanOut.
type Job struct {
	Agent  Agent
	Prompt string
}

// FanOut runs each Job in parallel and returns AgentResults in input
// order. Per-job timeout is enforced via ctx derivations. Results
// include parsed findings (with retry-once on malformed JSON) and a
// rough cost estimate.
func FanOut(ctx context.Context, jobs []Job, budgetUSD float64, perAgentTimeout time.Duration) []AgentResult {
	out := make([]AgentResult, len(jobs))
	var wg sync.WaitGroup
	for i, j := range jobs {
		wg.Add(1)
		go func(idx int, job Job) {
			defer wg.Done()
			start := time.Now()
			subCtx, cancel := context.WithTimeout(ctx, perAgentTimeout)
			defer cancel()
			raw, err := job.Agent.Run(subCtx, job.Prompt, budgetUSD)
			res := AgentResult{
				Agent:    string(job.Agent.Name()),
				Raw:      raw,
				Duration: time.Since(start),
			}
			if err != nil {
				res.Err = err.Error()
				out[idx] = res
				return
			}
			findings, finalRaw := ParseWithRetry(subCtx, job.Agent, job.Prompt, raw, budgetUSD)
			res.Raw = finalRaw
			res.Findings = findings
			res.Cost = EstimateCostUSD(job.Agent.Name(), EstimateTokens(job.Prompt), EstimateTokens(finalRaw))
			out[idx] = res
		}(i, j)
	}
	wg.Wait()
	return out
}
