package eval

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/momentmaker/kaijutsu/cli/internal/swarm"
)

// AgentResolver maps a name (persona name, preset+mode, etc.) to a
// runnable TargetAgent. Stage 2 abstraction so the swarm-shape
// runners (RunPersona, RunPreset, RunSwarmSkill) can stay generic
// over agent resolution while the cli layer plugs in the real
// persona-registry / preset-mode / driver lookups.
type AgentResolver interface {
	// Resolve returns the agent (or nil + error) for the given name.
	// Implementations: persona-name → driver via agents.yaml;
	// preset+mode → swarm preset dispatch; swarm-skill → wraps the
	// full swarm pipeline.
	Resolve(ctx context.Context, name string) (TargetAgent, error)
}

// SwarmShapeOpts is the shared options struct for all three Stage 2
// swarm-shape runners. Subset of RunOpts focused on baseline-vs-
// challenger semantics. Reuses the judge + cost guard machinery
// from runner.go.
type SwarmShapeOpts struct {
	Workspace     string
	Resolver      AgentResolver
	Judge         JudgeAgent
	JudgeName     swarm.AgentName
	JudgeTemplate string // resolved by caller via LoadJudgeTemplate
	Strict        bool
	Concurrency   int
	MaxCostUSD    float64
	PerCallBudget float64
	StderrW       Writer
}

// RunPersonaSuite executes the kaijutsu.personas[] cases head-to-
// head. Each case names a baseline persona + a challenger persona;
// the runner dispatches both against the case's prompt + grades
// each output against the case's assertions.
//
// Side dirs in the artifact tree are the persona names verbatim
// (e.g. `default-claude/`, `honest-deepseek/`) — sanitized at
// write-time per artifacts.go.
func RunPersonaSuite(ctx context.Context, suite *Suite, opts SwarmShapeOpts) (*RunResult, error) {
	if suite == nil || suite.Kaijutsu == nil || len(suite.Kaijutsu.Personas) == 0 {
		return nil, fmt.Errorf("suite has no kaijutsu.personas cases")
	}
	cases := suite.Kaijutsu.Personas
	res := &RunResult{
		Bench: Benchmark{
			SkillName:  suite.SkillName,
			TotalEvals: len(cases),
			Results:    make(map[string]EvalResult, len(cases)),
		},
		StartedAt: time.Now(),
	}
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, max1(1, opts.Concurrency))
	for _, c := range cases {
		wg.Add(1)
		sem <- struct{}{}
		go func(c PersonaCase) {
			defer wg.Done()
			defer func() { <-sem }()
			er := runOnePersonaCase(ctx, c, opts)
			mu.Lock()
			res.Bench.Results[c.ID] = er
			for _, s := range er.Sides {
				res.ActualCost += s.CostUSD
				if s.Indeterminate {
					res.Bench.Indeterminate++
				}
			}
			mu.Unlock()
		}(c)
	}
	wg.Wait()
	res.FinishedAt = time.Now()
	res.HasRegression = checkBaselineChallengerRegression(res.Bench, cases)
	return res, nil
}

// runOnePersonaCase resolves baseline + challenger personas and
// dispatches both against the case's prompt. Same judge across both
// sides so grading is comparable.
func runOnePersonaCase(ctx context.Context, c PersonaCase, opts SwarmShapeOpts) EvalResult {
	er := EvalResult{
		ID:    c.ID,
		Name:  "persona: " + c.Baseline + " vs " + c.Challenger,
		Sides: make(map[string]SideResult, 2),
	}
	er.Sides[c.Baseline] = runSwarmShapeSide(ctx, c.Baseline, c.Prompt, c.Assertions, opts)
	er.Sides[c.Challenger] = runSwarmShapeSide(ctx, c.Challenger, c.Prompt, c.Assertions, opts)
	return er
}

