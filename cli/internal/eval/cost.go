package eval

import (
	"github.com/momentmaker/kaijutsu/cli/internal/swarm"
)

// CostEstimate contains the pre-flight cost projection produced
// before any model calls are dispatched. The runner aborts before
// dispatch if Total exceeds --max-cost.
type CostEstimate struct {
	Total       float64 // estimated total USD across all evals + judge calls
	PerCall     float64 // average per-call cost (used as the mid-suite-overrun reference)
	NumCalls    int     // total expected model calls (target + judge)
	NumEvals    int     // eval count
	Sides       int     // 1 (no baseline) or 2 (with baseline)
}

// EstimateOpts carries the inputs needed for pre-flight cost
// estimation. Token counts default to a conservative 1500 in / 1500
// out per call; callers can override based on actual prompt sizes
// for more precise estimates (Stage 1 ships defaults; per-eval
// custom estimates are a v0.10.x candidate).
type EstimateOpts struct {
	Suite                  *Suite
	Sides                  int     // 1 or 2
	Target                 swarm.AgentName
	Judge                  swarm.AgentName
	TargetInputTokensEst   int
	TargetOutputTokensEst  int
	JudgeInputTokensEst    int
	JudgeOutputTokensEst   int
}

// DefaultEstimateOpts fills sensible token-count defaults. Callers
// can override per-call.
const (
	defaultTargetInputTokensEst  = 1500
	defaultTargetOutputTokensEst = 1500
	defaultJudgeInputTokensEst   = 1500
	defaultJudgeOutputTokensEst  = 500 // judges return short JSON; lower than target out
)

// Estimate computes the total expected cost for running the suite.
// Formula (per spec):
//
//	total = sum_over_evals(
//	    sides * (target_in_tokens + target_out_tokens) * target_rate
//	  + sides * assertions_count * (judge_in_tokens + judge_out_tokens) * judge_rate
//	)
//
// Output overestimates conservatively (1500 tokens out is generous
// for most evals). --max-cost guard then keeps the cost ceiling
// honest in practice.
func Estimate(opts EstimateOpts) CostEstimate {
	if opts.TargetInputTokensEst == 0 {
		opts.TargetInputTokensEst = defaultTargetInputTokensEst
	}
	if opts.TargetOutputTokensEst == 0 {
		opts.TargetOutputTokensEst = defaultTargetOutputTokensEst
	}
	if opts.JudgeInputTokensEst == 0 {
		opts.JudgeInputTokensEst = defaultJudgeInputTokensEst
	}
	if opts.JudgeOutputTokensEst == 0 {
		opts.JudgeOutputTokensEst = defaultJudgeOutputTokensEst
	}
	if opts.Sides <= 0 {
		opts.Sides = 2
	}

	var total float64
	numCalls := 0
	if opts.Suite != nil {
		for _, e := range opts.Suite.Evals {
			assertions := len(ResolvedAssertions(e))
			// Target: one call per side.
			targetCost := swarm.EstimateCostUSD(opts.Target, opts.TargetInputTokensEst, opts.TargetOutputTokensEst)
			total += float64(opts.Sides) * targetCost
			numCalls += opts.Sides

			// Judge: one call per side per assertion.
			judgeCost := swarm.EstimateCostUSD(opts.Judge, opts.JudgeInputTokensEst, opts.JudgeOutputTokensEst)
			total += float64(opts.Sides*assertions) * judgeCost
			numCalls += opts.Sides * assertions
		}
	}

	perCall := 0.0
	if numCalls > 0 {
		perCall = total / float64(numCalls)
	}

	numEvals := 0
	if opts.Suite != nil {
		numEvals = len(opts.Suite.Evals)
	}

	return CostEstimate{
		Total:    total,
		PerCall:  perCall,
		NumCalls: numCalls,
		NumEvals: numEvals,
		Sides:    opts.Sides,
	}
}

// MidSuiteOverrunThreshold is the per-call ratio that triggers the
// mid-suite abort guard. When an actual call costs > this multiplier
// of the pre-flight per-call estimate, we abort pending evals. The
// "2x" value gives normal token-count variance room while still
// catching runaway prompts.
const MidSuiteOverrunThreshold = 2.0

// IsOverrun reports whether an actual per-call cost exceeds the
// mid-suite overrun threshold relative to the original estimate.
// Returns false when estimatedPerCall is 0 (degenerate).
func IsOverrun(actualPerCall, estimatedPerCall float64) bool {
	if estimatedPerCall <= 0 {
		return false
	}
	return actualPerCall > MidSuiteOverrunThreshold*estimatedPerCall
}
