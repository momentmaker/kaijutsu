package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/momentmaker/kaijutsu/cli/internal/install"
	"github.com/momentmaker/kaijutsu/cli/internal/lint"
	"github.com/momentmaker/kaijutsu/cli/internal/skill"
	"github.com/spf13/cobra"
)

const upstreamRepo = "momentmaker/kaijutsu"

func newPublishCmd() *cobra.Command {
	var auto bool
	var yes bool
	var workdir string
	cmd := &cobra.Command{
		Use:   "publish [path]",
		Short: "Lint a skill and (optionally) fork+PR it against the kaijutsu registry",
		Long: `Validates the skill at <path> (or cwd if no arg) and prints the
manual steps to open a pull request against ` + upstreamRepo + `.

With --auto, shells out to gh to fork the upstream repo (if needed),
clones the fork into a working directory, copies the skill into
skills/community/<name>/, branches+commits+pushes, and opens the PR.

Requires gh on PATH + an authenticated session (run gh auth login first).`,
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

			if !auto {
				printPublishInstructions(out, abs, sk)
				return nil
			}

			return runAutoPublish(cmd, abs, sk, workdir, yes)
		},
	}
	cmd.Flags().BoolVar(&auto, "auto", false, "automate fork + clone + push + PR via gh")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "non-interactive: skip the pre-push confirmation prompt (--auto)")
	cmd.Flags().StringVar(&workdir, "workdir", "", "directory to clone the fork into (default: temp dir; reused if it already contains a clone)")
	return cmd
}

func printPublishInstructions(out io.Writer, abs string, sk *skill.Skill) {
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Skill is valid. To publish:")
	fmt.Fprintln(out, "")
	fmt.Fprintf(out, "  jutsu publish %s --auto\n", abs)
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Or run the steps manually:")
	fmt.Fprintf(out, "  1. gh repo fork %s --clone=false\n", upstreamRepo)
	fmt.Fprintf(out, "  2. git clone <your-fork-url> /tmp/kaijutsu-fork && cd /tmp/kaijutsu-fork\n")
	fmt.Fprintf(out, "  3. cp -r %q skills/community/%s/\n", abs, sk.Name)
	fmt.Fprintf(out, "  4. git checkout -b add-%s\n", sk.Name)
	fmt.Fprintf(out, "  5. git add skills/community/%s/ && git commit -m %q\n", sk.Name, "feat(skills): add "+sk.Name)
	fmt.Fprintf(out, "  6. git push origin add-%s\n", sk.Name)
	fmt.Fprintf(out, "  7. gh pr create --repo %s --title %q --body %q\n",
		upstreamRepo,
		"feat(skills): add "+sk.Name+" community skill",
		sk.Description)
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Alternative: if your skill lives in its own repo, append an entry to")
	fmt.Fprintln(out, "registry/index.json instead of copying the directory.")
}

