// eval_swarm_shape.go — v0.10 Stage 2 cobra wiring for the kaijutsu-
// native eval shapes (persona, preset, swarm-skill). Each subcommand
// reads kaijutsu.* extension blocks from a skill's evals.json and
// dispatches via the eval.AgentResolver abstraction.
package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/momentmaker/kaijutsu/cli/internal/agents"
	"github.com/momentmaker/kaijutsu/cli/internal/eval"
	"github.com/momentmaker/kaijutsu/cli/internal/swarm"
)

// personaResolver resolves persona names via the v0.6 driver
// registry. Stage 2 ships with the basic claude/codex/gemini
// drivers; persona-name lookup against agents.yaml is a v0.10.x
// follow-up that pulls in the persona registry layer.
type personaResolver struct {
	timeout time.Duration
	budget  float64
}

func (r *personaResolver) Resolve(ctx context.Context, name string) (eval.TargetAgent, error) {
	driver := agents.For(name)
	if driver == nil {
		return nil, fmt.Errorf("no driver for persona %q (v0.10 Stage 2 supports claude/codex/gemini; persona-registry resolution lands in v0.10.x)", name)
	}
	return &nativeCliEvalAgent{name: swarm.AgentName(name), timeout: r.timeout, budget: r.budget}, nil
}

// presetModeResolver resolves "<preset>:<mode>" tuples. v0.10 Stage
// 2 ships a placeholder that delegates to the preset's default
// agent (claude); future iterations route through the full swarm
// pipeline so --mode quick vs --mode full produces real lift data.
type presetModeResolver struct {
	timeout time.Duration
	budget  float64
}

func (r *presetModeResolver) Resolve(ctx context.Context, name string) (eval.TargetAgent, error) {
	// Stage 2 stub: dispatch via claude regardless of preset:mode
	// pair. Full integration with swarm.runSwarmPipeline is v0.10.x
	// — needs the eval-runner to receive structured swarm.Synthesis
	// outputs, not just text. Stage 2 ships the surface so authors
	// can write evals.json with these blocks; the runtime
	// integration arrives next.
	return &nativeCliEvalAgent{name: swarm.AgentClaude, timeout: r.timeout, budget: r.budget}, nil
}

