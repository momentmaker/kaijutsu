// swarm_bug_repro.go — `jutsu swarm bug-repro` cobra wiring. v0.12.
// Vague bug description → ranked repro hypotheses + minimal-repro
// steps for the top hypothesis.
//
// Cobra layer reads optional --files content + bakes into the
// prompt at command time via swarm.BuildBugReproPrompt (mirrors
// reverse's --spec / test-gap's --tests pattern). Bug description
// goes through the standard InputPrompt pipeline.
package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/momentmaker/kaijutsu/cli/internal/swarm"
)

// MaxBugReproFilesBytes caps the --files content baked into the
// prompt. 200 KB matches the spec's input cap (Decision #8). Bug
// reports rarely need full-codebase context; a single function or
// module is the right grain.
const MaxBugReproFilesBytes = 200 * 1024

func newSwarmBugReproCmd() *cobra.Command {
	var (
		flags     commonSwarmFlags
		filePaths []string
	)
	cmd := &cobra.Command{
		Use:   "bug-repro <bug description>",
		Short: "Vague bug → ranked repro hypotheses across categories (state/race/env/input/version) + minimal-repro steps.",
		Long: `Takes a free-form bug description (positional arg) and dispatches
the multi-agent swarm to generate ranked repro hypotheses. Each
reviewer imagines distinct failure-mode categories; the synthesizer
clusters by category + ranks by confidence + cross-reviewer
corroboration.

Optional --files <paths> bakes code context into the prompt so
hypotheses can cite specific functions / lines (paths are
comma-separated; each file content prepended).

Differs from brainstorm: brainstorm generates SOLUTIONS to a
problem; bug-repro generates CAUSES of a known bug. Differs from
pr-review: pr-review hunts BUGS in a diff; bug-repro reasons from a
bug REPORT (no diff context, just symptoms).

Examples:
  jutsu swarm bug-repro "intermittent login failure on mobile"
  jutsu swarm bug-repro "stale cache after deploy" --files src/cache,src/deploy

Severity vocab matches pr-review (blocker | issue | minor | info).
Default confidence threshold = 0.30 (matches reverse + test-gap) —
hypothesis-generation surfaces speculative findings; lower threshold
admits them; the synthesizer's ranking + your filter via the
confidence column do the gatekeeping.`,
		Args: cobra.ArbitraryArgs, // 0 args ok with --replay/--grant-consent
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			projectRoot, _ := os.Getwd()

			preset, err := swarm.LoadPresetWithSkillOverrides(projectRoot, "bug-repro")
			if err != nil {
				return err
			}
			if flags.grantConsent {
				return runGrantConsent(cmd, projectRoot, preset)
			}
			if flags.replayKey != "" {
				return runReplay(ctx, cmd, projectRoot, "bug-repro", flags.replayKey, flags.synthesizer, flags.perAgentBudget, flags.timeout, false)
			}
			if len(args) == 0 {
				return UsageError(errors.New("bug-repro requires a bug description argument (or --replay <key>). Quote multi-word descriptions."))
			}
			bug := strings.Join(args, " ")
			if strings.TrimSpace(bug) == "" {
				return UsageError(errors.New("bug description must be non-empty after canonicalization"))
			}

			// Read --files content if provided + bake into prompt.
			filesBody, err := readBugReproFiles(filePaths)
			if err != nil {
				return err
			}
			if len(filesBody) > MaxBugReproFilesBytes {
				return UsageError(fmt.Errorf("--files content is %d bytes — exceeds %d-byte cap. Narrow to a single function/module or fewer paths", len(filesBody), MaxBugReproFilesBytes))
			}
			presetCopy := *preset
			presetCopy.DefaultPrompt = swarm.BuildBugReproPrompt(filesBody)
			preset = &presetCopy

			ictx, err := swarm.ResolveInput(ctx, preset, swarm.InputOptions{
				Prompt: bug,
			})
			if err != nil {
				return err
			}
			return runSwarmPipeline(ctx, cmd, projectRoot, preset, ictx, flags)
		},
	}
	cmd.Flags().StringSliceVar(&filePaths, "files", nil, "comma-separated code-context paths (optional). Each file content prepended to the bug description so hypotheses can cite specific functions/lines.")
	bindCommonFlags(cmd, &flags, false) // no --post-comment for bug-repro
	return cmd
}

// readBugReproFiles reads each path in --files. For directories,
// walks recursively + concatenates with `// === <relative-path> ===`
// headers. Skips hidden dirs. Returns "" when filePaths is empty
// (no --files provided is fine — preset works without code context).
func readBugReproFiles(paths []string) (string, error) {
	if len(paths) == 0 {
		return "", nil
	}
	var b strings.Builder
	for _, root := range paths {
		info, err := os.Stat(root)
		if err != nil {
			return "", fmt.Errorf("--files %s: %w", root, err)
		}
		if !info.IsDir() {
			body, err := os.ReadFile(root)
			if err != nil {
				return "", fmt.Errorf("--files %s: %w", root, err)
			}
			fmt.Fprintf(&b, "// === %s ===\n", root)
			b.Write(body)
			b.WriteString("\n")
			continue
		}
		err = filepath.WalkDir(root, func(p string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				if strings.HasPrefix(d.Name(), ".") && d.Name() != "." {
					return filepath.SkipDir
				}
				return nil
			}
			if !d.Type().IsRegular() {
				return nil
			}
			body, err := os.ReadFile(p)
			if err != nil {
				return fmt.Errorf("read %s: %w", p, err)
			}
			rel, _ := filepath.Rel(root, p)
			fmt.Fprintf(&b, "// === %s ===\n", filepath.Join(filepath.Base(root), rel))
			b.Write(body)
			b.WriteString("\n")
			// Early-exit if we've blown past the cap.
			if b.Len() > MaxBugReproFilesBytes {
				return filepath.SkipAll
			}
			return nil
		})
		if err != nil {
			return "", err
		}
	}
	return b.String(), nil
}
