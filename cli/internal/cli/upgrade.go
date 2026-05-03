package cli

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/momentmaker/kaijutsu/cli/internal/fetch"
	"github.com/momentmaker/kaijutsu/cli/internal/install"
	"github.com/momentmaker/kaijutsu/cli/internal/manifest"
	"github.com/momentmaker/kaijutsu/cli/internal/source"
	"github.com/spf13/cobra"
)

func newUpgradeCmd() *cobra.Command {
	var global, yes, major bool
	cmd := &cobra.Command{
		Use:   "upgrade [<skill>]",
		Short: "Check for newer skill versions; show diffs and apply on confirm",
		Args:  cobra.MaximumNArgs(1),
		Long: `Check the source of every installed skill (or just the named skill)
for newer versions. By default upgrades respect each skill's manifest
constraint (e.g. ^1.0 stays within 1.x). Pass --major to allow upgrades
that exceed the constraint.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			installRoot, manifestPath, lockPath, err := resolveTargets(global, cwd)
			if err != nil {
				return err
			}
			m, err := loadOrInitManifest(manifestPath, global)
			if err != nil {
				return err
			}
			lf, err := manifest.LoadLockfile(lockPath)
			if err != nil {
				return err
			}
			if len(lf.Skills) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no skills installed")
				return nil
			}

			only := ""
			if len(args) == 1 {
				only = args[0]
			}

			fetcher := fetch.New()
			out := cmd.OutOrStdout()

			names := make([]string, 0, len(lf.Skills))
			for n := range lf.Skills {
				names = append(names, n)
			}
			sort.Strings(names)

			candidates, err := planUpgrades(cmd, fetcher, m, lf, names, only, major)
			if err != nil {
				return err
			}
			if len(candidates) == 0 {
				fmt.Fprintln(out, "all skills up to date")
				return nil
			}

			if !yes && !confirmInteractive(cmd, candidates) {
				fmt.Fprintln(out, "aborted")
				return nil
			}

			for _, c := range candidates {
				l, err := loadByLockEntry(cmd.Context(), fetcher, c.name, c.source, c.newRef, c.path, "")
				if err != nil {
					return fmt.Errorf("upgrade %s: %w", c.name, err)
				}
				if err := install.Install(l.dir, installRoot, m.Agents, l.skill); err != nil {
					l.cleanup()
					return err
				}
				v := c.newVersion
				lf.Skills[c.name] = manifest.LockEntry{
					Version:   &v,
					Source:    c.source,
					Ref:       c.newRef,
					Path:      c.path,
					Integrity: l.hash,
				}
				l.cleanup()
				fmt.Fprintf(out, "Upgraded %s: %s -> %s\n", c.name, c.oldVersion, c.newVersion)
			}
			lf.Agents = m.Agents
			return lf.Save(lockPath)
		},
	}
	cmd.Flags().BoolVarP(&global, "global", "g", false, "upgrade global installation")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "non-interactive: apply without prompting")
	cmd.Flags().BoolVar(&major, "major", false, "allow upgrades that exceed the manifest's version constraint")
	return cmd
}

type upgradeCandidate struct {
	name       string
	source     string
	path       string
	oldRef     string
	oldVersion string
	newRef     string
	newVersion string
}

func planUpgrades(cmd *cobra.Command, fetcher *fetch.Fetcher, m *manifest.Manifest, lf *manifest.Lockfile, names []string, only string, major bool) ([]upgradeCandidate, error) {
	var out []upgradeCandidate
	for _, name := range names {
		if only != "" && name != only {
			continue
		}
		entry := lf.Skills[name]
		if entry.Source == "local" {
			continue
		}
		src, err := source.Parse(entry.Source)
		if err != nil {
			return nil, fmt.Errorf("upgrade %s: %w", name, err)
		}
		tags, err := fetcher.ListTags(cmd.Context(), src)
		if err != nil {
			return nil, fmt.Errorf("list tags for %s: %w", name, err)
		}
		var (
			newTag     string
			newVersion string
			constraint *semver.Constraints
		)
		if rangeSpec, ok := m.Dependencies[name]; ok && !major {
			constraint, _ = semver.NewConstraint(rangeSpec)
		}
		for _, t := range tags {
			v, err := semver.NewVersion(t)
			if err != nil {
				continue
			}
			if constraint != nil && !constraint.Check(v) {
				continue
			}
			newTag = t
			newVersion = v.String()
			break
		}
		if newVersion == "" {
			continue
		}
		oldV := ""
		if entry.Version != nil {
			oldV = *entry.Version
		}
		if oldV == newVersion {
			continue
		}
		// Only flag actual upgrades (newer), not equal/older.
		if oldV != "" {
			oldSV, oerr := semver.NewVersion(oldV)
			newSV, nerr := semver.NewVersion(newVersion)
			if oerr == nil && nerr == nil && !newSV.GreaterThan(oldSV) {
				continue
			}
		}
		newRef, err := fetcher.ResolveTagSHA(cmd.Context(), src, newTag)
		if err != nil {
			return nil, fmt.Errorf("resolve tag %s for %s: %w", newTag, name, err)
		}
		out = append(out, upgradeCandidate{
			name:       name,
			source:     entry.Source,
			path:       entry.Path,
			oldRef:     entry.Ref,
			oldVersion: oldV,
			newRef:     newRef,
			newVersion: newVersion,
		})
	}
	return out, nil
}

func confirmInteractive(cmd *cobra.Command, candidates []upgradeCandidate) bool {
	out := cmd.OutOrStdout()
	fmt.Fprintln(out, "Upgrades available:")
	for _, c := range candidates {
		fmt.Fprintf(out, "  %s: %s -> %s\n", c.name, c.oldVersion, c.newVersion)
	}
	fmt.Fprint(out, "Apply? [y/N]: ")
	r := bufio.NewReader(os.Stdin)
	line, err := r.ReadString('\n')
	if err != nil {
		return false
	}
	line = strings.TrimSpace(strings.ToLower(line))
	return line == "y" || line == "yes"
}

