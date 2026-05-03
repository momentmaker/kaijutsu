package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/momentmaker/kaijutsu/cli/internal/lint"
	"github.com/momentmaker/kaijutsu/cli/internal/skill"
	"github.com/spf13/cobra"
)

func newPublishCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "publish [path]",
		Short: "Lint a skill and print the steps to open a PR against the kaijutsu registry",
		Long: `Validates the skill at <path> (or cwd if no arg) and prints the
manual steps to open a pull request against momentmaker/kaijutsu.

This v0 release does not automate the git/gh fork-and-PR dance — that
arrives in v0.1. For now, publish is a checklist: it lints, summarizes
the metadata, and prints the gh commands you should run.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "."
			if len(args) == 1 {
				path = args[0]
			}
			abs, err := filepath.Abs(path)
			if err != nil {
				return err
			}
			if _, err := os.Stat(filepath.Join(abs, "skill.yaml")); err != nil {
				return fmt.Errorf("no skill.yaml in %s", abs)
			}
			out := cmd.OutOrStdout()

			r, err := lint.Lint(abs)
			if err != nil {
				return err
			}
			printResult(out, r)
			if r.HasErrors() {
				return fmt.Errorf("lint failed; fix the errors above before publishing")
			}

			sk, err := skill.Load(filepath.Join(abs, "skill.yaml"))
			if err != nil {
				return err
			}

			fmt.Fprintln(out, "")
			fmt.Fprintln(out, "Skill is valid. To publish:")
			fmt.Fprintln(out, "")
			fmt.Fprintf(out, "  1. Fork momentmaker/kaijutsu on GitHub (or use an existing fork).\n")
			fmt.Fprintf(out, "  2. Clone it, then copy this skill into skills/community/%s/:\n", sk.Name)
			fmt.Fprintf(out, "       cp -r %q <fork>/skills/community/%s/\n", abs, sk.Name)
			fmt.Fprintf(out, "  3. Branch + commit + push:\n")
			fmt.Fprintf(out, "       git checkout -b add-%s\n", sk.Name)
			fmt.Fprintf(out, "       git add skills/community/%s/\n", sk.Name)
			fmt.Fprintf(out, "       git commit -m %q\n", "feat(skills): add "+sk.Name)
			fmt.Fprintf(out, "       git push origin add-%s\n", sk.Name)
			fmt.Fprintf(out, "  4. Open the PR:\n")
			fmt.Fprintf(out, "       gh pr create --base main --title %q --body %q\n",
				"feat(skills): add "+sk.Name+" community skill",
				sk.Description)
			fmt.Fprintln(out, "")
			fmt.Fprintln(out, "Alternative: if your skill lives in its own repo, append an entry to")
			fmt.Fprintln(out, "registry/index.json instead of copying the directory.")
			return nil
		},
	}
}
