package eval

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/momentmaker/kaijutsu/cli/internal/swarm"
)

// TargetAgent is the v0.6-driver-layer interface for the model that
// runs the eval prompt. Same shape as JudgeAgent — defined as a
// 1-method interface so tests can swap fakes without pulling the
// full Agent type from swarm.
type TargetAgent interface {
	Run(ctx context.Context, prompt string, budget float64) (string, error)
	Name() swarm.AgentName
}

// RunOpts bundles the user-facing knobs for RunSkill. Keeps the
// public function signature compact and lets tests stub the agents.
type RunOpts struct {
	Workspace    string
	Target       TargetAgent
	Judge        JudgeAgent
	JudgeName    swarm.AgentName // for cost estimate
	BaselineSide bool            // include without_skill side (default true)
	Strict       bool            // exit-1 on regression (returned in Result)
	Concurrency  int             // 1..N evals in parallel (default 4)
	MaxCostUSD   float64         // pre-flight cost cap; 0 = no cap
	PerCallBudget float64        // per-agent driver budget per spec
	SkillBody    string          // SKILL.md body to inject in with_skill side; empty = skill-not-loaded sentinel
	IncludeIDs   []string        // optional eval-id allowlist (empty = all)
	ExcludeIDs   []string        // optional eval-id denylist
	StderrW      io.Writer       // diagnostic warnings (cost overruns, etc.)
}

// RunResult is the full Stage 1 output bundle. Caller (cli) writes
// artifacts via the helpers in artifacts.go and renders the HTML
// report from this.
type RunResult struct {
	Bench       Benchmark
	Estimate    CostEstimate
	ActualCost  float64
	StartedAt   time.Time
	FinishedAt  time.Time
	HasRegression bool // populated iff Strict was set
}

// RunSkill executes every eval in suite against target+judge,
// optionally with the SKILL.md body injected into the prompt for
// the `with_skill` side. Concurrency controls parallel-eval
// dispatch; results aggregate into a Benchmark.
//
// Cost guard:
//   - Pre-flight: Estimate(suite) > MaxCostUSD → abort BEFORE any
//     model calls. Caller observes via err.
//   - Mid-suite: actual per-call cost > 2× estimate → log warning,
//     stop dispatching new evals; completed-eval artifacts retained.
//
// Concurrency disclaimer: timing fields are wall-clock and
// influenced by --concurrency. Pass-fail is the load-bearing
// signal.
func RunSkill(ctx context.Context, suite *Suite, opts RunOpts) (*RunResult, error) {
	if suite == nil {
		return nil, fmt.Errorf("nil suite")
	}
	if opts.Target == nil {
		return nil, fmt.Errorf("RunOpts.Target required")
	}
	if opts.Judge == nil {
		return nil, fmt.Errorf("RunOpts.Judge required")
	}
	if opts.Concurrency <= 0 {
		opts.Concurrency = 4
	}
	if opts.StderrW == nil {
		opts.StderrW = io.Discard
	}

	// Filter evals via include/exclude.
	evals := filterEvals(suite.Evals, opts.IncludeIDs, opts.ExcludeIDs)
	if len(evals) == 0 {
		return nil, fmt.Errorf("no evals match include/exclude filters")
	}

	sides := 2
	if !opts.BaselineSide {
		sides = 1
	}

	// Pre-flight cost estimate + cap check.
	filteredSuite := &Suite{SkillName: suite.SkillName, Evals: evals}
	estimate := Estimate(EstimateOpts{
		Suite:  filteredSuite,
		Sides:  sides,
		Target: opts.Target.Name(),
		Judge:  opts.JudgeName,
	})
	if opts.MaxCostUSD > 0 && estimate.Total > opts.MaxCostUSD {
		return nil, fmt.Errorf(
			"pre-flight cost estimate $%.2f exceeds --max-cost cap $%.2f (suite has %d evals × %d sides; raise --max-cost or narrow --include)",
			estimate.Total, opts.MaxCostUSD, estimate.NumEvals, sides,
		)
	}

	// Run dispatch.
	res := &RunResult{
		Bench: Benchmark{
			SkillName:  suite.SkillName,
			Iteration:  0, // filled by caller after PreparePaths
			TotalEvals: len(evals),
			Results:    make(map[string]EvalResult, len(evals)),
		},
		Estimate:  estimate,
		StartedAt: time.Now(),
	}
	res.Bench.Results = runEvalsParallel(ctx, evals, opts, sides, &res.ActualCost, &res.Bench.Indeterminate, estimate.PerCall)
	res.FinishedAt = time.Now()

	// --strict regression check (stateless mode — Stage 1).
	// Definition: with_skill fails an assertion that without_skill
	// passed in the SAME run. Per-side present check + AND.
	if opts.Strict && opts.BaselineSide {
		for _, evalRes := range res.Bench.Results {
			withSkill, hasW := evalRes.Sides[SideWithSkill]
			withoutSkill, hasWo := evalRes.Sides[SideWithoutSkill]
			if !hasW || !hasWo {
				continue
			}
			if !withSkill.Pass && withoutSkill.Pass {
				res.HasRegression = true
				break
			}
		}
	}
	return res, nil
}

