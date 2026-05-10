// swarm_code_archaeology.go — `jutsu swarm code-archaeology` cobra
// wiring. v0.12. Explain WHY legacy code looks the way it does.
//
// Cobra layer fetches git log (best-effort per spec Decision #9 —
// failures degrade to empty-history mode + stderr warning, NOT
// hard error) + bakes into the prompt at command time via
// swarm.BuildCodeArchaeologyPrompt. --code goes through standard
// InputFiles pipeline.
package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/momentmaker/kaijutsu/cli/internal/swarm"
)

// MaxGitLogBytes caps the git-log content baked into the prompt.
// 50 KB independent of the 200 KB code cap (per spec Decision #9).
// A 100-commit log on a frequently-touched file can easily exceed
// 100 KB; cap forces narrowing via --git-log <since>.
const MaxGitLogBytes = 50 * 1024

// DefaultGitLogCommitLimit is the default fallback when --git-log
// is unset: cap at the most-recent N commits touching the path.
const DefaultGitLogCommitLimit = 100

func newSwarmCodeArchaeologyCmd() *cobra.Command {
	var (
		flags    commonSwarmFlags
		codePath string
		gitLog   string
	)
	cmd := &cobra.Command{
		Use:   "code-archaeology",
		Short: "Explain WHY legacy code looks the way it does. Multi-agent generates competing historical-context theories ranked by evidence + corroboration.",
		Long: `Audits legacy code for HISTORY: workaround for past bug, era-
specific pattern, abandoned migration, perf optimization (still
load-bearing or obsolete?), security mitigation (still relevant?),
or just accidental complexity no one cleaned up.

Multi-agent dispatch + git-log evidence: each reviewer generates
theories citing specific commits / comments / structural patterns.
Synthesizer marks theories as "corroborated" (2+ reviewers,
independent evidence), "single-source" (one reviewer), or
"contested" (reviewers point different directions).

Use BEFORE a refactor — flags load-bearing patterns the user MUST
verify before deleting. Use AFTER joining a project — surfaces the
"why is this weird" answer faster than reading commit history
manually.

Examples:
  jutsu swarm code-archaeology --code cli/internal/swarm/preset.go
  jutsu swarm code-archaeology --code src/auth --git-log 1y

Default git-log: last %d commits touching --code (or its
descendants). Override with --git-log <since> using duration
syntax (1d / 6mo / 2y) or any git-log-compatible date string.
Git-log fetch is best-effort: failures (not a git repo, gh not on
PATH, no commits) degrade gracefully to empty-history mode +
stderr warning.

Severity vocab: blocker | issue | minor | info — but here it
means "trust this theory", not "code-quality severity":
  blocker: load-bearing theory user MUST verify before refactoring
  issue:   strong evidence; likely accurate
  minor:   plausible; corroborate with another source
  info:    speculative; only useful if other reviewers agree`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			projectRoot, _ := os.Getwd()

			preset, err := swarm.LoadPresetWithSkillOverrides(projectRoot, "code-archaeology")
			if err != nil {
				return err
			}
			if flags.grantConsent {
				return runGrantConsent(cmd, projectRoot, preset)
			}
			if flags.replayKey != "" {
				return runReplay(ctx, cmd, projectRoot, "code-archaeology", flags.replayKey, flags.synthesizer, flags.perAgentBudget, flags.timeout, false)
			}
			if codePath == "" {
				return UsageError(errors.New("--code <path> is required"))
			}
			if _, err := os.Stat(codePath); err != nil {
				if os.IsNotExist(err) {
					return NotFoundError(fmt.Errorf("--code %s: %w", codePath, err))
				}
				return fmt.Errorf("--code %s: %w", codePath, err)
			}

			// Fetch git log (best-effort per Decision #9).
			historyBody := fetchGitLog(projectRoot, codePath, gitLog, cmd.ErrOrStderr())
			if len(historyBody) > MaxGitLogBytes {
				// `git log` default order is newest-first, so the
				// prefix [0:MaxGitLogBytes] preserves the most-recent
				// commits. Newest-bias is intentional: recent commit
				// messages are the most-cited evidence in modern code
				// (active maintenance, bugfixes, recent refactors).
				// Era-pattern theories rely on commit dates regardless
				// of which end we keep — date metadata is per-commit.
				// Users wanting deeper history pass --git-log <since>
				// to widen the window before truncation kicks in.
				historyBody = historyBody[:MaxGitLogBytes]
			}

			presetCopy := *preset
			presetCopy.DefaultPrompt = swarm.BuildCodeArchaeologyPrompt(historyBody)
			preset = &presetCopy

			ictx, err := swarm.ResolveInput(ctx, preset, swarm.InputOptions{
				Files: []string{codePath},
			})
			if err != nil {
				return err
			}
			return runSwarmPipeline(ctx, cmd, projectRoot, preset, ictx, flags)
		},
	}
	cmd.Flags().StringVar(&codePath, "code", "", "path to legacy code (file or directory) — required")
	cmd.Flags().StringVar(&gitLog, "git-log", "", "git-log filter: duration (1d/6mo/2y) or git-log date string (default: last 100 commits)")
	bindCommonFlags(cmd, &flags, false) // no --post-comment for code-archaeology
	cmd.Long = fmt.Sprintf(cmd.Long, DefaultGitLogCommitLimit)
	return cmd
}

// fetchGitLog runs `git log` against codePath. Best-effort: any
// failure returns empty string + a one-line stderr warning. Per
// spec Decision #9, NOT a hard error — theories degrade gracefully
// (less evidence to cite). When `--git-log <since>` is provided
// and looks like a duration (1d / 6mo / 2y / etc), passes as
// --since=<since>; otherwise interprets as a git revision range.
// When --git-log is empty, falls back to -n <DefaultGitLogCommitLimit>.
func fetchGitLog(projectRoot, codePath, since string, stderrW interface {
	Write(p []byte) (int, error)
}) string {
	args := []string{"log", "--format=fuller"}
	if since == "" {
		args = append(args, fmt.Sprintf("-%d", DefaultGitLogCommitLimit))
	} else {
		args = append(args, "--since="+since)
	}
	args = append(args, "--", codePath)

	cmd := exec.Command("git", args...)
	if projectRoot != "" {
		cmd.Dir = projectRoot
	}
	out, err := cmd.Output()
	if err != nil {
		// Best-effort: surface the warning, return empty.
		fmt.Fprintf(stderrW, "warning: code-archaeology git-log fetch failed: %v (proceeding with empty history; theories will degrade)\n", err)
		return ""
	}
	return string(out)
}

