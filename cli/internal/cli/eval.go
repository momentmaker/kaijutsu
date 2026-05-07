// eval.go — `jutsu eval` cobra subcommand group. v0.10 Stage 1
// ships the `skill` subcommand; Stage 2 adds `persona`, `preset`,
// `swarm-skill`. Spec: docs/specs/2026-05-08-v0.10.0-eval-runner.md.
package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/momentmaker/kaijutsu/cli/internal/agents"
	"github.com/momentmaker/kaijutsu/cli/internal/eval"
	"github.com/momentmaker/kaijutsu/cli/internal/swarm"
)

func newEvalCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "eval",
		Short: "Run evals against skills (single-skill parity in Stage 1; swarm-shape in Stage 2)",
		Long: `v0.10 jutsu eval runner. Reads agentskills.io-format
evals.json files (compat with darkrishabh/agent-skills-eval), runs a
target model against each eval prompt with and without the SKILL.md
loaded into context, has a judge model grade both outputs, and
writes an iteration-N artifact tree with a static HTML report.

Subcommands:
  jutsu eval skill <path>   — single-skill eval (Stage 1)
  jutsu eval persona ...    — per-persona eval (Stage 2)
  jutsu eval preset ...     — per-preset eval (Stage 2)
  jutsu eval swarm-skill ... — per-(skill, swarm) eval (Stage 2)

Stage 1 ships only the skill subcommand. Stage 2 adds the swarm-
shape subcommands.`,
	}
	cmd.AddCommand(newEvalSkillCmd())
	return cmd
}

func newEvalSkillCmd() *cobra.Command {
	var (
		targetName     string
		judgeName      string
		baselineSide   bool
		strict         bool
		maxCost        float64
		workspace      string
		includeIDs     []string
		excludeIDs     []string
		concurrency    int
		report         bool
		baselineFrom   string
		acceptBaseline bool
		perAgentBudget float64
		timeout        time.Duration
	)
	cmd := &cobra.Command{
		Use:   "skill <path>",
		Short: "Run a single-skill eval suite (with_skill vs without_skill)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			skillPath := args[0]
			info, err := os.Stat(skillPath)
			if err != nil {
				return fmt.Errorf("skill path %q: %w", skillPath, err)
			}
			if !info.IsDir() {
				return fmt.Errorf("skill path must be a directory: %s", skillPath)
			}
			evalsJSON := filepath.Join(skillPath, "evals", "evals.json")
			data, err := os.ReadFile(evalsJSON)
			if err != nil {
				if os.IsNotExist(err) {
					fmt.Fprintf(cmd.OutOrStdout(), "no evals.json at %s — skill is not eval-covered yet (this is fine; run `jutsu eval` again after authoring evals)\n", evalsJSON)
					return nil
				}
				return fmt.Errorf("read evals.json: %w", err)
			}
			suite, err := eval.ParseAtPath(data, evalsJSON)
			if err != nil {
				return err
			}

			// SKILL.md body for with_skill side. Empty = skill-not-found.
			skillMD, _ := os.ReadFile(filepath.Join(skillPath, "SKILL.md"))

			// Default workspace under .kaijutsu/eval-runs/.
			if workspace == "" {
				workspace = filepath.Join(".kaijutsu", "eval-runs")
			}

			// Resolve target + judge against available agents. Stage 1
			// ships claude/codex/gemini via the v0.6 driver layer;
			// Stage 2's swarm-shape subcommands honor persona names.
			target, err := resolveEvalAgent(targetName, "target", perAgentBudget, timeout)
			if err != nil {
				return err
			}
			judge, err := resolveEvalAgent(judgeName, "judge", perAgentBudget, timeout)
			if err != nil {
				return err
			}

			stderr := cmd.ErrOrStderr()
			fmt.Fprintf(stderr, "eval: target=%s judge=%s sides=%d strict=%v\n",
				target.Name(), judge.Name(), sideCount(baselineSide), strict)

			// Acquire workspace lock.
			release, err := eval.AcquireLock(workspace, stderr)
			if err != nil {
				return err
			}
			defer release()

			iter, err := eval.NextIteration(workspace)
			if err != nil {
				return err
			}
			paths, err := eval.PreparePaths(workspace, iter)
			if err != nil {
				return err
			}

			// Run.
			res, err := eval.RunSkill(ctx, suite, eval.RunOpts{
				Workspace:     workspace,
				Target:        target,
				Judge:         judge,
				JudgeName:     swarm.AgentName(judge.Name()),
				BaselineSide:  baselineSide,
				Strict:        strict,
				Concurrency:   concurrency,
				MaxCostUSD:    maxCost,
				PerCallBudget: perAgentBudget,
				SkillBody:     string(skillMD),
				IncludeIDs:    includeIDs,
				ExcludeIDs:    excludeIDs,
				StderrW:       stderr,
			})
			if err != nil {
				return err
			}
			res.Bench.Iteration = iter

			// Write artifacts.
			meta := eval.Meta{
				StartedAt:        res.StartedAt,
				FinishedAt:       res.FinishedAt,
				Workspace:        workspace,
				Iteration:        iter,
				SkillName:        suite.SkillName,
				Target:           string(target.Name()),
				Judge:            string(judge.Name()),
				JudgeTemplateVer: eval.JudgeTemplateVersion,
				Sides:            sideCount(baselineSide),
				EstimatedCostUSD: res.Estimate.Total,
				ActualCostUSD:    res.ActualCost,
				Strict:           strict,
				Concurrency:      concurrency,
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
			if report {
				html, err := eval.RenderReport(meta, res.Bench, eval.SideWithSkill, eval.SideWithoutSkill, res.Estimate.Total)
				if err != nil {
					return err
				}
				if err := os.WriteFile(paths.ReportHTML, []byte(html), 0o644); err != nil {
					return err
				}
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "eval done: %d/%d evals passed (with_skill side); report: %s\n",
				countPassing(res.Bench, eval.SideWithSkill), res.Bench.TotalEvals, paths.ReportHTML)
			fmt.Fprintf(out, "actual cost: $%.4f (estimated $%.4f)\n", res.ActualCost, res.Estimate.Total)

			// --strict regression check.
			if strict && res.HasRegression {
				return errors.New("strict: with_skill regressed against without_skill on at least one eval")
			}
			// --baseline-from + --accept-baseline gate (Stage 3 wires
			// the comparison; Stage 1 honors --accept-baseline as a
			// no-op so callers can pass it eagerly).
			_ = baselineFrom
			_ = acceptBaseline
			return nil
		},
	}
	cmd.Flags().StringVar(&targetName, "target", "", "model used to execute the eval prompt (defaults to first available agent)")
	cmd.Flags().StringVar(&judgeName, "judge", "claude", "model used to grade outputs")
	cmd.Flags().BoolVar(&baselineSide, "baseline-side", true, "run the without_skill baseline side alongside with_skill")
	cmd.Flags().BoolVar(&strict, "strict", false, "exit 1 on with_skill regression vs without_skill")
	cmd.Flags().Float64Var(&maxCost, "max-cost", 20.00, "abort the eval suite if pre-flight estimated cost exceeds this cap (USD)")
	cmd.Flags().StringVar(&workspace, "workspace", "", "artifact root directory (default: ./.kaijutsu/eval-runs/)")
	cmd.Flags().StringSliceVar(&includeIDs, "include", nil, "eval-id glob filter (e.g. 'auth-*')")
	cmd.Flags().StringSliceVar(&excludeIDs, "exclude", nil, "eval-id glob filter (excluded)")
	cmd.Flags().IntVar(&concurrency, "concurrency", 4, "parallel evals (timing fields are wall-clock and influenced by concurrency)")
	cmd.Flags().BoolVar(&report, "report", true, "generate the static HTML report")
	cmd.Flags().StringVar(&baselineFrom, "baseline-from", "", "stateful --strict comparison source (Stage 3; git ref)")
	cmd.Flags().BoolVar(&acceptBaseline, "accept-baseline", false, "operator gate for seeding a new baseline that contains failures (Stage 3)")
	cmd.Flags().Float64Var(&perAgentBudget, "per-agent-budget", 0.50, "per-call budget passed to the agent driver")
	cmd.Flags().DurationVar(&timeout, "timeout", 600*time.Second, "per-call timeout")
	return cmd
}