func runAutoPublish(cmd *cobra.Command, skillDir string, sk *skill.Skill, workdir string, yes bool) error {
	out := cmd.OutOrStdout()
	stderr := cmd.ErrOrStderr()

	if _, err := exec.LookPath("gh"); err != nil {
		return errors.New("--auto requires the gh CLI on PATH; install from https://cli.github.com/ then run `gh auth login`")
	}
	if err := run(stderr, "", "gh", "auth", "status"); err != nil {
		return errors.New("--auto requires an authenticated gh session; run `gh auth login` first")
	}
	if _, err := exec.LookPath("git"); err != nil {
		return errors.New("--auto requires git on PATH")
	}

	user, err := capture("gh", "api", "user", "--jq", ".login")
	if err != nil {
		return fmt.Errorf("query gh user: %w", err)
	}
	user = strings.TrimSpace(user)
	if user == "" {
		return errors.New("gh returned empty user — run `gh auth login`")
	}
	forkRepo := user + "/kaijutsu"

	if workdir == "" {
		tmp, err := os.MkdirTemp("", "kaijutsu-fork-")
		if err != nil {
			return err
		}
		workdir = filepath.Join(tmp, "kaijutsu")
	}
	if err := os.MkdirAll(filepath.Dir(workdir), 0755); err != nil {
		return err
	}

	branch := "add-" + sk.Name
	commitMsg := "feat(skills): add " + sk.Name
	prTitle := "feat(skills): add " + sk.Name + " community skill"
	prBody := sk.Description

	fmt.Fprintf(out, "Plan:\n")
	fmt.Fprintf(out, "  fork:    %s -> %s\n", upstreamRepo, forkRepo)
	fmt.Fprintf(out, "  workdir: %s\n", workdir)
	fmt.Fprintf(out, "  branch:  %s\n", branch)
	fmt.Fprintf(out, "  copy:    %s -> skills/community/%s/\n", skillDir, sk.Name)
	fmt.Fprintf(out, "  pr:      %s -- %s\n", upstreamRepo, prTitle)
	if !yes {
		fmt.Fprint(out, "Proceed? [y/N]: ")
		r := bufio.NewReader(cmd.InOrStdin())
		line, rerr := r.ReadString('\n')
		if rerr != nil {
			return errors.New("aborted (no input)")
		}
		line = strings.TrimSpace(strings.ToLower(line))
		if line != "y" && line != "yes" {
			return errors.New("aborted")
		}
	}

	if err := ensureFork(stderr, user); err != nil {
		return err
	}
	if err := ensureClone(stderr, forkRepo, workdir); err != nil {
		return err
	}
	if err := run(stderr, workdir, "git", "fetch", "origin"); err != nil {
		return err
	}
	if err := run(stderr, workdir, "git", "checkout", "main"); err != nil {
		return err
	}
	if err := run(stderr, workdir, "git", "reset", "--hard", "origin/main"); err != nil {
		return err
	}

	dest := filepath.Join(workdir, "skills", "community", sk.Name)
	if _, err := os.Stat(dest); err == nil {
		return fmt.Errorf("skills/community/%s already exists in %s; pick a different name or update the existing skill manually", sk.Name, forkRepo)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}
	if err := install.CopyTree(skillDir, dest); err != nil {
		return fmt.Errorf("copy skill: %w", err)
	}

	if err := run(stderr, workdir, "git", "checkout", "-B", branch); err != nil {
		return err
	}
	if err := run(stderr, workdir, "git", "add", filepath.Join("skills", "community", sk.Name)); err != nil {
		return err
	}
	if err := run(stderr, workdir, "git", "commit", "-m", commitMsg); err != nil {
		return err
	}
	if err := run(stderr, workdir, "git", "push", "-u", "-f", "origin", branch); err != nil {
		return err
	}

	prURL, err := capture("gh", "pr", "create",
		"--repo", upstreamRepo,
		"--head", user+":"+branch,
		"--base", "main",
		"--title", prTitle,
		"--body", prBody,
	)
	if err != nil {
		return fmt.Errorf("gh pr create: %w", err)
	}
	prURL = strings.TrimSpace(prURL)
	fmt.Fprintf(out, "\nPR opened: %s\n", prURL)
	return nil
}

func ensureFork(stderr io.Writer, user string) error {
	forkRepo := user + "/kaijutsu"
	if err := exec.Command("gh", "repo", "view", forkRepo).Run(); err == nil {
		return nil
	}
	return run(stderr, "", "gh", "repo", "fork", upstreamRepo, "--clone=false", "--default-branch-only")
}

func ensureClone(stderr io.Writer, forkRepo, workdir string) error {
	if _, err := os.Stat(filepath.Join(workdir, ".git")); err == nil {
		return nil
	}
	return run(stderr, "", "gh", "repo", "clone", forkRepo, workdir, "--", "--depth=1")
}

func run(stderr io.Writer, dir, name string, args ...string) error {
	c := exec.Command(name, args...)
	if dir != "" {
		c.Dir = dir
	}
	c.Stdout = stderr
	c.Stderr = stderr
	return c.Run()
}

func capture(name string, args ...string) (string, error) {
	c := exec.Command(name, args...)
	out, err := c.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("%s: %s", name, strings.TrimSpace(string(ee.Stderr)))
		}
		return "", err
	}
	return string(out), nil
}
