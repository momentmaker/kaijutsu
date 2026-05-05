package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/momentmaker/kaijutsu/cli/internal/manifest"
	"github.com/momentmaker/kaijutsu/cli/internal/paths"
	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	var global bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List installed skills",
		RunE: func(cmd *cobra.Command, args []string) error {
			lockPath, err := lockfilePathFor(global)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()

			lf, err := manifest.LoadLockfile(lockPath)
			if err != nil {
				if os.IsNotExist(err) {
					fmt.Fprintln(out, "no skills installed")
					return nil
				}
				return err
			}
			if len(lf.Skills) == 0 {
				fmt.Fprintln(out, "no skills installed")
				return nil
			}

			names := make([]string, 0, len(lf.Skills))
			for n := range lf.Skills {
				names = append(names, n)
			}
			sort.Strings(names)
			fmt.Fprintf(out, "%-30s  %-10s  %-10s  %s\n", "NAME", "VERSION", "TAG", "SOURCE")
			for _, n := range names {
				e := lf.Skills[n]
				v := "(none)"
				if e.Version != nil {
					v = *e.Version
				}
				tag := e.Tag
				if tag == "" {
					tag = "(none)"
				}
				fmt.Fprintf(out, "%-30s  %-10s  %-10s  %s\n", n, v, tag, e.Source)
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&global, "global", "g", false, "list globally installed skills")
	return cmd
}

func lockfilePathFor(global bool) (string, error) {
	if global {
		home, err := paths.HomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, paths.GlobalConfigDirName, paths.GlobalLockfileFile), nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(cwd, paths.LockfileFile), nil
}
