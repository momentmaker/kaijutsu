// swarm_user.go — v0.13.0 Preset SDK cobra wiring. Reads project +
// home swarm.yaml at root construction, registers each valid user
// preset in the swarm package's default registry, and adds one
// `jutsu swarm <name>` cobra subcommand per registered preset.
//
// Per-InputKind flag wiring reuses the existing built-in patterns:
//   - InputDiff  → `--pr` / `--diff-from-branch` (mirrors pr-review)
//   - InputFiles → positional <paths> args (mirrors doc-review)
//   - InputPrompt → positional <prompt> args (mirrors brainstorm)
//
// Failure-mode contract: yaml load / parse failures surface to stderr
// as warnings, NOT panics. Built-in subcommands are NEVER affected
// by user-yaml issues (zero-impact failure mode). A recover block
// guards the whole user-preset flow.
package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/momentmaker/kaijutsu/cli/internal/swarm"
)

// addUserPresetSubcommands loads project + home swarm.yaml files,
// registers valid presets in the registry, and adds one cobra
// subcommand per successfully-registered name.
//
// Registry, projectRoot, and homeDir are explicit args (dependency
// injection, no global state — addresses plan-doc-review concern).
//
// Behavior on failure:
//   - Yaml parse error → stderr warning + zero user-preset subcommands.
//     Built-in subcommands NOT affected.
//   - Validation warnings (per-entry) → stderr warning per entry +
//     valid entries still register.
//   - Built-in shadow collision → stderr error + entry skipped + valid
//     entries still register.
//   - Panic deep in load (e.g. yaml lib bug) → recover + stderr warning;
//     built-in subcommands work normally.
func addUserPresetSubcommands(parent *cobra.Command, registry *swarm.PresetRegistry, projectRoot, homeDir string, stderrW io.Writer) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(stderrW, "warning: addUserPresetSubcommands recovered from panic (%v); user presets disabled for this run\n", r)
		}
	}()

	presets, warnings, err := swarm.LoadUserPresets(projectRoot, homeDir)
	for _, w := range warnings {
		fmt.Fprintf(stderrW, "warning: swarm.yaml: %s\n", w)
	}
	if err != nil {
		fmt.Fprintf(stderrW, "warning: load user presets: %v (built-in presets unaffected)\n", err)
		return
	}
	if len(presets) == 0 {
		return
	}

	registered, collisions := registry.RegisterUserPresets(presets)
	for _, c := range collisions {
		fmt.Fprintf(stderrW, "warning: %s\n", c)
	}

	for _, name := range registered {
		up := presets[name]
		parent.AddCommand(newUserPresetSubcommand(name, up))
	}
}

// newUserPresetSubcommand builds a cobra subcommand for a registered
// user preset. Selects the correct flag-binding helper based on
// p.InputKind. Subcommand's Short field is suffixed with the source
// tag (`[user:project]` / `[user:home]`) so help output
// differentiates user presets from built-ins.
func newUserPresetSubcommand(name string, up *swarm.UserPreset) *cobra.Command {
	short := up.Description
	short = fmt.Sprintf("%s [%s]", short, up.Source)

	switch up.InputKind {
	case swarm.InputDiff:
		return newUserPresetDiffCmd(name, short, up)
	case swarm.InputFiles:
		return newUserPresetFilesCmd(name, short, up)
	case swarm.InputPrompt:
		return newUserPresetPromptCmd(name, short, up)
	default:
		// Should be unreachable — LoadUserPresets validates InputKind.
		// Defense-in-depth: emit a stub that errors on invocation.
		return &cobra.Command{
			Use:   name,
			Short: short + " (DISABLED: invalid InputKind)",
			RunE: func(cmd *cobra.Command, args []string) error {
				return fmt.Errorf("user preset %q has invalid InputKind; check swarm.yaml + run `jutsu swarm validate`", name)
			},
		}
	}
}

