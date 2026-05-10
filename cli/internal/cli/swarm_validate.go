// swarm_validate.go — `jutsu swarm validate [path]` cobra wiring.
// v0.13.0 Preset SDK. Validates a user swarm.yaml file against the
// embedded JSON schema + the field-contract validators in
// swarm.LoadUserPresets.
//
// Reports issues with file:line citations + exits zero clean,
// non-zero on issues. Run BEFORE invoking a user preset to surface
// errors early.
package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/momentmaker/kaijutsu/cli/internal/swarm"
)

func newSwarmValidateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate [path]",
		Short: "Validate a swarm.yaml file against the v0.13 Preset SDK schema + name-collision rules.",
		Long: `Validates a user swarm.yaml file. Default path is .kaijutsu/swarm.yaml
in the current working directory; pass a path argument to override.

Checks:
  - YAML parses cleanly
  - Each entry validates against the embedded JSON schema (field
    types, required fields, enum values, %s slot counts)
  - No entry name collides with a built-in preset (pr-review,
    doc-review, brainstorm, dream, refactor-plan, security-audit,
    reverse, test-gap, bug-repro, code-archaeology)
  - No intra-file duplicate names (whole file rejected if so)

Exits zero on a clean file; non-zero with field-citation errors
otherwise.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := filepath.Join(".kaijutsu", "swarm.yaml")
			if len(args) == 1 {
				path = args[0]
			}
			out := cmd.OutOrStdout()
			stderr := cmd.ErrOrStderr()

			if _, err := os.Stat(path); os.IsNotExist(err) {
				return NotFoundError(fmt.Errorf("file not found: %s. Default path is .kaijutsu/swarm.yaml; pass a path argument or run `jutsu swarm validate <path>` if your file lives elsewhere", path))
			} else if err != nil {
				return fmt.Errorf("stat %s: %w", path, err)
			}

			// Load just this one file (not the project + home pair) so
			// validation errors point at the user-supplied path. Pass
			// it as the project root and a non-existent home dir so
			// LoadUserPresets's home path resolves to an absent file.
			absPath, err := filepath.Abs(path)
			if err != nil {
				return fmt.Errorf("absolute path %s: %w", path, err)
			}
			// LoadUserPresets reads <projectRoot>/.kaijutsu/swarm.yaml.
			// The user-supplied path may already include .kaijutsu/swarm.yaml
			// (default case) or may point elsewhere. We resolve via:
			//  - If path ends in .kaijutsu/swarm.yaml, projectRoot = parent of .kaijutsu/.
			//  - Else, copy to a tmp project root + load from there.
			projectRoot, useTmp, err := projectRootForValidate(absPath)
			if err != nil {
				return err
			}
			if useTmp {
				defer func() { _ = os.RemoveAll(projectRoot) }()
			}
			fakeHome := filepath.Join(projectRoot, "no-such-home-dir")
			presets, warnings, loadErr := swarm.LoadUserPresets(projectRoot, fakeHome)
			if loadErr != nil {
				return fmt.Errorf("validate %s: %w", path, loadErr)
			}

			if len(warnings) == 0 && len(presets) > 0 {
				fmt.Fprintf(out, "OK: %s — %d valid preset(s)\n", path, len(presets))
				for name := range presets {
					fmt.Fprintf(out, "  - %s\n", name)
				}
				return nil
			}
			if len(warnings) == 0 && len(presets) == 0 {
				fmt.Fprintf(out, "OK: %s contains zero presets (empty list)\n", path)
				return nil
			}
			fmt.Fprintf(stderr, "validate %s: %d issue(s) found:\n", path, len(warnings))
			for _, w := range warnings {
				fmt.Fprintf(stderr, "  - %s\n", w.Error())
			}
			if len(presets) > 0 {
				fmt.Fprintf(stderr, "%d valid preset(s) would still register:\n", len(presets))
				for name := range presets {
					fmt.Fprintf(stderr, "  - %s\n", name)
				}
			}
			return errors.New("swarm.yaml has validation issues; fix the entries above")
		},
	}
	return cmd
}

// projectRootForValidate resolves the projectRoot to pass to
// LoadUserPresets given a user-supplied --validate path. Two cases:
//
//	A. path ends in .kaijutsu/swarm.yaml (default invocation):
//	   projectRoot = grandparent (LoadUserPresets reads
//	   <root>/.kaijutsu/swarm.yaml verbatim).
//
//	B. arbitrary path (user passed explicit path):
//	   stage the file into a tmp dir's .kaijutsu/swarm.yaml so
//	   LoadUserPresets can read it without modification. Return
//	   useTmp=true so caller cleans up.
func projectRootForValidate(absPath string) (root string, useTmp bool, err error) {
	dir := filepath.Dir(absPath)
	if filepath.Base(dir) == ".kaijutsu" && filepath.Base(absPath) == "swarm.yaml" {
		return filepath.Dir(dir), false, nil
	}
	tmp, err := os.MkdirTemp("", "swarm-validate-")
	if err != nil {
		return "", false, fmt.Errorf("staging dir: %w", err)
	}
	stagedDir := filepath.Join(tmp, ".kaijutsu")
	if err := os.MkdirAll(stagedDir, 0o755); err != nil {
		_ = os.RemoveAll(tmp)
		return "", false, fmt.Errorf("create staging .kaijutsu: %w", err)
	}
	body, err := os.ReadFile(absPath)
	if err != nil {
		_ = os.RemoveAll(tmp)
		return "", false, fmt.Errorf("read %s: %w", absPath, err)
	}
	if err := os.WriteFile(filepath.Join(stagedDir, "swarm.yaml"), body, 0o644); err != nil {
		_ = os.RemoveAll(tmp)
		return "", false, fmt.Errorf("stage swarm.yaml: %w", err)
	}
	return tmp, true, nil
}
