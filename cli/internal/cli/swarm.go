package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/momentmaker/kaijutsu/cli/internal/agents"
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
	cmd.AddCommand(newSwarmRefactorPlanCmd())
	cmd.AddCommand(newSwarmSecurityAuditCmd())
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
	personas       []string // v0.6 Stage 3b — opt into persona-driven dispatch
	estimate       bool    // v0.6 Stage 4 — dry-run, print cost projection, exit 0
	noTelemWarn    bool    // v0.6 Stage 4 — suppress the cli-compat one-shot warning
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
	cmd.Flags().StringSliceVar(&f.personas, "personas", nil, "comma-separated persona names to dispatch (v0.6 Stage 3b — opts into agents.yaml-driven dispatch; without this flag, legacy v0.5 cli-only auto-detect path runs)")
	cmd.Flags().BoolVar(&f.estimate, "estimate", false, "dry-run: print per-persona token + cost projection table, then exit 0 without invoking agents")
	cmd.Flags().BoolVar(&f.noTelemWarn, "no-telemetry-warning", false, "suppress the cli-compat one-shot warning about metadata leakage to harness CLI vendor")
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
		// ArbitraryArgs allows any count including 0; the no-args case
		// is enforced inside RunE so --replay / --grant-consent can
		// short-circuit without requiring positional paths. cobra
		// validators run before RunE so a stricter Args here would
		// also block the short-circuit paths.
		Args: cobra.ArbitraryArgs,
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

func newSwarmRefactorPlanCmd() *cobra.Command {
	var flags commonSwarmFlags
	var goal string
	cmd := &cobra.Command{
		Use:   "refactor-plan <path>... --goal \"<goal>\"",
		Short: "Multi-agent refactor plan: ordered steps with risk per step",
		Long: `Plan a refactor of one or more files toward a goal. Three agents
each contribute steps from a different angle:
  - claude: architectural decomposition (right new shape)
  - codex:  stepwise risk (order minimizes regression risk)
  - gemini: pattern consistency (matches existing repo idioms)

Output: ordered step list with per-step risk assessment + ordering-
disagreement callouts.

The --goal flag is REQUIRED unless --replay or --grant-consent is
set. The goal text counts against the 200 KB InputFiles cap.`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			projectRoot, _ := os.Getwd()
			preset, err := swarm.LoadPresetWithSkillOverrides(projectRoot, "refactor-plan")
			if err != nil {
				return err
			}
			if flags.grantConsent {
				return runGrantConsent(cmd, projectRoot, preset)
			}
			if flags.replayKey != "" {
				return runReplay(ctx, cmd, projectRoot, "refactor-plan", flags.replayKey, flags.synthesizer, flags.perAgentBudget, flags.timeout, false)
			}
			if len(args) == 0 {
				return errors.New("refactor-plan requires at least one file path argument (or --replay <key>)")
			}
			if strings.TrimSpace(goal) == "" {
				return errors.New("refactor-plan requires --goal \"<refactor goal>\". Example: --goal \"extract HTTP handler into its own service\"")
			}
			ictx, err := swarm.ResolveInput(ctx, preset, swarm.InputOptions{
				Files: args,
				Goal:  goal,
			})
			if err != nil {
				return err
			}
			return runSwarmPipeline(ctx, cmd, projectRoot, preset, ictx, flags)
		},
	}
	cmd.Flags().StringVar(&goal, "goal", "", "refactor goal (REQUIRED) — one-line description of the desired end-state")
	bindCommonFlags(cmd, &flags, false) // no --post-comment for refactor-plan (no PR)
	return cmd
}