func newEvalPersonaCmd() *cobra.Command {
	var (
		evalsPath      string
		judgeName      string
		strict         bool
		maxCost        float64
		workspace      string
		concurrency    int
		report         bool
		perAgentBudget float64
		timeout        time.Duration
	)
	cmd := &cobra.Command{
		Use:   "persona",
		Short: "Per-persona head-to-head eval (kaijutsu.personas[] cases)",
		Long: `Run kaijutsu.personas[] cases head-to-head. Each case names
a baseline persona + challenger persona; both run against the same
prompt + are graded by the same judge.

v0.10 Stage 2 supports the native CLI personas (claude, codex,
gemini); persona-registry resolution against agents.yaml ships
v0.10.x.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEvalSwarmShape(cmd, swarmShapeCfg{
				evalsPath:      evalsPath,
				judgeName:      judgeName,
				strict:         strict,
				maxCost:        maxCost,
				workspace:      workspace,
				concurrency:    concurrency,
				report:         report,
				perAgentBudget: perAgentBudget,
				timeout:        timeout,
				kind:           shapePersona,
			})
		},
	}
	bindSwarmShapeFlags(cmd, &evalsPath, &judgeName, &strict, &maxCost, &workspace, &concurrency, &report, &perAgentBudget, &timeout)
	return cmd
}

func newEvalPresetCmd() *cobra.Command {
	var (
		evalsPath      string
		judgeName      string
		strict         bool
		maxCost        float64
		workspace      string
		concurrency    int
		report         bool
		perAgentBudget float64
		timeout        time.Duration
	)
	cmd := &cobra.Command{
		Use:   "preset",
		Short: "Per-preset head-to-head eval (kaijutsu.presets[] cases)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEvalSwarmShape(cmd, swarmShapeCfg{
				evalsPath:      evalsPath,
				judgeName:      judgeName,
				strict:         strict,
				maxCost:        maxCost,
				workspace:      workspace,
				concurrency:    concurrency,
				report:         report,
				perAgentBudget: perAgentBudget,
				timeout:        timeout,
				kind:           shapePreset,
			})
		},
	}
	bindSwarmShapeFlags(cmd, &evalsPath, &judgeName, &strict, &maxCost, &workspace, &concurrency, &report, &perAgentBudget, &timeout)
	return cmd
}

func newEvalSwarmSkillCmd() *cobra.Command {
	var (
		evalsPath      string
		judgeName      string
		strict         bool
		maxCost        float64
		workspace      string
		concurrency    int
		report         bool
		perAgentBudget float64
		timeout        time.Duration
	)
	cmd := &cobra.Command{
		Use:   "swarm-skill",
		Short: "Per-(skill, swarm) eval — does loading a skill into a swarm preset's pipeline change outcomes",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEvalSwarmShape(cmd, swarmShapeCfg{
				evalsPath:      evalsPath,
				judgeName:      judgeName,
				strict:         strict,
				maxCost:        maxCost,
				workspace:      workspace,
				concurrency:    concurrency,
				report:         report,
				perAgentBudget: perAgentBudget,
				timeout:        timeout,
				kind:           shapeSwarmSkill,
			})
		},
	}
	bindSwarmShapeFlags(cmd, &evalsPath, &judgeName, &strict, &maxCost, &workspace, &concurrency, &report, &perAgentBudget, &timeout)
	return cmd
}

func bindSwarmShapeFlags(cmd *cobra.Command,
	evalsPath, judgeName *string,
	strict *bool,
	maxCost *float64,
	workspace *string,
	concurrency *int,
	report *bool,
	perAgentBudget *float64,
	timeout *time.Duration,
) {
	cmd.Flags().StringVar(evalsPath, "evals", "", "path to evals.json (required)")
	cmd.Flags().StringVar(judgeName, "judge", "claude", "model used to grade outputs")
	cmd.Flags().BoolVar(strict, "strict", false, "exit 1 on baseline-vs-challenger regression")
	cmd.Flags().Float64Var(maxCost, "max-cost", 20.00, "abort if pre-flight estimate exceeds this cap (USD)")
	cmd.Flags().StringVar(workspace, "workspace", "", "artifact root (default: ./.kaijutsu/eval-runs/)")
	cmd.Flags().IntVar(concurrency, "concurrency", 4, "parallel cases")
	cmd.Flags().BoolVar(report, "report", true, "generate the static HTML report")
	cmd.Flags().Float64Var(perAgentBudget, "per-agent-budget", 0.50, "per-call agent budget")
	cmd.Flags().DurationVar(timeout, "timeout", 600*time.Second, "per-call timeout")
}

type swarmShapeKind int

const (
	shapePersona swarmShapeKind = iota
	shapePreset
	shapeSwarmSkill
)

type swarmShapeCfg struct {
	evalsPath      string
	judgeName      string
	strict         bool
	maxCost        float64
	workspace      string
	concurrency    int
	report         bool
	perAgentBudget float64
	timeout        time.Duration
	kind           swarmShapeKind
}

// runEvalSwarmShape is the shared dispatcher for the three Stage 2
// subcommands. Reads the evals.json, picks the right runner via
// `kind`, writes artifacts the same way Stage 1 does.
func runEvalSwarmShape(cmd *cobra.Command, cfg swarmShapeCfg) error {
	if cfg.evalsPath == "" {
		return fmt.Errorf("--evals <path> is required")
	}
	data, err := os.ReadFile(cfg.evalsPath)
	if err != nil {
		return fmt.Errorf("read evals.json: %w", err)
	}
	suite, err := eval.ParseAtPath(data, cfg.evalsPath)
	if err != nil {
		return err
	}
	if cfg.workspace == "" {
		cfg.workspace = filepath.Join(".kaijutsu", "eval-runs")
	}
	stderr := cmd.ErrOrStderr()
	release, err := eval.AcquireLock(cfg.workspace, stderr)
	if err != nil {
		return err
	}
	defer release()
	iter, err := eval.NextIteration(cfg.workspace)
	if err != nil {
		return err
	}
	paths, err := eval.PreparePaths(cfg.workspace, iter)
	if err != nil {
		return err
	}

	// Resolve judge.
	judgeAgent, err := resolveEvalAgent(cfg.judgeName, "judge", cfg.perAgentBudget, cfg.timeout)
	if err != nil {
		return err
	}
	skillDir := filepath.Dir(filepath.Dir(cfg.evalsPath))
	tmpl, err := eval.LoadJudgeTemplate(skillDir)
	if err != nil {
		return err
	}

	opts := eval.SwarmShapeOpts{
		Workspace:     cfg.workspace,
		Judge:         judgeAgent,
		JudgeName:     swarm.AgentName(judgeAgent.Name()),
		JudgeTemplate: tmpl,
		Strict:        cfg.strict,
		Concurrency:   cfg.concurrency,
		MaxCostUSD:    cfg.maxCost,
		PerCallBudget: cfg.perAgentBudget,
		StderrW:       stderr,
	}

	var res *eval.RunResult
	var baselineSide, challengerSide string
	switch cfg.kind {
	case shapePersona:
		opts.Resolver = &personaResolver{timeout: cfg.timeout, budget: cfg.perAgentBudget}
		res, err = eval.RunPersonaSuite(cmd.Context(), suite, opts)
		if err == nil && len(suite.Kaijutsu.Personas) > 0 {
			baselineSide = suite.Kaijutsu.Personas[0].Baseline
			challengerSide = suite.Kaijutsu.Personas[0].Challenger
		}
	case shapePreset:
		opts.Resolver = &presetModeResolver{timeout: cfg.timeout, budget: cfg.perAgentBudget}
		res, err = eval.RunPresetSuite(cmd.Context(), suite, opts)
		if err == nil && len(suite.Kaijutsu.Presets) > 0 {
			baselineSide = suite.Kaijutsu.Presets[0].Baseline
			challengerSide = suite.Kaijutsu.Presets[0].Challenger
		}
	case shapeSwarmSkill:
		opts.Resolver = &presetModeResolver{timeout: cfg.timeout, budget: cfg.perAgentBudget}
		res, err = eval.RunSwarmSkillSuite(cmd.Context(), suite, opts)
		if err == nil && len(suite.Kaijutsu.Swarm) > 0 {
			baselineSide = suite.Kaijutsu.Swarm[0].Baseline
			challengerSide = suite.Kaijutsu.Swarm[0].Challenger
		}
	}
	if err != nil {
		return err
	}
	res.Bench.Iteration = iter

	meta := eval.Meta{
		StartedAt:        res.StartedAt,
		FinishedAt:       res.FinishedAt,
		Workspace:        cfg.workspace,
		Iteration:        iter,
		SkillName:        suite.SkillName,
		Target:           "<resolver-driven>",
		Judge:            string(judgeAgent.Name()),
		JudgeTemplateVer: eval.JudgeTemplateVersion,
		Sides:            2,
		ActualCostUSD:    res.ActualCost,
		Strict:           cfg.strict,
		Concurrency:      cfg.concurrency,
	}
	if err := eval.WriteMeta(paths, meta); err != nil {
		return err
	}
	if err := eval.WriteBenchmark(paths, res.Bench); err != nil {
		return err
	}
	if err := eval.WriteBaseline(paths, res.Bench); err != nil {
		return err
	}
	for evalID, er := range res.Bench.Results {
		for side, sr := range er.Sides {
			if err := eval.WriteSideArtifacts(paths, evalID, side, sr); err != nil {
				return err
			}
		}
	}
	if cfg.report {
		html, err := eval.RenderReport(meta, res.Bench, baselineSide, challengerSide, 0)
		if err != nil {
			return err
		}
		if err := os.WriteFile(paths.ReportHTML, []byte(html), 0o644); err != nil {
			return err
		}
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "eval done: %d cases; report: %s\n", res.Bench.TotalEvals, paths.ReportHTML)
	if cfg.strict && res.HasRegression {
		return fmt.Errorf("strict: challenger regressed against baseline on at least one case")
	}
	return nil
}
