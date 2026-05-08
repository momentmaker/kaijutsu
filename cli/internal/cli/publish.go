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
	var dryRun bool
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

			if dryRun {
				printDryRunPlan(out, abs, sk, auto, workdir)
				return nil
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
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "lint + print the planned PR title/body + exit 0 without invoking gh, git, or any network call")
	return cmd
}

func printDryRunPlan(out io.Writer, abs string, sk *skill.Skill, auto bool, workdir string) {
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "[dry-run] skill is valid + would publish with this plan:")
	fmt.Fprintln(out, "")
	fmt.Fprintf(out, "[dry-run]   skill:   %s@%s\n", sk.Name, sk.Version)
	fmt.Fprintf(out, "[dry-run]   source:  %s\n", abs)
	fmt.Fprintf(out, "[dry-run]   target:  skills/community/%s/ in %s\n", sk.Name, upstreamRepo)
	fmt.Fprintf(out, "[dry-run]   branch:  add-%s\n", sk.Name)
	fmt.Fprintf(out, "[dry-run]   commit:  feat(skills): add %s\n", sk.Name)
	fmt.Fprintf(out, "[dry-run]   pr:      feat(skills): add %s community skill\n", sk.Name)
	if auto {
		if workdir == "" {
			fmt.Fprintln(out, "[dry-run]   workdir: (would auto-allocate temp dir)")
		} else {
			fmt.Fprintf(out, "[dry-run]   workdir: %s\n", workdir)
		}
		fmt.Fprintln(out, "[dry-run]   gh + git would NOT be invoked under --dry-run")
	}
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

	var tmpRoot string
	if workdir == "" {
		tmp, err := os.MkdirTemp("", "kaijutsu-fork-")
		if err != nil {
			return err
		}
		tmpRoot = tmp
		workdir = filepath.Join(tmp, "kaijutsu")
	} else {
		// User-supplied workdir: must be either non-existent or a
		// git checkout. A non-empty non-git dir would make `gh repo
		// clone` fail with a confusing error.
		if entries, statErr := os.ReadDir(workdir); statErr == nil && len(entries) > 0 {
			if _, gitErr := os.Stat(filepath.Join(workdir, ".git")); gitErr != nil {
				return fmt.Errorf("--workdir %s is non-empty but not a git checkout; pick an empty path or a previous fork clone", workdir)
			}
		}
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
	fmt.Fprintf(out, "  branch:  %s (force-push: existing branch on fork will be overwritten)\n", branch)
	fmt.Fprintf(out, "  copy:    %s -> skills/community/%s/\n", skillDir, sk.Name)
	fmt.Fprintf(out, "  pr:      %s -- %s\n", upstreamRepo, prTitle)
	// Fail-fast on a name collision against upstream before doing
	// the slow clone. gh api 404s on missing paths.
	exists, hasErr := upstreamHas(upstreamRepo, "skills/community/"+sk.Name)
	if hasErr != nil {
		return fmt.Errorf("collision pre-check failed: %w", hasErr)
	}
	if exists {
		return fmt.Errorf("skills/community/%s already exists in %s; pick a different name or update via a manual PR against the existing dir", sk.Name, upstreamRepo)
	}

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
	// Catastrophic-failure guard: if --workdir points at the upstream
	// source checkout (origin = upstreamRepo) instead of the user's
	// fork, every subsequent op (reset --hard, force-push) would
	// destroy upstream history. Verify origin matches the fork.
	if err := verifyWorkdirOrigin(workdir, forkRepo); err != nil {
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
	// Drop untracked files from a failed previous run — `git reset
	// --hard` only resets tracked files. Without this, a stale
	// skills/community/<sk.Name> from an earlier aborted run would
	// trip the existence check below.
	if err := run(stderr, workdir, "git", "clean", "-fd"); err != nil {
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
	// gh pr create may print warnings on stdout before the URL
	// (e.g. "a pull request already exists: <url>"). Take the last
	// non-empty line as the canonical URL.
	prURL = lastNonEmptyLine(prURL)
	fmt.Fprintf(out, "\nPR opened: %s\n", prURL)
	// Clean up the auto-allocated temp dir on success only — leak
	// on failure so the user can inspect the partial state.
	if tmpRoot != "" {
		_ = os.RemoveAll(tmpRoot)
	}
	return nil
}

func lastNonEmptyLine(s string) string {
	lines := strings.Split(s, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		l := strings.TrimSpace(lines[i])
		if l != "" {
			return l
		}
	}
	return ""
}

// upstreamHas reports whether <repo> already contains <path>. Returns
// (true, nil) when the path exists, (false, nil) on a clean 404, and
// (false, err) on any other failure (auth, rate-limit, repo gone). The
// caller MUST treat err != nil as a hard failure rather than "path
// missing", to avoid silently skipping the collision check.
func upstreamHas(repo, path string) (bool, error) {
	c := exec.Command("gh", "api", fmt.Sprintf("repos/%s/contents/%s", repo, path), "--silent")
	var stderrBuf strings.Builder
	c.Stderr = &stderrBuf
	if err := c.Run(); err != nil {
		stderr := stderrBuf.String()
		if strings.Contains(stderr, "HTTP 404") || strings.Contains(stderr, "Not Found") {
			return false, nil
		}
		return false, fmt.Errorf("gh api %s: %s", repo, strings.TrimSpace(stderr))
	}
	return true, nil
}

// verifyForkParent asserts that <user>/kaijutsu is actually a fork of
// upstreamRepo. Without this check, an unrelated repo with the same
// name (e.g. a personal project the user happened to call kaijutsu)
// would be force-pushed to and PRed against, producing a confused
// cross-repo PR.
func verifyForkParent(forkRepo string) error {
	parent, err := capture("gh", "api", "repos/"+forkRepo, "--jq", ".parent.full_name")
	if err != nil {
		return fmt.Errorf("verify fork parent for %s: %w", forkRepo, err)
	}
	parent = strings.TrimSpace(parent)
	if parent == "" {
		return fmt.Errorf("%s exists but has no parent — it's not a fork. Rename or delete it before re-running --auto", forkRepo)
	}
	if parent != upstreamRepo {
		return fmt.Errorf("%s is a fork of %s, not %s. Rename your local fork or pick a different workdir", forkRepo, parent, upstreamRepo)
	}
	return nil
}

func ensureFork(stderr io.Writer, user string) error {
	forkRepo := user + "/kaijutsu"
	if err := exec.Command("gh", "repo", "view", forkRepo).Run(); err == nil {
		return verifyForkParent(forkRepo)
	}
	return run(stderr, "", "gh", "repo", "fork", upstreamRepo, "--clone=false")
}

// verifyWorkdirOrigin asserts that <workdir>'s `origin` remote points
// at <forkRepo>. If a user passes --workdir pointing at the upstream
// source checkout (or any unrelated kaijutsu clone), this catches it
// before the destructive `git reset --hard` and force-push.
func verifyWorkdirOrigin(workdir, forkRepo string) error {
	c := exec.Command("git", "remote", "get-url", "origin")
	c.Dir = workdir
	out, err := c.Output()
	if err != nil {
		return fmt.Errorf("verify workdir origin: %w", err)
	}
	url := strings.TrimSpace(string(out))
	// Match either ssh (git@github.com:owner/repo.git) or https
	// (https://github.com/owner/repo[.git]).
	want := []string{
		"git@github.com:" + forkRepo + ".git",
		"git@github.com:" + forkRepo,
		"https://github.com/" + forkRepo + ".git",
		"https://github.com/" + forkRepo,
	}
	for _, w := range want {
		if url == w {
			return nil
		}
	}
	return fmt.Errorf("--workdir %s has origin %q but expected the fork at %s. Refusing to proceed — would force-push to the wrong repo. Pick a different --workdir, or remove the existing one to let --auto re-clone the fork",
		workdir, url, forkRepo)
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