// runEvalsParallel dispatches evals via a worker pool. Returns the
// per-eval result map. Updates *actualCost + *indeterminateCount
// atomically through a single mutex guarding the result struct.
func runEvalsParallel(
	ctx context.Context,
	evals []Eval,
	opts RunOpts,
	sides int,
	actualCost *float64,
	indeterminateCount *int,
	estimatedPerCall float64,
) map[string]EvalResult {
	results := make(map[string]EvalResult, len(evals))
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, opts.Concurrency)
	abortCh := make(chan struct{})
	abortOnce := sync.Once{}

dispatch:
	for _, e := range evals {
		// Race-safe slot acquisition: the SAME select must cover
		// both abort + semaphore so we don't block on a full
		// semaphore while abortCh is closed (gemini pr-review
		// finding). If abortCh fires while waiting, exit cleanly.
		wg.Add(1)
		select {
		case <-abortCh:
			wg.Done()
			break dispatch
		case sem <- struct{}{}:
			// got a slot — proceed to dispatch
		}
		go func(e Eval) {
			defer wg.Done()
			defer func() { <-sem }()
			select {
			case <-abortCh:
				return
			case <-ctx.Done():
				return
			default:
			}
			er := runOneEval(ctx, e, opts, sides)
			mu.Lock()
			results[e.ID] = er
			for _, s := range er.Sides {
				*actualCost += s.CostUSD
				if s.Indeterminate {
					*indeterminateCount++
				}
				// Mid-suite overrun guard.
				avgCallCost := s.CostUSD / float64(max1(1, len(s.Assertions)+1))
				if IsOverrun(avgCallCost, estimatedPerCall) {
					fmt.Fprintf(opts.StderrW, "warning: per-call cost $%.4f exceeds %.0fx estimate $%.4f; aborting pending evals\n",
						avgCallCost, MidSuiteOverrunThreshold, estimatedPerCall)
					abortOnce.Do(func() { close(abortCh) })
				}
			}
			mu.Unlock()
		}(e)
	}
	wg.Wait()
	return results
}

// runOneEval dispatches both sides for a single eval case. Returns
// the per-side result map.
func runOneEval(ctx context.Context, e Eval, opts RunOpts, sides int) EvalResult {
	er := EvalResult{
		ID:    e.ID,
		Name:  e.Name,
		Sides: make(map[string]SideResult, sides),
	}
	er.Sides[SideWithSkill] = runOneSide(ctx, e, opts, true)
	if sides == 2 {
		er.Sides[SideWithoutSkill] = runOneSide(ctx, e, opts, false)
	}
	return er
}

