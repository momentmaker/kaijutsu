package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
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

Phase 2 ships pr-review (diff input) and doc-review (markdown
artifact input). Stages 5-7 add brainstorm, refactor-plan, and
security-audit.

The swarm probes each CLI for availability + auth and runs only the
ones that pass. With --quick (default), each agent emits findings
independently and a synthesizer pass consolidates them. With --full,
a Pass-2 critique round runs first so agents can dispute each other
before the synthesizer.`,
	}
	cmd.AddCommand(newSwarmPRReviewCmd())
	cmd.AddCommand(newSwarmDocReviewCmd())
	cmd.AddCommand(newSwarmBrainstormCmd())
	return cmd
}

// commonSwarmFlags carries the flag values that every preset
// subcommand shares. Each subcommand registers its own preset-
// specific flags (--pr, --diff-from-branch, etc.) on top.
type commonSwarmFlags struct {
	mode           string
	strict         bool
	maxCostUSD     float64
	synthesizer    string
	perAgentBudget float64
	timeout        time.Duration
	format         string
	postComment    bool
	allowSecrets   bool
	yes            bool
	replayKey      string
	grantConsent   bool
}

func bindCommonFlags(cmd *cobra.Command, f *commonSwarmFlags, supportsPostComment bool) {
	cmd.Flags().StringVar(&f.mode, "mode", "quick", "quick | full (--full enables round-robin debate)")
	cmd.Flags().BoolVar(&f.strict, "strict", false, "lie-to-them filter on synthesis draft")
	cmd.Flags().Float64Var(&f.maxCostUSD, "max-cost", 1.00, "skip optional --full/--strict spend if Pass-1 estimate already exceeds this many USD; warn at end if total exceeds")
	cmd.Flags().Float64Var(&f.perAgentBudget, "per-agent-budget", 0.50, "passed to each agent's --max-budget-usd if supported")
	cmd.Flags().StringVar(&f.synthesizer, "synthesizer", "claude", "which agent runs the synthesis pass")
	cmd.Flags().DurationVar(&f.timeout, "timeout", 180*time.Second, "per-agent invocation timeout")
	cmd.Flags().StringVar(&f.format, "format", "markdown", "markdown (default — synthesized review) | json (raw multi-agent dump, no synthesis)")
	cmd.Flags().BoolVar(&f.allowSecrets, "allow-secrets", false, "bypass the pre-flight secrets scan (DANGEROUS — input will be sent to remote model providers)")
	cmd.Flags().BoolVarP(&f.yes, "yes", "y", false, "non-interactive: skip the consent prompt; require .kaijutsu/<preset>.yaml has allow-multi-model: true")
	cmd.Flags().StringVar(&f.replayKey, "replay", "", "re-run synthesis on cached per-agent findings for a key (SHA for diff presets, hash for files/prompt) without calling model APIs")
	cmd.Flags().BoolVar(&f.grantConsent, "grant-consent", false, "persist `allow-multi-model: true` to .kaijutsu/<preset>.yaml and exit (no swarm run). Use this once per repo when running headless / from inside an agent CLI session.")
	if supportsPostComment {
		cmd.Flags().BoolVar(&f.postComment, "post-comment", false, "after synthesis, post the markdown as a PR comment via gh (edits prior kaijutsu-pr-review comment if found)")
	}
}

func newSwarmPRReviewCmd() *cobra.Command {
	var (
		flags          commonSwarmFlags
		pr             int
		diffFromBranch string
	)
	cmd := &cobra.Command{
		Use:   "pr-review",
		Short: "Multi-agent pull request review",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			projectRoot, _ := os.Getwd()
			preset, err := swarm.LoadPresetWithSkillOverrides(projectRoot, "pr-review")
			if err != nil {
				return err
			}
			if flags.grantConsent {
				return runGrantConsent(cmd, projectRoot, preset)
			}
			if flags.replayKey != "" {
				return runReplay(ctx, cmd, projectRoot, "pr-review", flags.replayKey, flags.synthesizer, flags.perAgentBudget, flags.timeout, flags.postComment)
			}
			ictx, err := swarm.ResolveInput(ctx, preset, swarm.InputOptions{
				PR:             pr,
				DiffFromBranch: diffFromBranch,
			})
			if err != nil {
				return err
			}
			return runSwarmPipeline(ctx, cmd, projectRoot, preset, ictx, flags)
		},
	}
	cmd.Flags().IntVar(&pr, "pr", 0, "PR number (default: detect from current branch)")
	cmd.Flags().StringVar(&diffFromBranch, "diff-from-branch", "", "review the local branch vs base ref instead of a PR (e.g. origin/main)")
	bindCommonFlags(cmd, &flags, true)
	return cmd
}

func newSwarmDocReviewCmd() *cobra.Command {
	var flags commonSwarmFlags
	cmd := &cobra.Command{
		Use:   "doc-review <path>",
		Short: "Multi-agent review of a markdown artifact (spec, plan, decision record)",
		Args:  cobra.MinimumNArgs(0), // 0 args allowed when --replay is set
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			projectRoot, _ := os.Getwd()
			preset, err := swarm.LoadPresetWithSkillOverrides(projectRoot, "doc-review")
			if err != nil {
				return err
			}
			if flags.grantConsent {
				return runGrantConsent(cmd, projectRoot, preset)
			}
			if flags.replayKey != "" {
				return runReplay(ctx, cmd, projectRoot, "doc-review", flags.replayKey, flags.synthesizer, flags.perAgentBudget, flags.timeout, false)
			}
			if len(args) == 0 {
				return errors.New("doc-review requires at least one markdown path argument (or --replay <key>)")
			}
			ictx, err := swarm.ResolveInput(ctx, preset, swarm.InputOptions{
				Files: args,
			})
			if err != nil {
				return err
			}
			return runSwarmPipeline(ctx, cmd, projectRoot, preset, ictx, flags)
		},
	}
	bindCommonFlags(cmd, &flags, false) // no --post-comment for doc-review
	return cmd
}

func newSwarmBrainstormCmd() *cobra.Command {
	var flags commonSwarmFlags
	cmd := &cobra.Command{
		Use:   "brainstorm <prompt>",
		Short: "Multi-agent ideation against a free-form prompt",
		Long: `Run claude / codex / gemini in parallel against a single
