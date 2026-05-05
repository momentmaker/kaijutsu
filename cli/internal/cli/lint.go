package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/momentmaker/kaijutsu/cli/internal/lint"
	"github.com/momentmaker/kaijutsu/cli/internal/skill"
	"github.com/spf13/cobra"
)

func newLintCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "lint [path]",
		Short: "Validate one or more skill directories against the kaijutsu schema",
		Long: `Lint validates a skill directory.

If <path> contains a skill.yaml directly, that single skill is checked.
Otherwise <path> is treated as a parent directory: every immediate child
that contains a skill.yaml is linted in turn.

Without arguments, lint walks ./skills/core/ and ./skills/community/.

Exits with code 1 if any error-severity issue is reported.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			roots, err := resolveLintRoots(args)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			anyError := false
			anyResult := false
			skillsByName := map[string]*skill.Skill{}
			for _, root := range roots {
				results, err := lintTree(root)
				if err != nil {
					return err
				}
				for _, r := range results {
					anyResult = true
					if r.HasErrors() {
						anyError = true
					}
					printResult(out, r)
					if sk, lerr := skill.Load(filepath.Join(r.SkillDir, "skill.yaml")); lerr == nil {
						skillsByName[sk.Name] = sk
					}
				}
			}
			if !anyResult {
				fmt.Fprintln(out, "no skills found")
				return nil
			}
			if conflicts := lint.CheckTriggerConflicts(skillsByName); len(conflicts) > 0 {
				fmt.Fprintf(out, "\ntrigger conflicts:\n")
				for _, c := range conflicts {
					fmt.Fprintf(out, "  warn   %q overlaps across: %s\n", c.Phrase, strings.Join(c.Skills, ", "))
				}
			}
			if anyError {
				return fmt.Errorf("lint failed")
			}
			return nil
		},
	}
}

func resolveLintRoots(args []string) ([]string, error) {
	if len(args) == 1 {
		return []string{args[0]}, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	var roots []string
	for _, sub := range []string{"skills/core", "skills/community"} {
		path := filepath.Join(cwd, sub)
		if _, err := os.Stat(path); err == nil {
			roots = append(roots, path)
		}
	}
	if len(roots) == 0 {
		return nil, fmt.Errorf("no skills/core or skills/community in current directory; pass a path argument")
	}
	return roots, nil
}

// lintTree returns lint results for a path. If path contains skill.yaml
// directly, it returns a single result. Otherwise it expects path to be a
// parent directory containing skill subdirectories.
func lintTree(path string) ([]*lint.Result, error) {
	if _, err := os.Stat(filepath.Join(path, "skill.yaml")); err == nil {
		r, err := lint.Lint(path)
		if err != nil {
			return nil, err
		}
		return []*lint.Result{r}, nil
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	var out []*lint.Result
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		child := filepath.Join(path, e.Name())
		if _, err := os.Stat(filepath.Join(child, "skill.yaml")); err != nil {
			continue
		}
		r, err := lint.Lint(child)
		if err != nil {
			out = append(out, &lint.Result{
				SkillDir: child,
				Issues:   []lint.Issue{{Severity: lint.SeverityError, Path: child, Message: err.Error()}},
			})
			continue
		}
		out = append(out, r)
	}
	return out, nil
}

func printResult(out io.Writer, r *lint.Result) {
	if len(r.Issues) == 0 {
		fmt.Fprintf(out, "ok    %s\n", r.SkillDir)
		return
	}
	fmt.Fprintf(out, "fail  %s\n", r.SkillDir)
	for _, i := range r.Issues {
		marker := "  error  "
		if i.Severity == lint.SeverityWarning {
			marker = "  warn   "
		}
		fmt.Fprintf(out, "%s%s: %s\n", marker, i.Path, i.Message)
	}
}
