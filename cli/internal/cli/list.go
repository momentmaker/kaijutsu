package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/momentmaker/kaijutsu/cli/internal/lint"
	"github.com/momentmaker/kaijutsu/cli/internal/manifest"
	"github.com/momentmaker/kaijutsu/cli/internal/paths"
	"github.com/momentmaker/kaijutsu/cli/internal/skill"
	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	var global bool
	var conflicts bool
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

			if conflicts {
				printInstalledConflicts(cmd, global, lf)
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&global, "global", "g", false, "list globally installed skills")
	cmd.Flags().BoolVar(&conflicts, "conflicts", false, "also report trigger-phrase overlaps across installed skills")
	return cmd
}

func printInstalledConflicts(cmd *cobra.Command, global bool, lf *manifest.Lockfile) {
	out := cmd.OutOrStdout()
	stderr := cmd.ErrOrStderr()
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "warning: cwd lookup failed: %v\n", err)
		return
	}
	installRoot := cwd
	if global {
		home, herr := paths.HomeDir()
		if herr != nil {
			fmt.Fprintf(stderr, "warning: home dir lookup failed: %v\n", herr)
			return
		}
		installRoot = home
	}
	skillsByName := map[string]*skill.Skill{}
	for name := range lf.Skills {
		yamlPath := findSkillYaml(installRoot, name, lf.Agents)
		if yamlPath == "" {
			continue
		}
		sk, lerr := skill.Load(yamlPath)
		if lerr != nil {
			continue
		}
		skillsByName[name] = sk
	}
	conflicts := lint.CheckTriggerConflicts(skillsByName)
	if len(conflicts) == 0 {
		fmt.Fprintln(out, "\nno trigger conflicts")
		return
	}
	fmt.Fprintln(out, "\ntrigger conflicts:")
	for _, c := range conflicts {
		fmt.Fprintf(out, "  %q overlaps across: %s\n", c.Phrase, strings.Join(c.Skills, ", "))
	}
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