// resolveEvalAgent maps a name (claude/codex/gemini, or empty for
// auto-detect) to a TargetAgent / JudgeAgent. v0.6 driver layer is
// reused; Stage 2 adds persona-name resolution.
func resolveEvalAgent(name, role string, budget float64, timeout time.Duration) (eval.TargetAgent, error) {
	if name == "" {
		// Auto-detect: pick the first available native CLI.
		for _, a := range swarm.AvailableAgents() {
			return makeEvalAgent(a, budget, timeout), nil
		}
		return nil, fmt.Errorf("no agents available for %s; install at least one of: claude, codex, gemini OR pass --%s explicitly", role, role)
	}
	an := swarm.AgentName(strings.ToLower(name))
	if !swarm.Available(an) {
		return nil, fmt.Errorf("%s agent %q not available (install the CLI or use --%s with a different name)", role, name, role)
	}
	return makeEvalAgent(an, budget, timeout), nil
}

// makeEvalAgent wraps a swarm.AgentName into the TargetAgent /
// JudgeAgent interface (both same shape: Run + Name).
func makeEvalAgent(name swarm.AgentName, budget float64, timeout time.Duration) eval.TargetAgent {
	return &nativeCliEvalAgent{name: name, timeout: timeout, budget: budget}
}

// nativeCliEvalAgent dispatches via the agents-package CLI driver.
// Stage 1 covers claude/codex/gemini; Stage 2 routes through the
// full driver registry (http / mcp / cli-compat) for persona-named
// targets.
type nativeCliEvalAgent struct {
	name    swarm.AgentName
	timeout time.Duration
	budget  float64
}

func (a *nativeCliEvalAgent) Name() swarm.AgentName { return a.name }

func (a *nativeCliEvalAgent) Run(ctx context.Context, prompt string, budget float64) (string, error) {
	if a.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, a.timeout)
		defer cancel()
	}
	driver := agents.For(string(a.name))
	if driver == nil {
		return "", fmt.Errorf("no driver registered for agent %q", a.name)
	}
	res, err := driver.Invoke(ctx, prompt, agents.InvokeOpts{MaxBudgetUSD: budget})
	if err != nil {
		return "", err
	}
	if res.Err != "" {
		return res.Raw, fmt.Errorf("agent %s: %s", a.name, res.Err)
	}
	return res.Raw, nil
}

func sideCount(baselineSide bool) int {
	if baselineSide {
		return 2
	}
	return 1
}

func countPassing(b eval.Benchmark, side string) int {
	n := 0
	for _, r := range b.Results {
		if s, ok := r.Sides[side]; ok && s.Pass {
			n++
		}
	}
	return n
}