free-form prompt with three different ideation lenses, then return
a ranked list of options.

Lenses:
  - claude: long-horizon framing — ideal end-state, ambition gap
  - codex:  code-pattern grounding — concrete patterns + libraries
  - gemini: cross-domain analogy — adjacent fields + prior art

The prompt is a positional arg. Quote it. Cap is 8 KB; longer
prompts abort with a "shorten" hint.

Output: ranked-list markdown (recommended → alternative → risky →
speculative) with cross-cut themes called out separately.`,
		Args: cobra.ArbitraryArgs, // can be 0 with --replay or --grant-consent; otherwise joined
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			projectRoot, _ := os.Getwd()
			preset, err := swarm.LoadPresetWithSkillOverrides(projectRoot, "brainstorm")
			if err != nil {
				return err
			}
			if flags.grantConsent {
				return runGrantConsent(cmd, projectRoot, preset)
			}
			if flags.replayKey != "" {
				return runReplay(ctx, cmd, projectRoot, "brainstorm", flags.replayKey, flags.synthesizer, flags.perAgentBudget, flags.timeout, false)
			}
			if len(args) == 0 {
				return errors.New("brainstorm requires a prompt argument (or --replay <key>)")
			}
			prompt := strings.Join(args, " ")
			ictx, err := swarm.ResolveInput(ctx, preset, swarm.InputOptions{
				Prompt: prompt,
			})
			if err != nil {
				return err
			}
			return runSwarmPipeline(ctx, cmd, projectRoot, preset, ictx, flags)
		},
	}
	bindCommonFlags(cmd, &flags, false) // no --post-comment for brainstorm (no PR)
	return cmd
}

// runSwarmPipeline is the shared per-preset pipeline: privacy gate →
// consent gate → fan-out → synthesis (+ optional debate / strict) →
// cache → optional PR-comment post → stderr summary.
//
// pr-review and doc-review share this. Stages 5-7 will plug in
// brainstorm, refactor-plan, security-audit by adding subcommands
// that fill InputOptions and call this same pipeline.
func runSwarmPipeline(ctx context.Context, cmd *cobra.Command, projectRoot string, preset *swarm.Preset, ictx *swarm.InputContext, f commonSwarmFlags) error {
	out := cmd.OutOrStdout()
	stderr := cmd.ErrOrStderr()

	// Privacy gate: hard-block on secrets unless explicitly
	// overridden. InputDiff + InputFiles get scanned; InputPrompt
	// is user-authored and skips (callers shouldn't dump secrets
	// into a brainstorm prompt; if they do, --allow-secrets isn't
	// the gate that protects them).
	if ictx.InputKind == swarm.InputDiff || ictx.InputKind == swarm.InputFiles {
		hits := swarm.SecretsScan(ictx.Body)
		if len(hits) > 0 && !f.allowSecrets {
			fmt.Fprintf(stderr, "secrets pre-flight scan blocked %d match(es):\n", len(hits))
			for _, h := range hits {
				fmt.Fprintf(stderr, "  - %s (%s)\n", h.Match, h.Reason)
			}
			return errors.New("refusing to send input to remote models; remove the secrets or pass --allow-secrets at your own risk")
		}
		if len(hits) > 0 {
			fmt.Fprintf(stderr, "warning: --allow-secrets bypassed %d secrets-scan hit(s); input WILL be sent to model providers\n", len(hits))
		}
	}

	available := swarm.AvailableAgents()
	if len(available) == 0 {
		return errors.New("no agent CLI available. Install at least one of: claude, codex, gemini, then re-run")
	}

	if cerr := swarm.EnsureConsent(projectRoot, preset, cmd.InOrStdin(), stderr, available, f.yes); cerr != nil {
		return cerr
	}

	fmt.Fprintf(stderr, "swarm: %d agent(s) available: %v\n", len(available), available)

	jobs := make([]swarm.Job, 0, len(available))
	for _, name := range available {
		tmpl, ok := preset.PerAgent[name]
		if !ok {
			fmt.Fprintf(stderr, "swarm: no preset prompt for %s; skipping\n", name)
			continue
		}
		jobs = append(jobs, swarm.Job{
			Agent:  swarm.AgentFor(name),
			Prompt: fmt.Sprintf(tmpl, ictx.Body),
		})
	}
	if len(jobs) == 0 {
		return fmt.Errorf("no jobs assembled — preset %q is missing prompts for every available agent", preset.Name)
	}

	start := time.Now()
	results := swarm.FanOut(ctx, jobs, f.perAgentBudget, f.timeout)
	finished := time.Now()

	run := swarm.SwarmRun{
		Preset:     preset.Name,
		PR:         ictx.PR,
		SHA:        ictx.SHA,
		Mode:       f.mode,
		Agents:     results,
		StartedAt:  start,
		FinishedAt: finished,
	}
	for _, r := range results {
		run.TotalCost += r.Cost
	}

	// JSON-only path: skip synthesis entirely.
	if f.format == "json" {
		if f.maxCostUSD > 0 && run.TotalCost > f.maxCostUSD {
			fmt.Fprintf(stderr, "warning: estimated total cost $%.2f exceeded --max-cost $%.2f\n", run.TotalCost, f.maxCostUSD)
		}
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		if err := enc.Encode(run); err != nil {
			return err
		}
		reportSwarmStderr(stderr, results, finished.Sub(start), run.TotalCost)
		return nil
	}

	// Markdown synthesis path.
	synthAgent := pickSynthesizer(f.synthesizer, results)
	if synthAgent == nil {
		fmt.Fprintln(stderr, "warning: no synthesizer agent available; falling back to JSON dump")
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		if err := enc.Encode(run); err != nil {
			return err
		}
		reportSwarmStderr(stderr, results, finished.Sub(start), run.TotalCost)
		return nil
	}

	overBudget := f.maxCostUSD > 0 && run.TotalCost > f.maxCostUSD
	if overBudget {
		fmt.Fprintf(stderr, "warning: Pass-1 cost $%.2f already exceeds --max-cost $%.2f; skipping optional debate/strict passes\n", run.TotalCost, f.maxCostUSD)
	}

	var (
		synth    *swarm.Synthesis
		synthErr error
	)
	if f.mode == "full" && !overBudget {
		fmt.Fprintln(stderr, "swarm: --full mode — Pass 2 round-robin debate starting")
		pass2 := swarm.Debate(ctx, results, preset, f.perAgentBudget, f.timeout)
		for _, r := range pass2 {
			run.TotalCost += r.Cost
		}
		synth, synthErr = swarm.SynthesizeWithDebate(ctx, results, pass2, synthAgent, preset, f.perAgentBudget, f.timeout)
	} else {
		synth, synthErr = swarm.Synthesize(ctx, results, synthAgent, preset, f.perAgentBudget, f.timeout)
	}
	if synth != nil {
		run.TotalCost += synth.Cost
	}
	if synthErr != nil {
		fmt.Fprintf(stderr, "warning: synthesis: %v (using deterministic fallback markdown)\n", synthErr)
	}
	md := ""
	if synth != nil {
		md = synth.Markdown
	}
	if f.strict && md != "" && !overBudget {
		filtered, lieCost, lieErr := swarm.LieToThem(ctx, md, synthAgent, f.perAgentBudget, f.timeout)
		if lieErr != nil {
			fmt.Fprintf(stderr, "warning: --strict lie-to-them filter failed: %v (keeping unfiltered draft)\n", lieErr)
		} else {
			md = filtered
			run.TotalCost += lieCost
		}
	}
	if f.maxCostUSD > 0 && run.TotalCost > f.maxCostUSD {
		fmt.Fprintf(stderr, "warning: total cost $%.2f exceeded --max-cost $%.2f\n", run.TotalCost, f.maxCostUSD)
	}
	md = appendMarker(md, ictx.CacheKey)
	if cerr := swarm.CacheRun(projectRoot, preset, ictx.CacheKey, results, md); cerr != nil {
		fmt.Fprintf(stderr, "warning: cache write failed: %v\n", cerr)
	}
	fmt.Fprint(out, md)
	if f.postComment {
		if ictx.PR == 0 {
			fmt.Fprintln(stderr, "warning: --post-comment requested but no PR detected; skipping post")
		} else if perr := swarm.PostOrUpdateComment(ctx, ictx.PR, md); perr != nil {
			fmt.Fprintf(stderr, "warning: post comment failed: %v\n", perr)
		} else {
			fmt.Fprintf(stderr, "posted/updated PR comment on #%d\n", ictx.PR)
		}
	}
	reportSwarmStderr(stderr, results, finished.Sub(start), run.TotalCost)
	return nil
}

// appendMarker tacks the kaijutsu-pr-review HTML comment marker
// onto the synthesis markdown so re-runs can find it via
// PostOrUpdateComment. Marker key is the CacheKey (SHA for InputDiff,
// hash for InputFiles/InputPrompt) so re-runs in any preset are
// disambiguable.
func appendMarker(md, key string) string {
	return strings.TrimRight(md, "\n") + "\n\n" + swarm.Marker(swarm.NewRunID(), key) + "\n"
}

// runReplay re-renders the synthesis markdown from cached per-agent
// results without re-calling any model APIs. Useful for tuning the
// synthesizer prompt offline.
func runReplay(ctx context.Context, cmd *cobra.Command, projectRoot, presetName, key, synthesizer string, budget float64, timeout time.Duration, postComment bool) error {
	out := cmd.OutOrStdout()
	stderr := cmd.ErrOrStderr()

	preset, err := swarm.LoadPresetWithSkillOverrides(projectRoot, presetName)
	if err != nil {
		return err
	}
	results, err := swarm.LoadCachedResults(projectRoot, preset, key)
	if err != nil {
		return fmt.Errorf("--replay %s: %w", key, err)
	}
	synthAgent := pickSynthesizer(synthesizer, results)
	if synthAgent == nil {
		return errors.New("--replay: no synthesizer agent available; install claude/codex/gemini first")
	}
	synth, synthErr := swarm.Synthesize(ctx, results, synthAgent, preset, budget, timeout)
	if synthErr != nil {
		fmt.Fprintf(stderr, "warning: synthesis: %v (using fallback markdown)\n", synthErr)
	}
	md := ""
	if synth != nil {
		md = synth.Markdown
	}
	md = appendMarker(md, key)
	fmt.Fprint(out, md)
	if postComment {
		fmt.Fprintln(stderr, "warning: --post-comment + --replay requires the original PR number; resolve manually")
	}
	fmt.Fprintf(stderr, "\nreplay done · preset=%s · synthesizer=%s\n", presetName, synthAgent.Name())
	return nil
}

// runGrantConsent persists allow-multi-model: true for the named
// preset's per-repo config and exits. Used when the user knows
// they're about to run swarm headless (from inside an agent CLI
// session, CI, etc.) and wants to skip the interactive consent
// prompt that would otherwise block.
func runGrantConsent(cmd *cobra.Command, projectRoot string, preset *swarm.Preset) error {
	if err := swarm.SaveConsent(projectRoot, preset); err != nil {
		return fmt.Errorf("persist consent for %s: %w", preset.Name, err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Consent granted for preset %q. Subsequent runs of `jutsu swarm %s` will skip the prompt.\n", preset.Name, preset.Name)
	return nil
}

// pickSynthesizer chooses which agent runs the synthesis pass.
// Prefers the user-requested name if available; otherwise falls back
// to the first agent that produced findings.
func pickSynthesizer(want string, results []swarm.AgentResult) swarm.Agent {
	if want != "" && swarm.Available(swarm.AgentName(want)) {
		return swarm.AgentFor(swarm.AgentName(want))
	}
	for _, r := range results {
		if r.Err == "" && swarm.Available(swarm.AgentName(r.Agent)) {
			return swarm.AgentFor(swarm.AgentName(r.Agent))
		}
	}
	return nil
}

func reportSwarmStderr(stderr interface{ Write(p []byte) (int, error) }, results []swarm.AgentResult, dur time.Duration, totalCost float64) {
	fmt.Fprintf(stderr, "\nswarm done in %s · est cost $%.2f\n",
		dur.Round(time.Millisecond), totalCost)
	for _, r := range results {
		if r.Err != "" {
			fmt.Fprintf(stderr, "  %s: ERROR %s\n", r.Agent, r.Err)
		} else {
			fmt.Fprintf(stderr, "  %s: %d finding(s) · %s\n", r.Agent, len(r.Findings), r.Duration.Round(time.Millisecond))
		}
	}
}