func newSwarmSecurityAuditCmd() *cobra.Command {
	var flags commonSwarmFlags
	var (
		pr             int
		diffFromBranch string
	)
	cmd := &cobra.Command{
		Use:   "security-audit",
		Short: "Multi-agent security audit (CVSS-aligned). Diff or files in, threat model out.",
		Long: `Audit a code change OR file set for security issues. Three lenses:
  - claude: auth + data flow (boundaries, identity, trust)
  - codex:  injection + privilege escalation (concrete attack vectors)
  - gemini: dependency + supply-chain (third-party trust, version drift)

Severity vocabulary is CVSS-aligned (critical | high | medium | low |
informational), distinct from pr-review's blocker/issue/minor/info.

Two input modes (mutually exclusive):
  - PR mode: --pr <n> or --diff-from-branch <ref>
  - Files mode: positional file/dir paths

--full mode defaults ON for security-audit (high-stakes; the Pass-2
debate catches false-positives that erode trust). Pass --mode quick
to disable explicitly.

Cache directory: .kaijutsu/security-audit-runs/<key>/. Same SHA can
be audited via both pr-review AND security-audit without colliding —
the preset name is part of the cache-key salt.`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			projectRoot, _ := os.Getwd()
			preset, err := swarm.LoadPresetWithSkillOverrides(projectRoot, "security-audit")
			if err != nil {
				return err
			}
			if flags.grantConsent {
				return runGrantConsent(cmd, projectRoot, preset)
			}
			if flags.replayKey != "" {
				return runReplay(ctx, cmd, projectRoot, "security-audit", flags.replayKey, flags.synthesizer, flags.perAgentBudget, flags.timeout, false)
			}
			diffMode := pr != 0 || diffFromBranch != ""
			filesMode := len(args) > 0
			if diffMode && filesMode {
				return errors.New("security-audit: pick ONE input mode — either --pr/--diff-from-branch (diff mode) OR positional file paths (files mode), not both")
			}
			if !diffMode && !filesMode {
				return errors.New("security-audit requires either --pr <n> / --diff-from-branch <ref> (diff mode) or one or more file paths (files mode)")
			}
			// Default --full ON for security-audit. User can override
			// to --mode quick if they want a cheap first-pass scan.
			if !cmd.Flags().Changed("mode") {
				flags.mode = "full"
			}
			// Build the right kind of InputContext per mode. The
			// preset's static metadata declares InputDiff (for
			// registry/help purposes); runtime swap is fine because
			// runSwarmPipeline reads ictx.InputKind, not preset.InputKind.
			var (
				ictx     *swarm.InputContext
				resolveErr error
			)
			if diffMode {
				ictx, resolveErr = swarm.ResolveInput(ctx, preset, swarm.InputOptions{
					PR:             pr,
					DiffFromBranch: diffFromBranch,
				})
			} else {
				// Override InputKind on a per-invocation copy so
				// resolveFilesInput is called even though the preset
				// metadata says InputDiff.
				//
				// Safety note: this is a shallow copy. PerAgent (a map)
				// is shared by reference between filesPreset and preset.
				// Today this is safe because:
				//   (a) LoadPresetWithSkillOverrides above already
				//       returned a freshly-allocated map (skill_prompts.go
				//       deep-copies into a new map for every Find call)
				//       so `preset` is NOT the registry singleton; AND
				//   (b) nothing in this function or in runSwarmPipeline
				//       mutates PerAgent.
				// If a future caller stops going through
				// LoadPresetWithSkillOverrides AND introduces PerAgent
				// mutation, the map share becomes a registry-corruption
				// hazard. Re-allocate the map here at that point.
				filesPreset := *preset
				filesPreset.InputKind = swarm.InputFiles
				ictx, resolveErr = swarm.ResolveInput(ctx, &filesPreset, swarm.InputOptions{
					Files: args,
				})
			}
			if resolveErr != nil {
				return resolveErr
			}
			return runSwarmPipeline(ctx, cmd, projectRoot, preset, ictx, flags)
		},
	}
	cmd.Flags().IntVar(&pr, "pr", 0, "PR number (diff mode; default: detect from current branch)")
	cmd.Flags().StringVar(&diffFromBranch, "diff-from-branch", "", "review the local branch vs base ref (diff mode; e.g. origin/main)")
	bindCommonFlags(cmd, &flags, false) // no --post-comment for security-audit (might leak vuln details to a public PR)
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

	// Estimate dry-run: print cost projection table and exit before
	// touching the privacy gate / consent / fan-out.
	if f.estimate {
		return runEstimate(out, stderr, projectRoot, preset, ictx, f)
	}

	// Telemetry-warning sink: route the cli-compat one-shot warning
	// through cobra's stderr so tests can capture it; suppress when
	// --no-telemetry-warning is set.
	if f.noTelemWarn {
		agents.SetCompatWarningSink(io.Discard)
	} else {
		agents.SetCompatWarningSink(stderr)
	}

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

	// Persona-driven dispatch (v0.6 Stage 3b): when --personas is set,
	// jobs are assembled from agents.yaml-resolved personas instead of
	// auto-detected native CLIs. Legacy v0.5 path runs unchanged when
	// the flag is absent.
	var (
		jobs            []swarm.Job
		personaAdapters []*personaAdapter
	)
	if len(f.personas) > 0 {
		var err error
		jobs, personaAdapters, err = assemblePersonaJobs(projectRoot, preset, ictx, f.personas)
		if err != nil {
			return err
		}
		// Consent uses persona names as the provider list. The
		// existing EnsureConsent contract takes []AgentName, so we
		// reify each persona's name as a synthetic AgentName.
		consentNames := make([]swarm.AgentName, 0, len(personaAdapters))
		for _, p := range personaAdapters {
			consentNames = append(consentNames, p.Name())
		}
		if cerr := swarm.EnsureConsent(projectRoot, preset, cmd.InOrStdin(), stderr, consentNames, f.yes); cerr != nil {
			return cerr
		}
		fmt.Fprintf(stderr, "swarm: %d persona(s) dispatching: %v\n", len(personaAdapters), f.personas)
	} else {
		available := swarm.AvailableAgents()
		if len(available) == 0 {
			return errors.New("no agent CLI available. Install at least one of: claude, codex, gemini, then re-run (or pass --personas to dispatch via agents.yaml)")
		}

		if cerr := swarm.EnsureConsent(projectRoot, preset, cmd.InOrStdin(), stderr, available, f.yes); cerr != nil {
			return cerr
		}

		fmt.Fprintf(stderr, "swarm: %d agent(s) available: %v\n", len(available), available)

		jobs = make([]swarm.Job, 0, len(available))
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
	}

	start := time.Now()
	results := swarm.FanOut(ctx, jobs, f.perAgentBudget, f.timeout)
	finished := time.Now()

	// Persona mode: replace estimated costs with the real billed cost
	// reported by drivers (HTTP driver populates Result.CostUSD from
	// the API's usage block). cli driver leaves the estimate in place
	// because Result.CostUSD is 0 there.
	if len(personaAdapters) > 0 {
		overlayPersonaCosts(results, personaAdapters)
	}

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

	// Hard-fail when EVERY agent errored. Without this guard, a
	// 3/3-error fan-out would proceed to synthesis with an empty
	// findings list, render a "0 findings" markdown report, and
	// bury the per-agent errors in a collapsed <details> block —
	// user reads "no findings" and assumes the input was clean.
	usable := 0
	for _, r := range results {
		if r.Err == "" {
			usable++
		}
	}
	if usable == 0 {
		var b strings.Builder
		fmt.Fprintf(&b, "all %d agent(s) errored; no synthesis performed:", len(results))
		for _, r := range results {
			fmt.Fprintf(&b, "\n  - %s: %s", r.Agent, r.Err)
		}
		reportSwarmStderr(stderr, results, finished.Sub(start), run.TotalCost)
		return errors.New(b.String())
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

	// Markdown synthesis path. In persona mode the synthesizer is
	// picked from the persona list; legacy mode uses native CLI lookup.
	var synthAgent swarm.Agent
	if len(personaAdapters) > 0 {
		synthAgent = pickPersonaSynthesizer(f.synthesizer, results, personaAdapters)
	} else {
		synthAgent = pickSynthesizer(f.synthesizer, results)
	}
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
