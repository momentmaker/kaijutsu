// swarm_reverse.go — `jutsu swarm reverse` cobra wiring. v0.9.
// Spec-vs-impl drift detector. Takes a SPEC (--spec <path>) + DIFF
// (--diff <range>, default origin/main...HEAD) and runs the reverse
// preset. Output is a drift table with ADDED / OMITTED / CHANGED /
// AMBIGUOUS categories.
package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/momentmaker/kaijutsu/cli/internal/swarm"
)

func newSwarmReverseCmd() *cobra.Command {
	var (
		flags         commonSwarmFlags
		specPath      string
		diffRange     string
		confThreshold float64
		lieToThemArg  string
	)
	cmd := &cobra.Command{
		Use:   "reverse",
		Short: "Spec-vs-impl drift detector. Compares --spec against --diff (default origin/main...HEAD).",
		Long: `Audits a code diff against a spec document for drift. Categorizes
findings into ADDED (in diff, not in spec), OMITTED (in spec, not in
diff), CHANGED (spec said one thing, diff did another), AMBIGUOUS
(spec was vague, diff made a choice).

Use BEFORE merging a feature PR — flags scope creep, missed
acceptance criteria, and unauthorized choices the reviewer should
ratify or push back on.

Example:
  jutsu swarm reverse --spec docs/specs/foo.md --diff origin/main...HEAD

The reverse preset uses a lower default confidence threshold (0.30
vs pr-review's 0.55) because drift detection is harder than code
review — false-positive drifts are easier to dismiss than false-
negative misses. Tune via --confidence-threshold.

The lie-to-them sycophancy filter is OFF by default for the same
reason. Override with --lie-to-them=on to apply v0.7 strict
filtering.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			projectRoot, _ := os.Getwd()

			if specPath == "" {
				return UsageError(errors.New("--spec <path> is required for reverse preset"))
			}
			specBytes, err := os.ReadFile(specPath)
			if err != nil {
				if os.IsNotExist(err) {
					return NotFoundError(fmt.Errorf("read spec %s: %w", specPath, err))
				}
				return fmt.Errorf("read spec %s: %w", specPath, err)
			}

			preset, err := swarm.LoadPresetWithSkillOverrides(projectRoot, "reverse")
			if err != nil {
				return err
			}
			// Bake spec content into DefaultPrompt at command-time —
			// reverse needs both spec and diff in the prompt body but
			// the preset registry's default has only the diff slot.
			// strings.Replace substitutes the SPEC placeholder, leaving
			// the DIFF %s slot for ResolveInput's substitution at
			// dispatch time.
			presetCopy := *preset
			presetCopy.DefaultPrompt = swarm.BuildReversePrompt(string(specBytes))
			preset = &presetCopy

			// Apply v0.9 reverse-preset defaults: lower confidence
			// threshold + lie-to-them OFF. Both are flag-overridable.
			if !cmd.Flags().Changed("strict") {
				flags.strict = lieToThemFromArg(lieToThemArg)
			}
			// confThreshold flows into runSwarmPipeline via the
			// confidenceFloor field on commonSwarmFlags (set here,
			// consumed by runSwarmPipeline → SynthOpts.ConfidenceFloor).
			flags.confidenceFloor = confThreshold

			if flags.grantConsent {
				return runGrantConsent(cmd, projectRoot, preset)
			}
			if flags.replayKey != "" {
				return runReplay(ctx, cmd, projectRoot, "reverse", flags.replayKey, flags.synthesizer, flags.perAgentBudget, flags.timeout, flags.postComment)
			}
			ictx, err := swarm.ResolveInput(ctx, preset, swarm.InputOptions{
				DiffFromBranch: diffRange,
			})
			if err != nil {
				return err
			}
			return runSwarmPipeline(ctx, cmd, projectRoot, preset, ictx, flags)
		},
	}
	cmd.Flags().StringVar(&specPath, "spec", "", "path to the spec markdown to audit against (required)")
	cmd.Flags().StringVar(&diffRange, "diff", "origin/main...HEAD", "git diff range to audit (e.g. origin/main...HEAD, abc123...def456)")
	cmd.Flags().Float64Var(&confThreshold, "confidence-threshold", swarm.ReverseConfidenceThreshold, "drop synthesized findings below this confidence (reverse default 0.30, vs pr-review 0.55)")
	cmd.Flags().StringVar(&lieToThemArg, "lie-to-them", "off", "sycophancy filter: 'off' (reverse default) | 'on' (apply v0.7 strict filter)")
	bindCommonFlags(cmd, &flags, true)
	return cmd
}

// lieToThemFromArg parses the --lie-to-them flag value into the
// commonSwarmFlags.strict bool (the v0.7 sycophancy filter is the
// same machinery; just a different surface name for reverse). Empty
// or unknown values default to false (v0.9 reverse default).
func lieToThemFromArg(arg string) bool {
	switch arg {
	case "on", "true", "1":
		return true
	}
	return false
}
