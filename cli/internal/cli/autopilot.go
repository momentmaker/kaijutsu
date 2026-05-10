// autopilot.go — v0.11.0 — `jutsu autopilot` cobra subcommand
// group. Owns the non-skill CLI surface for the autopilot v2
// pipeline: init (write .kaijutsu/autopilot.yaml), status (read
// state file), abort (clean state + worktree), resume (continue
// from last completed phase), run (non-interactive entry point).
//
// The interactive entry point remains the `/autopilot` slash command
// invoked inside an agent CLI session. This cobra group is for
// scripted / automated invocations, status checks during a run, and
// recovery operations.
//
// Per spec: docs/specs/2026-05-07-v0.11.0-autopilot.md (Decision #11).
package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// MaxAutopilotCostUSD is the hard ceiling on total spend in a
// single autopilot run. Lives in compiled Go code (not in
// .kaijutsu/autopilot.yaml) so that a malicious PR setting
// `cost.max_total_usd: 10000` cannot raise the ceiling. Users who
// genuinely need to exceed this set the per-shell env var
// KAIJUTSU_AUTOPILOT_HARD_CAP_OVERRIDE; that value is read once
// at autopilot start, never from a config file.
const MaxAutopilotCostUSD = 100.0

// AutopilotEnvHardCapOverride names the env var that raises the
// hard ceiling per-shell. Documented in skill SKILL.md +
// references/cost-model.md.
const AutopilotEnvHardCapOverride = "KAIJUTSU_AUTOPILOT_HARD_CAP_OVERRIDE"

// AutopilotEnvTestMode names the env var that switches autopilot
// run-mode into "write planned PR JSON to .kaijutsu/autopilot-pr.json
// instead of invoking gh". Used by CI to validate the pipeline shape
// without making real GitHub API calls.
const AutopilotEnvTestMode = "KAIJUTSU_AUTOPILOT_TEST_MODE"

// AutopilotConfigPath returns the per-project config path. Optional
// — autopilot runs with baked-in defaults if the file is absent.
func AutopilotConfigPath(projectRoot string) string {
	return filepath.Join(projectRoot, ".kaijutsu", "autopilot.yaml")
}

// AutopilotStatePath returns the per-project state file path. The
// state file persists across phases so resume + status work after a
// session interruption.
func AutopilotStatePath(projectRoot string) string {
	return filepath.Join(projectRoot, ".kaijutsu", "autopilot-state.md")
}

// AutopilotPlannedPRPath returns the test-mode planned-PR JSON path.
// Written by `jutsu autopilot run` when AutopilotEnvTestMode is set
// instead of invoking `gh pr create`.
func AutopilotPlannedPRPath(projectRoot string) string {
	return filepath.Join(projectRoot, ".kaijutsu", "autopilot-pr.json")
}

// resolveHardCap returns the effective hard ceiling in USD. Reads
// AutopilotEnvHardCapOverride once at call time; falls back to
// MaxAutopilotCostUSD when unset / unparseable.
func resolveHardCap() float64 {
	v := os.Getenv(AutopilotEnvHardCapOverride)
	if v == "" {
		return MaxAutopilotCostUSD
	}
	parsed, err := strconv.ParseFloat(v, 64)
	if err != nil || parsed <= 0 {
		return MaxAutopilotCostUSD
	}
	return parsed
}

// defaultAutopilotConfigYAML is the baked-in default config that
// `jutsu autopilot init` writes. Mirrors the spec's Decision #6 +
// the SKILL.md persona-defaults reference.
const defaultAutopilotConfigYAML = `# .kaijutsu/autopilot.yaml — autopilot v2 per-project config
# Optional. autopilot runs with baked-in defaults when this file is absent.
# Spec: docs/specs/2026-05-07-v0.11.0-autopilot.md

gates:
  brainstorm: review        # skip | review (default review = pause for user approval)
  post_spec: skip           # skip | review — off by default; paranoid users enable
  post_plan: skip           # skip | review
  pr: github                # PR review IS gate 2 — always; not config-overridable

brainstorm:
  preset: dream
  personas: [brainstorm-creative-claude, architecture-purist-gemini, claim-auditor-claude]
  mode: full
  lenses: all                # 8-lens canonical cycle

spec_review:
  preset: doc-review
  personas: [architecture-purist-gemini, paranoid-security-claude, claim-auditor-claude]
  mode: quick

plan_review:
  preset: doc-review
  personas: [perf-purist-codex, claim-auditor-claude, cross-file-gemini]
  mode: quick

per_stage_polish:
  max_passes: 4

final_review:
  preset: pr-review
  personas: [paranoid-security-claude, architecture-purist-gemini, perf-purist-codex, claim-auditor-claude]
  mode: full
  strict: true

findings:
  auto_apply: false          # report-only; never mutate artifact silently
  escalate_threshold: blocker

cost:
  max_total_usd: 20.00       # soft cap; ceiling = $100 in skill code
  pause_at_pct: 50

orchestration:
  skill_suggest: false       # deferred to v0.12+
  drift_check: tag           # tag | block | off
`

func newAutopilotCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "autopilot",
		Short: "Intent-to-PR pipeline (kaijutsu skill `autopilot`)",
		Long: `Multi-phase pipeline that turns a natural-language intent into
a polished PR. Two gates: brainstorm approval + GitHub PR review.
Multi-agent adversarial review at every artifact stage (spec, plan,
final). Reverse-drift gate before PR opens. Cost-capped (default
$20 soft, $100 hard ceiling).

The interactive entry point is the /autopilot slash command inside
an agent CLI session. This cobra group exists for scripted use,
status checks, and recovery operations.

See: skills/core/autopilot/ + docs/specs/2026-05-07-v0.11.0-autopilot.md.`,
	}
	cmd.AddCommand(
		newAutopilotInitCmd(),
		newAutopilotStatusCmd(),
		newAutopilotAbortCmd(),
		newAutopilotResumeCmd(),
		newAutopilotRunCmd(),
	)
	return cmd
}

// newAutopilotInitCmd writes the baked-in default config to
// .kaijutsu/autopilot.yaml. Refuses to overwrite without --force.
func newAutopilotInitCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Write .kaijutsu/autopilot.yaml from baked-in defaults",
		Long: `Writes a per-project autopilot config. Refuses to overwrite an
existing file unless --force is passed.

The config is OPTIONAL — autopilot runs with baked-in defaults when
the file is absent.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := projectRoot()
			if err != nil {
				return fmt.Errorf("locate project root: %w", err)
			}
			path := AutopilotConfigPath(root)
			if _, err := os.Stat(path); err == nil && !force {
				return UsageError(fmt.Errorf("%s already exists; pass --force to overwrite", path))
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return fmt.Errorf("create config dir: %w", err)
			}
			if err := os.WriteFile(path, []byte(defaultAutopilotConfigYAML), 0o644); err != nil {
				return fmt.Errorf("write %s: %w", path, err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "wrote %s\n", path)
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing autopilot.yaml")
	return cmd
}

// newAutopilotStatusCmd reads the state file and prints current
// phase + paths. Reports "no autopilot run in progress" when state
// file is absent.
func newAutopilotStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show current autopilot run phase + paths",
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := projectRoot()
			if err != nil {
				return fmt.Errorf("locate project root: %w", err)
			}
			path := AutopilotStatePath(root)
			body, err := os.ReadFile(path)
			if os.IsNotExist(err) {
				fmt.Fprintln(cmd.OutOrStdout(), "no autopilot run in progress")
				return nil
			}
			if err != nil {
				return fmt.Errorf("read %s: %w", path, err)
			}
			fmt.Fprint(cmd.OutOrStdout(), string(body))
			return nil
		},
	}
}

// newAutopilotAbortCmd cleans state file + worktree + branch (if any).
// Confirms via prompt unless --yes is passed.
func newAutopilotAbortCmd() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "abort",
		Short: "Abort the in-progress autopilot run; clean state + worktree",
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := projectRoot()
			if err != nil {
				return fmt.Errorf("locate project root: %w", err)
			}
			path := AutopilotStatePath(root)
			if _, err := os.Stat(path); os.IsNotExist(err) {
				fmt.Fprintln(cmd.OutOrStdout(), "no autopilot run in progress")
				return nil
			}
			if !yes {
				fmt.Fprint(cmd.OutOrStdout(), "Abort autopilot run? This deletes the state file. [y/N]: ")
				var resp string
				if _, scanErr := fmt.Fscanln(cmd.InOrStdin(), &resp); scanErr != nil {
					resp = ""
				}
				resp = strings.ToLower(strings.TrimSpace(resp))
				if resp != "y" && resp != "yes" {
					return errors.New("aborted (no confirmation)")
				}
			}
			if err := os.Remove(path); err != nil {
				return fmt.Errorf("remove %s: %w", path, err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "removed %s\n", path)
			return nil
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip the confirm prompt")
	return cmd
}

// newAutopilotResumeCmd reads the state file and continues from
// the last completed phase. The actual phase orchestration lives in
// the skill body (SKILL.md) — this CLI surface only validates state
// existence and prints the resume target so an agent can pick up.
func newAutopilotResumeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "resume",
		Short: "Continue the in-progress autopilot run from the last completed phase",
		Long: `Reads .kaijutsu/autopilot-state.md and prints the resume target
