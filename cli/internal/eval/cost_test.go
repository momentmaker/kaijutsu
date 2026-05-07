package eval

import (
	"testing"

	"github.com/momentmaker/kaijutsu/cli/internal/swarm"
)

// TestEstimate_TwoSidesTwoEvalsThreeAssertions covers the typical
// pre-flight calculation. Default token counts; pinned suite shape.
// Verifies NumCalls + Total are within sane bounds.
func TestEstimate_TwoSidesTwoEvalsThreeAssertions(t *testing.T) {
	suite := &Suite{
		SkillName: "x",
		Evals: []Eval{
			{ID: "a", Name: "A", Prompt: "p", Assertions: []string{"a1", "a2", "a3"}},
			{ID: "b", Name: "B", Prompt: "p", Assertions: []string{"b1"}},
		},
	}
	est := Estimate(EstimateOpts{
		Suite:  suite,
		Sides:  2,
		Target: swarm.AgentClaude,
		Judge:  swarm.AgentClaude,
	})
	// Calls: 2 evals × 2 sides target = 4 target calls.
	// Plus 2 sides × (3+1) assertions = 8 judge calls.
	// Total = 12.
	if est.NumCalls != 12 {
		t.Errorf("NumCalls=%d, want 12", est.NumCalls)
	}
	if est.NumEvals != 2 {
		t.Errorf("NumEvals=%d, want 2", est.NumEvals)
	}
	if est.Sides != 2 {
		t.Errorf("Sides=%d, want 2", est.Sides)
	}
	if est.Total <= 0 {
		t.Errorf("Total=%v, want > 0", est.Total)
	}
	// Sanity-bound the cost: 12 calls at ~$0.005/1k blended × ~3000
	// tokens/call ≈ $0.18. Must be in the $0.01 - $1.00 range to
	// catch off-by-orders-of-magnitude regressions.
	if est.Total < 0.01 || est.Total > 1.00 {
		t.Errorf("Total=%v outside ballpark [0.01, 1.00]", est.Total)
	}
}

// TestEstimate_NoBaselineHalvesCost verifies sides=1 path: cost
// scales with side count.
func TestEstimate_NoBaselineHalvesCost(t *testing.T) {
	suite := &Suite{
		SkillName: "x",
		Evals: []Eval{
			{ID: "a", Name: "A", Prompt: "p", Assertions: []string{"a1"}},
		},
	}
	twoSides := Estimate(EstimateOpts{Suite: suite, Sides: 2, Target: swarm.AgentClaude, Judge: swarm.AgentClaude})
	oneSide := Estimate(EstimateOpts{Suite: suite, Sides: 1, Target: swarm.AgentClaude, Judge: swarm.AgentClaude})

	if oneSide.Total*2 < twoSides.Total*0.99 || oneSide.Total*2 > twoSides.Total*1.01 {
		t.Errorf("expected sides=1 to be ~half of sides=2; got %v vs %v", oneSide.Total, twoSides.Total)
	}
}

// TestEstimate_AutoPromotedAssertionCounted covers the auto-promote
// path: an eval with expected_output but no assertions still gets
// ONE judge call counted in the estimate.
func TestEstimate_AutoPromotedAssertionCounted(t *testing.T) {
	suite := &Suite{
		SkillName: "x",
		Evals: []Eval{
			{ID: "a", Name: "A", Prompt: "p", ExpectedOutput: "the answer is 42"},
		},
	}
	est := Estimate(EstimateOpts{Suite: suite, Sides: 2, Target: swarm.AgentClaude, Judge: swarm.AgentClaude})
	// 2 sides × 1 target + 2 sides × 1 auto-promoted assertion = 4 calls.
	if est.NumCalls != 4 {
		t.Errorf("NumCalls=%d, want 4 (auto-promoted assertion counted)", est.NumCalls)
	}
}

// TestIsOverrun_ThresholdBoundary pins the 2× ratio. Catches
// regressions if the constant flips to e.g. 3× silently.
func TestIsOverrun_ThresholdBoundary(t *testing.T) {
	cases := []struct {
		actual    float64
		estimated float64
		want      bool
	}{
		{0.10, 0.05, false}, // exactly 2×, NOT over (strict greater-than)
		{0.11, 0.05, true},  // 2.2× — over
		{0.09, 0.05, false}, // 1.8× — under
		{0.05, 0.05, false}, // equal — under
		{0.05, 0.0, false},  // degenerate estimate — never over
	}
	for _, tc := range cases {
		got := IsOverrun(tc.actual, tc.estimated)
		if got != tc.want {
			t.Errorf("IsOverrun(%v, %v) = %v, want %v", tc.actual, tc.estimated, got, tc.want)
		}
	}
}

// TestEstimate_NilSuiteReturnsZero covers the degenerate input.
func TestEstimate_NilSuiteReturnsZero(t *testing.T) {
	est := Estimate(EstimateOpts{Sides: 2, Target: swarm.AgentClaude, Judge: swarm.AgentClaude})
	if est.Total != 0 || est.NumCalls != 0 {
		t.Errorf("nil suite should produce zero estimate; got %+v", est)
	}
}