// RunPresetSuite executes the kaijutsu.presets[] cases. Each case
// names a preset + baseline mode + challenger mode (e.g. dream
// quick vs full). Side dirs use the mode names.
//
// Sequential dispatch (no concurrency): preset evals fan out to
// the full swarm pipeline per side, which itself dispatches N
// agents in parallel. Running multiple preset cases in parallel
// would violate --max-cost without the per-call cost tracking
// runner.go uses for skill evals. RunPersonaSuite IS parallel
// because each persona = single agent invocation; preset/swarm-
// skill are not because each side = full swarm run.
func RunPresetSuite(ctx context.Context, suite *Suite, opts SwarmShapeOpts) (*RunResult, error) {
	if suite == nil || suite.Kaijutsu == nil || len(suite.Kaijutsu.Presets) == 0 {
		return nil, fmt.Errorf("suite has no kaijutsu.presets cases")
	}
	cases := suite.Kaijutsu.Presets
	res := &RunResult{
		Bench: Benchmark{
			SkillName:  suite.SkillName,
			TotalEvals: len(cases),
			Results:    make(map[string]EvalResult, len(cases)),
		},
		StartedAt: time.Now(),
	}
	for _, c := range cases {
		er := runOnePresetCase(ctx, c, opts)
		res.Bench.Results[c.ID] = er
		for _, s := range er.Sides {
			res.ActualCost += s.CostUSD
			if s.Indeterminate {
				res.Bench.Indeterminate++
			}
		}
	}
	res.FinishedAt = time.Now()
	res.HasRegression = checkBaselineChallengerRegressionPreset(res.Bench, cases)
	return res, nil
}

func runOnePresetCase(ctx context.Context, c PresetCase, opts SwarmShapeOpts) EvalResult {
	er := EvalResult{
		ID:    c.ID,
		Name:  "preset " + c.Preset + ": " + c.Baseline + " vs " + c.Challenger,
		Sides: make(map[string]SideResult, 2),
	}
	// Resolver receives "<preset>:<mode>" so the cli layer can
	// dispatch the full swarm pipeline with the right --mode flag.
	er.Sides[c.Baseline] = runSwarmShapeSide(ctx, c.Preset+":"+c.Baseline, c.Prompt, c.Assertions, opts)
	er.Sides[c.Challenger] = runSwarmShapeSide(ctx, c.Preset+":"+c.Challenger, c.Prompt, c.Assertions, opts)
	return er
}

// RunSwarmSkillSuite executes kaijutsu.swarm[] cases — does loading
// a skill into a swarm preset's pipeline outperform the same preset
// without the skill. Side dirs typically use
// `with_skill_in_swarm/` + `without_skill_in_swarm/`.
func RunSwarmSkillSuite(ctx context.Context, suite *Suite, opts SwarmShapeOpts) (*RunResult, error) {
	if suite == nil || suite.Kaijutsu == nil || len(suite.Kaijutsu.Swarm) == 0 {
		return nil, fmt.Errorf("suite has no kaijutsu.swarm cases")
	}
	cases := suite.Kaijutsu.Swarm
	res := &RunResult{
		Bench: Benchmark{
			SkillName:  suite.SkillName,
			TotalEvals: len(cases),
			Results:    make(map[string]EvalResult, len(cases)),
		},
		StartedAt: time.Now(),
	}
	for _, c := range cases {
		er := runOneSwarmCase(ctx, c, opts)
		res.Bench.Results[c.ID] = er
		for _, s := range er.Sides {
			res.ActualCost += s.CostUSD
			if s.Indeterminate {
				res.Bench.Indeterminate++
			}
		}
	}
	res.FinishedAt = time.Now()
	res.HasRegression = checkBaselineChallengerRegressionSwarm(res.Bench, cases)
	return res, nil
}

