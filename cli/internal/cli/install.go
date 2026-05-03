package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/momentmaker/kaijutsu/cli/internal/detect"
	"github.com/momentmaker/kaijutsu/cli/internal/install"
	"github.com/momentmaker/kaijutsu/cli/internal/manifest"
	"github.com/momentmaker/kaijutsu/cli/internal/paths"
	"github.com/momentmaker/kaijutsu/cli/internal/skill"
	"github.com/spf13/cobra"
)

func newInstallCmd() *cobra.Command {
	var global bool
	var registry string

	cmd := &cobra.Command{
		Use:   "install <skill>",
		Short: "Install a skill into the project (or globally with -g)",
		Args:  cobra.ExactArgs(1),
		Long: `Install a skill from a local kaijutsu monorepo checkout.

For Stage 2 the registry must be a local filesystem path. Use --registry,
the KAIJUTSU_REGISTRY env var, or run from inside a kaijutsu monorepo
(directory containing skills/core/).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			skillName := args[0]
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}

			registryPath, err := resolveRegistry(registry, cwd)
			if err != nil {
				return err
			}

			srcDir := filepath.Join(registryPath, "skills", "core", skillName)
			if _, err := os.Stat(filepath.Join(srcDir, "skill.yaml")); err != nil {
				return fmt.Errorf("skill %q not found in registry at %s", skillName, registryPath)
			}

			sk, err := skill.Load(filepath.Join(srcDir, "skill.yaml"))
			if err != nil {
				return err
			}

			installRoot, manifestPath, lockPath := resolveTargets(global, cwd)

			m, err := loadOrInitManifest(manifestPath, global)
			if err != nil {
				return err
			}

			lf, err := loadOrInitLockfile(lockPath, m.Agents)
			if err != nil {
				return err
			}

			if err := install.Install(srcDir, installRoot, m.Agents, sk); err != nil {
				return err
			}

			m.Dependencies[sk.Name] = "^" + sk.Version
			v := sk.Version
			lf.Skills[sk.Name] = manifest.LockEntry{
				Version:   &v,
				Source:    "momentmaker/kaijutsu",
				Ref:       "local",
				Integrity: "",
			}

			if err := m.Save(manifestPath); err != nil {
				return err
			}
			if err := lf.Save(lockPath); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Installed %s@%s\n", sk.Name, sk.Version)
			return nil
		},
	}
	cmd.Flags().BoolVarP(&global, "global", "g", false, "install globally (~/.claude/skills/, ~/.agents/skills/)")
	cmd.Flags().StringVar(&registry, "registry", "", "path to a local kaijutsu monorepo checkout (defaults to KAIJUTSU_REGISTRY env, then cwd if it looks like a registry)")
	return cmd
}

func resolveRegistry(flag, cwd string) (string, error) {
	if flag != "" {
		return flag, nil
	}
	if env := os.Getenv("KAIJUTSU_REGISTRY"); env != "" {
		return env, nil
	}
	// If cwd looks like a kaijutsu monorepo (has skills/core/), use it.
	if _, err := os.Stat(filepath.Join(cwd, "skills", "core")); err == nil {
		return cwd, nil
	}
	return "", errors.New("no registry source: pass --registry, set KAIJUTSU_REGISTRY, or run from inside a kaijutsu monorepo")
}

func resolveTargets(global bool, cwd string) (installRoot, manifestPath, lockPath string) {
	if global {
		installRoot = paths.HomeDir()
		dir := filepath.Join(installRoot, paths.GlobalConfigDirName)
		_ = os.MkdirAll(dir, 0755)
		manifestPath = filepath.Join(dir, paths.GlobalManifestFile)
		lockPath = filepath.Join(dir, paths.GlobalLockfileFile)
		return
	}
	installRoot = cwd
	manifestPath = filepath.Join(cwd, paths.ManifestFile)
	lockPath = filepath.Join(cwd, paths.LockfileFile)
	return
}

func loadOrInitManifest(path string, global bool) (*manifest.Manifest, error) {
	if _, err := os.Stat(path); err == nil {
		return manifest.Load(path)
	}
	if !global {
		return nil, errors.New("no kaijutsu.json in current directory; run `jutsu init` first")
	}
	// Global: synthesize a default manifest from currently detected agents
	// (or fall back to [claude] if none detected).
	active := detect.Active()
	if len(active) == 0 {
		active = []string{"claude"}
	}
	return manifest.New(active), nil
}

func loadOrInitLockfile(path string, agents []string) (*manifest.Lockfile, error) {
	if _, err := os.Stat(path); err == nil {
		return manifest.LoadLockfile(path)
	}
	return manifest.NewLockfile(agents), nil
}