phase. The actual phase orchestration is performed by the autopilot
skill (running inside an agent CLI session).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := projectRoot()
			if err != nil {
				return fmt.Errorf("locate project root: %w", err)
			}
			path := AutopilotStatePath(root)
			body, err := os.ReadFile(path)
			if os.IsNotExist(err) {
				return NotFoundError(errors.New("no autopilot run in progress; use /autopilot to start one"))
			}
			if err != nil {
				return fmt.Errorf("read %s: %w", path, err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "resuming autopilot from state:")
			fmt.Fprint(cmd.OutOrStdout(), string(body))
			fmt.Fprintln(cmd.OutOrStdout(), "")
			fmt.Fprintln(cmd.OutOrStdout(), "(invoke /autopilot inside your agent CLI to continue the orchestration)")
			return nil
		},
	}
}

// newAutopilotRunCmd is the non-interactive entry point. Real
// orchestration lives in the skill body (running inside an agent
// CLI session via `/autopilot`); this CLI command exists for
// scripted use + CI test mode.
//
// In test mode (KAIJUTSU_AUTOPILOT_TEST_MODE=1), the command writes
// a planned-PR JSON to .kaijutsu/autopilot-pr.json instead of
// invoking gh. CI asserts against that file. Production runs
// (env var unset) print a one-line marker + tell the user to use
// /autopilot for the actual orchestration.
func newAutopilotRunCmd() *cobra.Command {
	var (
		yes     bool
		maxCost float64
	)
	cmd := &cobra.Command{
		Use:   "run \"<intent>\"",
		Short: "Non-interactive autopilot run (script-driven; for interactive use, invoke /autopilot inside an agent CLI)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			intent := args[0]
			if strings.TrimSpace(intent) == "" {
				return UsageError(errors.New("intent must be non-empty"))
			}
			ceiling := resolveHardCap()
			if maxCost < 0 {
				return UsageError(errors.New("--max-cost must be non-negative"))
			}
			if maxCost > ceiling {
				return UsageError(fmt.Errorf("--max-cost $%.2f exceeds hard ceiling $%.2f. To raise the ceiling, set %s=N in your shell environment (per-shell, never in repo)",
					maxCost, ceiling, AutopilotEnvHardCapOverride))
			}
			if !yes {
				return UsageError(errors.New("--yes required for non-interactive run; for interactive runs invoke /autopilot inside an agent CLI session"))
			}
			out := cmd.OutOrStdout()
			testMode := os.Getenv(AutopilotEnvTestMode) == "1"
			if testMode {
				return writeTestModePlannedPR(cmd, intent, maxCost, ceiling)
			}
			fmt.Fprintf(out, "autopilot run: intent=%q max_cost=$%.2f hard_ceiling=$%.2f test_mode=%v\n",
				intent, maxCost, ceiling, testMode)
			fmt.Fprintln(out, "(orchestration is performed by the autopilot skill running inside an agent CLI session)")
			return nil
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "non-interactive: skip Gate 1 brainstorm approval")
	cmd.Flags().Float64Var(&maxCost, "max-cost", 20.0, "soft cap on total run spend in USD (cannot exceed the hard ceiling)")
	return cmd
}

// writeTestModePlannedPR writes a planned-PR JSON describing what
// the autopilot run WOULD have submitted. CI tests assert against
// this file rather than against a real GitHub API call. The shape
// mirrors gh's pr-create flag set so future versions can add fields
// (labels for autopilot-drift, body for the synthesized review)
// without breaking consumers.
func writeTestModePlannedPR(cmd *cobra.Command, intent string, maxCost, ceiling float64) error {
	root, err := projectRoot()
	if err != nil {
		return fmt.Errorf("locate project root: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".kaijutsu"), 0o755); err != nil {
		return fmt.Errorf("create .kaijutsu/: %w", err)
	}
	plannedPR := struct {
		Intent       string   `json:"intent"`
		Title        string   `json:"title"`
		Body         string   `json:"body"`
		Branch       string   `json:"branch"`
		Labels       []string `json:"labels"`
		MaxCostUSD   float64  `json:"max_cost_usd"`
		HardCeiling  float64  `json:"hard_ceiling_usd"`
		TestMode     bool     `json:"test_mode"`
		DriftFindings int      `json:"drift_findings"`
	}{
		Intent:       intent,
		Title:        "autopilot: " + intent,
		Body:         "(test-mode placeholder: real autopilot run via /autopilot inside an agent CLI synthesizes the body)",
		Branch:       "feat/autopilot-test-mode",
		Labels:       []string{},
		MaxCostUSD:   maxCost,
		HardCeiling:  ceiling,
		TestMode:     true,
		DriftFindings: 0,
	}
	body, err := json.MarshalIndent(plannedPR, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal planned PR: %w", err)
	}
	path := AutopilotPlannedPRPath(root)
	if err := os.WriteFile(path, body, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "test-mode: planned PR written to %s\n", path)
	return nil
}