// newUserPresetDiffCmd mirrors pr-review's flag wiring for InputDiff.
// yaml-set Mode / Personas / ConfidenceFloor become DEFAULT flag
// values; CLI flags override per-invocation.
func newUserPresetDiffCmd(name, short string, up *swarm.UserPreset) *cobra.Command {
	var (
		flags          commonSwarmFlags
		pr             int
		diffFromBranch string
	)
	cmd := &cobra.Command{
		Use:   name,
		Short: short,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			projectRoot, _ := os.Getwd()
			preset, err := swarm.LoadPresetWithSkillOverrides(projectRoot, name)
			if err != nil {
				return err
			}
			if flags.grantConsent {
				return runGrantConsent(cmd, projectRoot, preset)
			}
			if flags.replayKey != "" {
				return runReplay(ctx, cmd, projectRoot, name, flags.replayKey, flags.synthesizer, flags.perAgentBudget, flags.timeout, flags.postComment)
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
	bindCommonFlags(cmd, &flags, true) // user presets get --post-comment by default
	applyUserPresetDefaults(cmd, &flags, up)
	return cmd
}

// newUserPresetFilesCmd mirrors doc-review's positional-paths wiring
// for InputFiles. yaml defaults applied via applyUserPresetDefaults.
func newUserPresetFilesCmd(name, short string, up *swarm.UserPreset) *cobra.Command {
	var flags commonSwarmFlags
	cmd := &cobra.Command{
		Use:   name + " <path>...",
		Short: short,
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			projectRoot, _ := os.Getwd()
			preset, err := swarm.LoadPresetWithSkillOverrides(projectRoot, name)
			if err != nil {
				return err
			}
			if flags.grantConsent {
				return runGrantConsent(cmd, projectRoot, preset)
			}
			if flags.replayKey != "" {
				return runReplay(ctx, cmd, projectRoot, name, flags.replayKey, flags.synthesizer, flags.perAgentBudget, flags.timeout, false)
			}
			if len(args) == 0 {
				return fmt.Errorf("%s requires at least one file path argument (or --replay <key>)", name)
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
	bindCommonFlags(cmd, &flags, false)
	applyUserPresetDefaults(cmd, &flags, up)
	return cmd
}

// newUserPresetPromptCmd mirrors brainstorm's positional-prompt
// wiring for InputPrompt. yaml defaults applied.
func newUserPresetPromptCmd(name, short string, up *swarm.UserPreset) *cobra.Command {
	var flags commonSwarmFlags
	cmd := &cobra.Command{
		Use:   name + " <prompt>",
		Short: short,
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			projectRoot, _ := os.Getwd()
			preset, err := swarm.LoadPresetWithSkillOverrides(projectRoot, name)
			if err != nil {
				return err
			}
			if flags.grantConsent {
				return runGrantConsent(cmd, projectRoot, preset)
			}
			if flags.replayKey != "" {
				return runReplay(ctx, cmd, projectRoot, name, flags.replayKey, flags.synthesizer, flags.perAgentBudget, flags.timeout, false)
			}
			if len(args) == 0 {
				return errors.New("user preset requires a prompt argument (or --replay <key>)")
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
	bindCommonFlags(cmd, &flags, false)
	applyUserPresetDefaults(cmd, &flags, up)
	return cmd
}

// applyUserPresetDefaults overrides the default values of the
// shared swarm flags (--mode, --personas, --confidence-threshold)
// with the yaml-set values from the user preset. Per-invocation
// flag overrides still win — applyUserPresetDefaults runs at
// cobra-construction time, BEFORE any flag parsing, so it just
// sets DefValue/initial value. Cobra will overlay user-supplied
// CLI args on top.
//
// Without this, the flag defaults from bindCommonFlags ("quick",
// nil, 0.0) silently override the yaml's defaults — the v0.13
// pr-review-caught blocker.
func applyUserPresetDefaults(cmd *cobra.Command, flags *commonSwarmFlags, up *swarm.UserPreset) {
	if up == nil {
		return
	}
	if up.Mode != "" {
		flags.mode = up.Mode
		if f := cmd.Flags().Lookup("mode"); f != nil {
			f.DefValue = up.Mode
			_ = f.Value.Set(up.Mode)
		}
	}
	if len(up.Personas) > 0 {
		flags.personas = up.Personas
		if f := cmd.Flags().Lookup("personas"); f != nil {
			_ = f.Value.Set(strings.Join(up.Personas, ","))
			f.DefValue = strings.Join(up.Personas, ",")
		}
	}
	if up.ConfidenceFloor > 0 {
		flags.confidenceFloor = up.ConfidenceFloor
		// confidenceFloor isn't bound as a cobra flag in the shared
		// flag set (only some presets like reverse expose
		// --confidence-threshold). The flags struct value above is
		// what runSwarmPipeline reads, so setting flags.confidenceFloor
		// is sufficient.
	}
}
