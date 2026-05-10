// eval_swarm_shape.go — v0.10 Stage 2 cobra wiring for the kaijutsu-
// native eval shapes (persona, preset, swarm-skill). Each subcommand
// reads kaijutsu.* extension blocks from a skill's evals.json and
// dispatches via the eval.AgentResolver abstraction.
package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/momentmaker/kaijutsu/cli/internal/agents"
	"github.com/momentmaker/kaijutsu/cli/internal/eval"
	"github.com/momentmaker/kaijutsu/cli/internal/findings"
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
	// Lookup early; reject unknown persona names before constructing
	// the agent wrapper. agents.For supports the v0.6 native CLIs;
	// returns nil for personas not in the registry.
	if agents.For(name) == nil {
		return nil, NotFoundError(fmt.Errorf("no driver for persona %q (v0.10 Stage 2 supports claude/codex/gemini; persona-registry resolution lands in v0.10.x)", name))
	}
	return &nativeCliEvalAgent{name: swarm.AgentName(name), timeout: r.timeout, budget: r.budget}, nil
}

// presetModeResolver resolves "<preset>:<mode>" tuples. v0.11.0 wires
// real `swarm.RunPipeline` dispatch — each baseline / challenger side
// runs a full swarm pipeline (privacy gate → consent → fan-out →
// debate-if-full → synthesis → strict-if-set), and the synthesis
// markdown becomes the eval-runner's "agent output" for that side.
//
// The v0.10 stub returned identical claude output regardless of
// preset:mode pair; v0.11.0 makes the per-side outputs reflect the
// actual preset + mode behavior. Eval reports finally show real lift.
type presetModeResolver struct {
	timeout     time.Duration
	budget      float64
	projectRoot string
	stderrW     interface {
		Write(p []byte) (int, error)
	}
}

func (r *presetModeResolver) Resolve(ctx context.Context, name string) (eval.TargetAgent, error) {
	// Parse "<preset>:<mode>"; default mode = "quick" when missing.
	preset := name
	mode := "quick"
	if i := strings.Index(name, ":"); i > 0 {
		preset = name[:i]
		mode = name[i+1:]
	}
	p, err := swarm.LoadPresetWithSkillOverrides(r.projectRoot, preset)
	if err != nil {
		return nil, fmt.Errorf("preset %q: %w", preset, err)
	}
	return &swarmPipelineEvalAgent{
		preset:      p,
		mode:        mode,
		budget:      r.budget,
		timeout:     r.timeout,
		projectRoot: r.projectRoot,
		stderrW:     r.stderrW,
	}, nil
}

// swarmPipelineEvalAgent adapts a configured swarm pipeline (preset +
// mode) into the eval.TargetAgent interface. Each Run dispatches a
// fresh pipeline; the synthesis markdown becomes the side's output.
type swarmPipelineEvalAgent struct {
	preset      *swarm.Preset
	mode        string
	budget      float64
	timeout     time.Duration
	projectRoot string
	stderrW     interface {
		Write(p []byte) (int, error)
	}
}

func (a *swarmPipelineEvalAgent) Name() swarm.AgentName {
	return swarm.AgentName(fmt.Sprintf("%s:%s", a.preset.Name, a.mode))
}

func (a *swarmPipelineEvalAgent) Run(ctx context.Context, prompt string, budget float64) (string, error) {
	// Build a prompt-input context from the eval prompt. Eval prompts
	// are user-authored text, not diffs/files, so InputPrompt is the
	// right kind. CacheKey hashes the (preset, prompt) pair so each
	// (preset:mode, prompt) combo gets its own cache entry.
	ictx := &swarm.InputContext{
		Preset:    a.preset,
		InputKind: swarm.InputPrompt,
		Body:      swarm.CanonicalizePrompt(prompt),
		CacheKey:  swarm.CacheKeyForPrompt(a.preset, prompt),
	}
	stderr := a.stderrW
	if stderr == nil {
		stderr = io.Discard
	}
	// Output goes to discard — eval-runner captures the synthesis
	// markdown via Result.Markdown, not via stdout. Stderr stays
	// connected so warnings (cost, telemetry, dream-archive) surface.
	opts := swarm.PipelineOpts{
		ProjectRoot:             a.projectRoot,
		Preset:                  a.preset,
		Input:                   ictx,
		Mode:                    a.mode,
		PerAgentBudget:          a.budget,
		Timeout:                 a.timeout,
		Yes:                     true, // non-interactive: eval runs are always headless
		Stdout:                  io.Discard,
		Stderr:                  stderr,
		Stdin:                   strings.NewReader(""),
		RecordFindings:          recordFindingsBestEffort,
		ResolveSynthWeights:     resolveSynthWeights,
		ResolveDreamLensWeights: resolveDreamLensWeights,
		ArchiveDreamSession: func(topic, cwd, body string, lensOrder []string, mode string, full bool) (string, error) {
			fp := findings.Fingerprint(cwd)
			return WriteDreamSession(topic, fp, body, lensOrder, mode, full)
		},
	}
	res, err := swarm.RunPipeline(ctx, opts)
	if err != nil {
		return "", err
	}
	return res.Markdown, nil
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
	_ = cmd.MarkFlagRequired("evals")
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
		return UsageError(fmt.Errorf("--evals <path> is required"))
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
		root, _ := projectRoot()
		opts.Resolver = &presetModeResolver{timeout: cfg.timeout, budget: cfg.perAgentBudget, projectRoot: root, stderrW: stderr}
		res, err = eval.RunPresetSuite(cmd.Context(), suite, opts)
		if err == nil && len(suite.Kaijutsu.Presets) > 0 {
			baselineSide = suite.Kaijutsu.Presets[0].Baseline
			challengerSide = suite.Kaijutsu.Presets[0].Challenger
		}
	case shapeSwarmSkill:
		root, _ := projectRoot()
		opts.Resolver = &presetModeResolver{timeout: cfg.timeout, budget: cfg.perAgentBudget, projectRoot: root, stderrW: stderr}
		res, err = eval.RunSwarmSkillSuite(cmd.Context(), suite, opts)
		if err == nil && len(suite.Kaijutsu.Swarm) > 0 {
			baselineSide = suite.Kaijutsu.Swarm[0].Baseline
			challengerSide = suite.Kaijutsu.Swarm[0].Challenger
		}
	default:
		// Defensive — cobra subcommand registration covers the
		// known kinds; future shapes added without updating this
		// switch surface here cleanly rather than nil-deref'ing
		// on res.Bench.Iteration below.
		return fmt.Errorf("unsupported eval shape kind %d", cfg.kind)
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
