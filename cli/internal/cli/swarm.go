package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/momentmaker/kaijutsu/cli/internal/swarm"
	"github.com/spf13/cobra"
)

func newSwarmCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "swarm <preset>",
		Short: "Run multiple agents in parallel against a shared task and aggregate findings",
		Long: `Orchestrate claude / codex / gemini in parallel against the same input
and aggregate their structured findings.

Phase 1 ships one preset: pr-review. Phase 2 generalizes to brainstorm,
refactor-plan, and security-audit.

The swarm probes each CLI for availability + auth and runs only the
ones that pass. With --quick (default), each agent emits findings
independently and a synthesizer pass consolidates them. With --full,
a Pass-2 critique round runs first so agents can dispute each other
before the synthesizer.`,
	}
	cmd.AddCommand(newSwarmPRReviewCmd())
	return cmd
}

func newSwarmPRReviewCmd() *cobra.Command {
	var (
		pr             int
		mode           string
		strict         bool
		maxCostUSD     float64
		synthesizer    string
		perAgentBudget float64
		timeout        time.Duration
		diffFromBranch string
	)
	cmd := &cobra.Command{
		Use:   "pr-review",
		Short: "Multi-agent pull request review",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			out := cmd.OutOrStdout()
			stderr := cmd.ErrOrStderr()

			pctx, err := resolveSwarmDiff(ctx, pr, diffFromBranch)
			if err != nil {
				return err
			}
			if strings.TrimSpace(pctx.Diff) == "" {
				return errors.New("diff is empty; nothing to review")
			}

			available := swarm.AvailableAgents()
			if len(available) == 0 {
				return errors.New("no agent CLI available. Install at least one of: claude, codex, gemini, then re-run")
			}
			fmt.Fprintf(stderr, "swarm: %d agent(s) available: %v\n", len(available), available)

			preset, err := swarm.PresetFor("pr-review")
			if err != nil {
				return err
			}

			jobs := make([]swarm.Job, 0, len(available))
			for _, name := range available {
				tmpl, ok := preset.PerAgent[name]
				if !ok {
					fmt.Fprintf(stderr, "swarm: no preset prompt for %s; skipping\n", name)
					continue
				}
				jobs = append(jobs, swarm.Job{
					Agent:  swarm.AgentFor(name),
					Prompt: fmt.Sprintf(tmpl, pctx.Diff),
				})
			}
			if len(jobs) == 0 {
				return errors.New("no jobs assembled — preset is missing prompts for every available agent")
			}

			start := time.Now()
			results := swarm.FanOut(ctx, jobs, perAgentBudget, timeout)
			finished := time.Now()

			run := swarm.SwarmRun{
				Preset:     "pr-review",
				PR:         pctx.PR,
				SHA:        pctx.SHA,
				Mode:       mode,
				Agents:     results,
				StartedAt:  start,
				FinishedAt: finished,
			}
			for _, r := range results {
				run.TotalCost += r.Cost
			}
			if maxCostUSD > 0 && run.TotalCost > maxCostUSD {
				fmt.Fprintf(stderr, "warning: estimated total cost $%.2f exceeded --max-cost $%.2f\n", run.TotalCost, maxCostUSD)
			}

			enc := json.NewEncoder(out)
			enc.SetIndent("", "  ")
			if err := enc.Encode(run); err != nil {
				return err
			}

			fmt.Fprintf(stderr, "\nswarm done in %s · est cost $%.2f\n",
				finished.Sub(start).Round(time.Millisecond), run.TotalCost)
			for _, r := range results {
				if r.Err != "" {
					fmt.Fprintf(stderr, "  %s: ERROR %s\n", r.Agent, r.Err)
				} else {
					fmt.Fprintf(stderr, "  %s: %d finding(s) · %s\n", r.Agent, len(r.Findings), r.Duration.Round(time.Millisecond))
				}
			}

			// Stages 2–3 plug in here: synthesize results into markdown,
			// optionally run a Pass-2 debate first, optionally lie-to-them
			// filter the synthesis draft.
			_ = strict
			_ = synthesizer
			return nil
		},
	}
	cmd.Flags().IntVar(&pr, "pr", 0, "PR number (default: detect from current branch)")
	cmd.Flags().StringVar(&diffFromBranch, "diff-from-branch", "", "review the local branch vs base ref instead of a PR (e.g. origin/main)")
	cmd.Flags().StringVar(&mode, "mode", "quick", "quick | full (Stage 3+: --full enables round-robin debate)")
	cmd.Flags().BoolVar(&strict, "strict", false, "(Stage 3) lie-to-them filter on synthesis draft")
	cmd.Flags().Float64Var(&maxCostUSD, "max-cost", 1.00, "warn (Stage 1) / abort (Stage 2+) if estimated total cost exceeds this many USD")
	cmd.Flags().Float64Var(&perAgentBudget, "per-agent-budget", 0.50, "passed to each agent's --max-budget-usd if supported")
	cmd.Flags().StringVar(&synthesizer, "synthesizer", "claude", "(Stage 2) which agent runs the synthesis pass")
	cmd.Flags().DurationVar(&timeout, "timeout", 180*time.Second, "per-agent invocation timeout")
	return cmd
}

// resolveSwarmDiff figures out which diff to review based on flags.
// --diff-from-branch wins; --pr uses gh; otherwise auto-detect a PR
// for the current branch.
func resolveSwarmDiff(ctx context.Context, pr int, diffFromBranch string) (*swarm.PRContext, error) {
	if diffFromBranch != "" {
		return swarm.FetchBranchDiff(ctx, diffFromBranch)
	}
	return swarm.FetchPRContext(ctx, pr)
}