// runOneSide dispatches the target call (with/without SKILL.md) +
// per-assertion judge calls. Aggregates pass/fail/indeterminate per
// the multi-assertion contract: pass = AND of all assertions;
// indeterminate = ANY indeterminate.
func runOneSide(ctx context.Context, e Eval, opts RunOpts, withSkill bool) SideResult {
	prompt := buildPrompt(e, opts.SkillBody, withSkill)
	start := time.Now()
	output, err := opts.Target.Run(ctx, prompt, opts.PerCallBudget)
	dur := time.Since(start)
	sr := SideResult{
		Output:     output,
		DurationMS: dur.Milliseconds(),
	}
	if err != nil {
		sr.Indeterminate = true
		sr.Pass = false
		// Surface the target error as a single synthetic assertion
		// failure so the report drill-down explains why.
		sr.Assertions = []AssertionResult{{
			Assertion: "target call succeeds",
			Pass:      false,
			Reason:    "target call failed: " + err.Error(),
		}}
		return sr
	}
	// Cost estimate per side (target output + judge calls). Coarse;
	// real cost depends on actual token counts which the driver
	// doesn't yet surface. v0.10.x integration with driver-side
	// cost reporting will tighten this.
	sr.CostUSD = swarm.EstimateCostUSD(opts.Target.Name(), swarm.EstimateTokens(prompt), swarm.EstimateTokens(output))

	assertions := ResolvedAssertions(e)
	if len(assertions) == 0 {
		// No grading possible — eval has neither assertions nor
		// expected_output. Mark pass=true (no failure to surface).
		sr.Pass = true
		return sr
	}
	allPass := true
	anyIndeterminate := false
	for _, a := range assertions {
		v := Grade(ctx, opts.Judge, DefaultJudgeTemplate, a, output, opts.PerCallBudget)
		ar := AssertionResult{
			Assertion:     a,
			Pass:          v.Pass,
			Indeterminate: v.Indeterminate,
			Reason:        v.Reason,
			JudgeStage:    v.Stage,
			CostUSD:       swarm.EstimateCostUSD(opts.JudgeName, swarm.EstimateTokens(a)+swarm.EstimateTokens(output), 200),
		}
		sr.CostUSD += ar.CostUSD
		sr.Assertions = append(sr.Assertions, ar)
		if v.Indeterminate {
			anyIndeterminate = true
			allPass = false
		} else if !v.Pass {
			allPass = false
		}
	}
	sr.Pass = allPass
	sr.Indeterminate = anyIndeterminate
	return sr
}

// buildPrompt constructs the target-side prompt. with_skill side
// prepends the SKILL.md body to the eval prompt; without_skill
// uses just the eval prompt. SkillBody empty + withSkill=true is
// equivalent to withSkill=false (callers pass empty when SKILL.md
// isn't on disk).
func buildPrompt(e Eval, skillBody string, withSkill bool) string {
	if !withSkill || strings.TrimSpace(skillBody) == "" {
		return e.Prompt
	}
	return skillBody + "\n\n---\n\n" + e.Prompt
}

// filterEvals applies the --include / --exclude glob filters.
// Empty include = all. Exclude takes precedence over include.
func filterEvals(evals []Eval, include, exclude []string) []Eval {
	out := make([]Eval, 0, len(evals))
	for _, e := range evals {
		if matchesAny(e.ID, exclude) {
			continue
		}
		if len(include) > 0 && !matchesAny(e.ID, include) {
			continue
		}
		out = append(out, e)
	}
	return out
}

// matchesAny is the include/exclude pattern matcher.
//
// **Pattern grammar (documented per v0.10 pr-review feedback):**
//   - Patterns containing `*` are treated as glob: `auth-*` matches
//     `auth-bug` AND `auth-perm` but NOT `failauth`.
//   - Patterns WITHOUT `*` use SUBSTRING match: `auth` matches
//     `auth-bug`, `auth-perm`, AND `failauth`. This is intentional
//     so quick filters like `--include auth` cast a wide net; users
//     who want exact match should anchor with no surrounding
//     content (since exact-match is a substring match where the
//     pattern equals the whole id).
//
// Trade-off: substring fallback is broad. Documented explicitly so
// users can predict matches; v0.11+ may add a `--include-exact`
// flag for the precise-match case if real users hit surprising
// matches in practice.
func matchesAny(id string, patterns []string) bool {
	for _, p := range patterns {
		if p == id {
			return true
		}
		if strings.Contains(p, "*") {
			parts := strings.Split(p, "*")
			ok := true
			rest := id
			for i, part := range parts {
				if i == 0 {
					if !strings.HasPrefix(rest, part) {
						ok = false
						break
					}
					rest = rest[len(part):]
					continue
				}
				idx := strings.Index(rest, part)
				if idx < 0 {
					ok = false
					break
				}
				rest = rest[idx+len(part):]
			}
			if ok {
				return true
			}
		} else if strings.Contains(id, p) {
			return true
		}
	}
	return false
}

// max1 returns max(a, b) for ints; tiny helper to avoid pulling
// generic builtins.
func max1(a, b int) int {
	if a > b {
		return a
	}
	return b
}