func runOneSwarmCase(ctx context.Context, c SwarmCase, opts SwarmShapeOpts) EvalResult {
	er := EvalResult{
		ID:    c.ID,
		Name:  "swarm-skill " + c.Preset,
		Sides: make(map[string]SideResult, 2),
	}
	prompt := c.Prompt
	if prompt == "" {
		prompt = c.Diff // fallback: when no explicit prompt, use the diff as input
	}
	// Resolver receives "<preset>:<side>" so the cli layer can
	// dispatch with/without skill in swarm appropriately.
	er.Sides[c.Baseline] = runSwarmShapeSide(ctx, c.Preset+":"+c.Baseline, prompt, c.Assertions, opts)
	er.Sides[c.Challenger] = runSwarmShapeSide(ctx, c.Preset+":"+c.Challenger, prompt, c.Assertions, opts)
	return er
}

// runSwarmShapeSide dispatches one side of a swarm-shape eval. The
// agent-resolution path lets the cli layer plug in different
// strategies (persona registry, preset+mode dispatch, full swarm
// pipeline) without runner.go knowing the specifics.
func runSwarmShapeSide(ctx context.Context, name, prompt string, assertions []string, opts SwarmShapeOpts) SideResult {
	start := time.Now()
	target, err := opts.Resolver.Resolve(ctx, name)
	if err != nil {
		return SideResult{
			Indeterminate: true,
			Pass:          false,
			DurationMS:    time.Since(start).Milliseconds(),
			Assertions:    []AssertionResult{{Assertion: "agent resolves", Pass: false, Reason: err.Error()}},
		}
	}
	output, err := target.Run(ctx, prompt, opts.PerCallBudget)
	dur := time.Since(start)
	sr := SideResult{
		Output:     output,
		DurationMS: dur.Milliseconds(),
	}
	if err != nil {
		sr.Indeterminate = true
		sr.Pass = false
		sr.Assertions = []AssertionResult{{Assertion: "side runs", Pass: false, Reason: err.Error()}}
		return sr
	}
	sr.CostUSD = swarm.EstimateCostUSD(target.Name(), swarm.EstimateTokens(prompt), swarm.EstimateTokens(output))
	template := opts.JudgeTemplate
	if template == "" {
		template = DefaultJudgeTemplate
	}
	allPass := true
	anyIndeterminate := false
	for _, a := range assertions {
		v := Grade(ctx, opts.Judge, template, a, output, opts.PerCallBudget)
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

// checkBaselineChallengerRegression: regression = challenger fails
// what baseline passed (the inverse of skill eval — challenger is
// the "new thing under test"; we ship it only if it's at least as
// good as baseline).
func checkBaselineChallengerRegression(b Benchmark, cases []PersonaCase) bool {
	for _, c := range cases {
		er, ok := b.Results[c.ID]
		if !ok {
			continue
		}
		baseline, hasB := er.Sides[c.Baseline]
		challenger, hasC := er.Sides[c.Challenger]
		if !hasB || !hasC {
			continue
		}
		if baseline.Pass && !challenger.Pass {
			return true
		}
	}
	return false
}

func checkBaselineChallengerRegressionPreset(b Benchmark, cases []PresetCase) bool {
	for _, c := range cases {
		er, ok := b.Results[c.ID]
		if !ok {
			continue
		}
		baseline, hasB := er.Sides[c.Baseline]
		challenger, hasC := er.Sides[c.Challenger]
		if !hasB || !hasC {
			continue
		}
		if baseline.Pass && !challenger.Pass {
			return true
		}
	}
	return false
}

func checkBaselineChallengerRegressionSwarm(b Benchmark, cases []SwarmCase) bool {
	for _, c := range cases {
		er, ok := b.Results[c.ID]
		if !ok {
			continue
		}
		baseline, hasB := er.Sides[c.Baseline]
		challenger, hasC := er.Sides[c.Challenger]
		if !hasB || !hasC {
			continue
		}
		if baseline.Pass && !challenger.Pass {
			return true
		}
	}
	return false
}
