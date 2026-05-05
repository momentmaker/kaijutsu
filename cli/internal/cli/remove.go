package cli

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/momentmaker/kaijutsu/cli/internal/hooks"
	"github.com/momentmaker/kaijutsu/cli/internal/install"
	"github.com/momentmaker/kaijutsu/cli/internal/manifest"
	"github.com/momentmaker/kaijutsu/cli/internal/paths"
	"github.com/momentmaker/kaijutsu/cli/internal/skill"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newRemoveCmd() *cobra.Command {
	var global bool
	var cascade bool
	var yes bool
	cmd := &cobra.Command{
		Use:   "remove <skill>",
		Short: "Remove an installed skill",
		Long: `Remove an installed skill.

By default, refuses to remove a skill that another installed skill
depends on (via deps.skills). Pass --cascade to also remove orphans —
deps that exist only because of the target. The dependency graph is
computed dynamically by walking every installed skill's on-disk
skill.yaml; the lockfile's installedAs field is informational only.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := args[0]

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
			if _, ok := lf.Skills[target]; !ok {
				return errors.New("skill is not installed")
			}

			fwd, err := buildDepGraph(installRoot, lf)
			if err != nil {
				return fmt.Errorf("compute dep graph: %w", err)
			}

			rev := reverseDepGraph(fwd)
			dependents := rev[target]
			if len(dependents) > 0 && !cascade {
				sort.Strings(dependents)
				return fmt.Errorf("refusing to remove %s: still required by %s. Run with --cascade to remove the parent(s) too, or remove them first",
					target, strings.Join(dependents, ", "))
			}

			toRemove := []string{target}
			if cascade {
				toRemove = computeCascade(target, fwd, rev)
				sort.Strings(toRemove)
			}

			out := cmd.OutOrStdout()
			if cascade && len(toRemove) > 1 {
				fmt.Fprintf(out, "Cascade will remove: %s\n", strings.Join(toRemove, ", "))
				if !yes {
					fmt.Fprint(out, "Continue? [y/N]: ")
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
			}

			var m *manifest.Manifest
			if _, statErr := os.Stat(manifestPath); statErr == nil {
				m, _ = manifest.Load(manifestPath)
			}

			for _, name := range toRemove {
				if err := hooks.RemoveForSkill(installRoot, lf.Agents, name); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: hook cleanup for %s failed: %v\n", name, err)
				}
				if err := install.Remove(installRoot, name, lf.Agents); err != nil {
					return err
				}
				delete(lf.Skills, name)
				if m != nil {
					delete(m.Dependencies, name)
				}
				fmt.Fprintf(out, "Removed %s\n", name)
			}

			if err := lf.Save(lockPath); err != nil {
				return err
			}
			if m != nil {
				_ = m.Save(manifestPath)
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&global, "global", "g", false, "remove from global installation")
	cmd.Flags().BoolVar(&cascade, "cascade", false, "also remove dependencies that become orphaned")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "non-interactive: skip the cascade-confirm prompt")
	return cmd
}

// buildDepGraph walks every installed skill's on-disk skill.yaml and
// returns a forward dependency map: name → list of skill names this
// skill directly depends on. Skills without an on-disk skill.yaml
// (vanilla SKILL.md installs) contribute no edges.
func buildDepGraph(installRoot string, lf *manifest.Lockfile) (map[string][]string, error) {
	fwd := make(map[string][]string, len(lf.Skills))
	for name := range lf.Skills {
		fwd[name] = nil
		yamlPath := findSkillYaml(installRoot, name, lf.Agents)
		if yamlPath == "" {
			continue
		}
		data, err := os.ReadFile(yamlPath)
		if err != nil {
			continue
		}
		var sk skill.Skill
		if err := yaml.Unmarshal(data, &sk); err != nil {
			continue
		}
		if sk.Deps == nil {
			continue
		}
		var deps []string
		for _, spec := range sk.Deps.Skills {
			depName, _ := parseSpec(spec)
			deps = append(deps, depName)
		}
		fwd[name] = deps
	}
	return fwd, nil
}

func findSkillYaml(installRoot, name string, agents []string) string {
	seen := map[string]bool{}
	for _, a := range agents {
		dir := paths.AgentSkillsDir(installRoot, a)
		if dir == "" || seen[dir] {
			continue
		}
		seen[dir] = true
		candidate := filepath.Join(dir, name, "skill.yaml")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}

// reverseDepGraph inverts the forward map: name → skills that
// directly depend on name.
func reverseDepGraph(fwd map[string][]string) map[string][]string {
	rev := make(map[string][]string, len(fwd))
	for parent, deps := range fwd {
		for _, d := range deps {
			rev[d] = append(rev[d], parent)
		}
	}
	return rev
}

// computeCascade returns the set of skills to remove when --cascade is
// passed: the target plus every transitive dep whose only living
// dependents are inside the cascade set itself.
func computeCascade(target string, fwd, rev map[string][]string) []string {
	removing := map[string]bool{target: true}
	queue := []string{target}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, dep := range fwd[cur] {
			if removing[dep] {
				continue
			}
			orphan := true
			for _, parent := range rev[dep] {
				if !removing[parent] {
					orphan = false
					break
				}
			}
			if orphan {
				removing[dep] = true
				queue = append(queue, dep)
			}
		}
	}
	out := make([]string, 0, len(removing))
	for n := range removing {
		out = append(out, n)
	}
	return out
}
