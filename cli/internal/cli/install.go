package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/momentmaker/kaijutsu/cli/internal/detect"
	"github.com/momentmaker/kaijutsu/cli/internal/fetch"
	"github.com/momentmaker/kaijutsu/cli/internal/install"
	"github.com/momentmaker/kaijutsu/cli/internal/manifest"
	"github.com/momentmaker/kaijutsu/cli/internal/paths"
	"github.com/momentmaker/kaijutsu/cli/internal/sign"
	"github.com/momentmaker/kaijutsu/cli/internal/skill"
	"github.com/spf13/cobra"
)

func newInstallCmd() *cobra.Command {
	var global bool
	var registryPath string

	cmd := &cobra.Command{
		Use:   "install [<skill>[@constraint]]",
		Short: "Install a skill (and its deps), or sync the lockfile when run without args",
		Args:  cobra.MaximumNArgs(1),
		Long: `Install a skill from the default kaijutsu registry, a third-party
source listed in registry/index.json, or a local monorepo checkout.

Source resolution order:
  1. --registry <path>    : install from a local kaijutsu monorepo (dev workflow)
  2. KAIJUTSU_REGISTRY    : same as --registry
  3. cwd looks like a kaijutsu monorepo (has skills/core/) : use cwd
  4. otherwise            : fetch from the default remote registry, falling back
                            to registry/index.json for third-party skills.

Skills with deps.skills entries pull in their dependencies recursively.
Each transitive install is recorded in kaijutsu.lock.json with
installedAs = "dep:<parent>" so removal can warn about orphaned deps.

Run without arguments inside a project to sync from kaijutsu.lock.json:
every entry is re-fetched at its pinned ref and verified against its
recorded sha256 integrity.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			installRoot, manifestPath, lockPath, err := resolveTargets(global, cwd)
			if err != nil {
				return err
			}

			if len(args) == 0 {
				return runSyncFromLockfile(cmd, installRoot, manifestPath, lockPath, global)
			}

			name, constraint := parseSpec(args[0])
			localReg := resolveLocalRegistry(registryPath, cwd)

			m, err := loadOrInitManifest(manifestPath, global)
			if err != nil {
				return err
			}
			lf, err := loadOrInitLockfile(lockPath, m.Agents)
			if err != nil {
				return err
			}

			sess := &installSession{
				cmd:         cmd,
				fetcher:     fetch.New(),
				localReg:    localReg,
				defaultReg:  registryDefault(m),
				installRoot: installRoot,
				m:           m,
				lf:          lf,
				visited:     map[string]bool{},
			}

			if err := sess.installOne(name, constraint, ""); err != nil {
				return err
			}

			if err := m.Save(manifestPath); err != nil {
				return err
			}
			if err := lf.Save(lockPath); err != nil {
				return err
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&global, "global", "g", false, "install globally (~/.claude/skills/, ~/.agents/skills/)")
	cmd.Flags().StringVar(&registryPath, "registry", "", "path to a local kaijutsu monorepo checkout (skips remote fetch)")
	return cmd
}

// installSession bundles the per-invocation state for a recursive install.
// visited tracks every skill name encountered in this invocation so cycles
// (or skills already pulled in as transitive deps) install at most once.
type installSession struct {
	cmd         *cobra.Command
	fetcher     *fetch.Fetcher
	localReg    string
	defaultReg  string
	installRoot string
	m           *manifest.Manifest
	lf          *manifest.Lockfile
	visited     map[string]bool
}

// installOne loads the named skill (locally or remotely), recurses into
// deps.skills, then installs the skill itself. parent is "" for the
// user-requested skill and the parent skill name for transitive deps.
func (s *installSession) installOne(name, constraint, parent string) error {
	if s.visited[name] {
		return nil
	}
	s.visited[name] = true

	l, err := s.load(name, constraint)
	if err != nil {
		return err
	}
	defer l.cleanup()

	if l.skill.Deps != nil {
		for _, depSpec := range l.skill.Deps.Skills {
			depName, depConstraint := parseSpec(depSpec)
			if err := s.installOne(depName, depConstraint, l.skill.Name); err != nil {
				return fmt.Errorf("dep %s of %s: %w", depName, l.skill.Name, err)
			}
		}
	}

	advisorySignerNotice(s.cmd, l.skill)

	if err := install.Install(l.dir, s.installRoot, s.m.Agents, l.skill); err != nil {
		return err
	}

	s.lf.Agents = s.m.Agents
	recordInstall(s.m, s.lf, l, constraint, parent)

	suffix := ""
	if parent != "" {
		suffix = fmt.Sprintf(" [dep of %s]", parent)
	}
	fmt.Fprintf(s.cmd.OutOrStdout(), "Installed %s@%s (%s, %s)%s\n",
		l.skill.Name, l.skill.Version, l.source, shortRef(l.ref), suffix)
	return nil
}

func (s *installSession) load(name, constraint string) (*loaded, error) {
	if s.localReg != "" {
		return loadLocal(s.localReg, name)
	}
	return loadRemote(s.cmd.Context(), s.fetcher, s.defaultReg, name, constraint)
}

// runSyncFromLockfile re-installs every skill listed in the lockfile at
// its pinned ref, verifying the recorded integrity. Powers `jutsu install`
// with no arguments — the reproducibility primitive.
func runSyncFromLockfile(cmd *cobra.Command, installRoot, manifestPath, lockPath string, global bool) error {
	if _, err := os.Stat(lockPath); err != nil {
		return errors.New("no lockfile to sync from; run `jutsu install <skill>` first")
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
		fmt.Fprintln(cmd.OutOrStdout(), "lockfile is empty; nothing to install")
		return nil
	}
	fetcher := fetch.New()
	for name, entry := range lf.Skills {
		if entry.Source == "local" {
			fmt.Fprintf(cmd.ErrOrStderr(), "skipping %s: source=local cannot be re-fetched\n", name)
			continue
		}
		l, err := loadByLockEntry(cmd.Context(), fetcher, name, entry.Source, entry.Ref, entry.Path, entry.Integrity)
		if err != nil {
			return fmt.Errorf("sync %s: %w", name, err)
		}
		if err := install.Install(l.dir, installRoot, m.Agents, l.skill); err != nil {
			l.cleanup()
			return err
		}
		l.cleanup()
		fmt.Fprintf(cmd.OutOrStdout(), "Synced %s@%s\n", name, displayVersion(entry.Version, entry.Ref))
	}
	return nil
}

// displayVersion formats the version for log output, falling back to a
// short ref when no semver was recorded (SHA-pinned skills).
func displayVersion(version *string, ref string) string {
	if version != nil {
		return *version
	}
	return shortRef(ref)
}

// recordInstall mutates manifest dependencies + lockfile entry to reflect
// a successful install. constraint is the user-supplied spec (e.g., "^1.0")
// or "" — when empty, the manifest dependency uses "^<resolved-version>".
// parent is the skill that pulled this one in transitively, or "" for direct.
func recordInstall(m *manifest.Manifest, lf *manifest.Lockfile, l *loaded, constraint, parent string) {
	// Only direct installs add an entry to manifest dependencies.
	if parent == "" {
		depConstraint := constraint
		if depConstraint == "" && l.version != "" {
			depConstraint = "^" + l.version
		}
		if depConstraint == "" {
			depConstraint = l.ref // SHA pin when no version available
		}
		m.Dependencies[l.skill.Name] = depConstraint
	}

	var versionPtr *string
	if l.version != "" {
		v := l.version
		versionPtr = &v
	}

	installedAs := "direct"
	if parent != "" {
		installedAs = "dep:" + parent
	}

	// Preserve "direct" if a skill is already installed directly and is
	// being re-encountered as a transitive dep.
	if existing, ok := lf.Skills[l.skill.Name]; ok {
		if existing.InstalledAs == "direct" || existing.InstalledAs == "" {
			installedAs = existing.InstalledAs
			if installedAs == "" {
				installedAs = "direct"
			}
		}
	}

	lf.Skills[l.skill.Name] = manifest.LockEntry{
		Version:     versionPtr,
		Source:      l.source,
		Ref:         l.ref,
		Path:        l.path,
		Integrity:   l.hash,
		InstalledAs: installedAs,
	}
}

// resolveLocalRegistry inspects --registry, $KAIJUTSU_REGISTRY, and cwd to
// decide whether to use a local registry path. Returns "" for remote mode.
func resolveLocalRegistry(flag, cwd string) string {
	if flag != "" {
		return flag
	}
	if env := os.Getenv("KAIJUTSU_REGISTRY"); env != "" {
		return env
	}
	if _, err := os.Stat(filepath.Join(cwd, "skills", "core")); err == nil {
		return cwd
	}
	return ""
}

func resolveTargets(global bool, cwd string) (installRoot, manifestPath, lockPath string, err error) {
	if global {
		installRoot, err = paths.HomeDir()
		if err != nil {
			return "", "", "", err
		}
		dir := filepath.Join(installRoot, paths.GlobalConfigDirName)
		if err = os.MkdirAll(dir, 0755); err != nil {
			return "", "", "", err
		}
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

// registryDefault returns the manifest's registry.default URL or the
// canonical kaijutsu URL.
func registryDefault(m *manifest.Manifest) string {
	if m.Registry != nil && m.Registry.Default != "" {
		return m.Registry.Default
	}
	return "https://github.com/momentmaker/kaijutsu"
}

// shortRef trims a SHA to 7 chars for display; passes through "local".
func shortRef(ref string) string {
	if len(ref) > 7 && ref != "local" {
		return ref[:7]
	}
	return ref
}

// advisorySignerNotice emits a warning line when a skill declares an
// expected-signer but the install path can't (yet) verify the signature.
// v0 placeholder — once sign-core.yml is publishing signature bundles
// and skills/core/* declare an expected-signer, this should call
// sign.VerifyBlob and hard-fail on mismatch.
func advisorySignerNotice(cmd *cobra.Command, sk *skill.Skill) {
	if sk.Trust == nil || sk.Trust.ExpectedSigner == "" {
		return
	}
	stderr := cmd.ErrOrStderr()
	if !sign.Available() {
		fmt.Fprintf(stderr, "note: %s declares expected-signer %q but cosign is not on PATH; skipping verification.\n", sk.Name, sk.Trust.ExpectedSigner)
		return
	}
	fmt.Fprintf(stderr, "note: %s declares expected-signer %q; v0 verify is a stub (signed releases not yet published).\n", sk.Name, sk.Trust.ExpectedSigner)
}
