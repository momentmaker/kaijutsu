package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/momentmaker/kaijutsu/cli/internal/install"
	"github.com/momentmaker/kaijutsu/cli/internal/manifest"
	"github.com/spf13/cobra"
)

func newRemoveCmd() *cobra.Command {
	var global bool
	cmd := &cobra.Command{
		Use:   "remove <skill>",
		Short: "Remove an installed skill",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			skillName := args[0]

			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			installRoot, manifestPath, lockPath, err := resolveTargets(global, cwd)
			if err != nil {
				return err
			}

			lf, err := manifest.LoadLockfile(lockPath)
			if err != nil {
				return err
			}
			if _, ok := lf.Skills[skillName]; !ok {
				return errors.New("skill is not installed")
			}

			if err := install.Remove(installRoot, skillName, lf.Agents); err != nil {
				return err
			}

			delete(lf.Skills, skillName)
			if err := lf.Save(lockPath); err != nil {
				return err
			}

			if _, err := os.Stat(manifestPath); err == nil {
				if m, mErr := manifest.Load(manifestPath); mErr == nil {
					delete(m.Dependencies, skillName)
					_ = m.Save(manifestPath)
				}
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Removed %s\n", skillName)
			return nil
		},
	}
	cmd.Flags().BoolVarP(&global, "global", "g", false, "remove from global installation")
	return cmd
}
