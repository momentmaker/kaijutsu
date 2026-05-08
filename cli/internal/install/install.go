// Package install copies a skill directory tree to its target install
// paths under the given root (project cwd or ~ for global).
package install

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/momentmaker/kaijutsu/cli/internal/paths"
	"github.com/momentmaker/kaijutsu/cli/internal/skill"
)

// Install copies srcDir (the skill directory in the registry) to each
// install destination implied by activeAgents ∩ skill.agents, beneath
// installRoot. Existing destinations are replaced.
func Install(srcDir, installRoot string, activeAgents []string, sk *skill.Skill) error {
	if err := skill.ValidateName(sk.Name); err != nil {
		return err
	}
	targets := intersect(activeAgents, sk.Agents)
	if len(targets) == 0 {
		return fmt.Errorf("no overlap between active agents %v and skill's supported agents %v", activeAgents, sk.Agents)
	}
	for _, dest := range destinations(installRoot, targets) {
		fullDest := filepath.Join(dest, sk.Name)
		if err := os.MkdirAll(filepath.Dir(fullDest), 0755); err != nil {
			return err
		}
		// v0.11.0: archive a pre-existing SKILL.md to .archived/SKILL.md
		// before overwrite. Currently only autopilot needs this — v1
		// users have local edits worth preserving across the v2
		// replacement. Other skills that need this in the future can
		// either special-case here or graduate to a generic
		// `archive_on_overwrite: true` skill.yaml field.
		if sk.Name == "autopilot" {
			if err := archivePreExistingAutopilot(fullDest); err != nil {
				// Best-effort: log to stderr would require plumbing;
				// we silently continue. The install will proceed +
				// overwrite without the archive backup.
				_ = err
			}
		}
		if err := os.RemoveAll(fullDest); err != nil {
			return err
		}
		if err := copyDir(srcDir, fullDest); err != nil {
			return fmt.Errorf("copy to %s: %w", fullDest, err)
		}
	}
	return nil
}

// archivePreExistingAutopilot copies a pre-existing SKILL.md at
// fullDest/SKILL.md into a SIBLING dir (autopilot.archived/SKILL.md)
// before the install pipeline RemoveAll's fullDest. Sibling-not-
// child placement matters: putting the archive inside fullDest
// would get nuked by the very RemoveAll that follows.
//
// Idempotent: if no existing SKILL.md, no-op. Safe across re-runs:
// each call overwrites the prior archive so multiple v0.11.x
// updates don't stack — only the most-recent pre-overwrite content
// is preserved.
func archivePreExistingAutopilot(fullDest string) error {
	src := filepath.Join(fullDest, "SKILL.md")
	body, err := os.ReadFile(src)
	if os.IsNotExist(err) {
		return nil // nothing to archive
	}
	if err != nil {
		return err
	}
	archDir := fullDest + ".archived"
	if err := os.MkdirAll(archDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(archDir, "SKILL.md"), body, 0o644)
}

// Remove deletes the named skill from each install destination implied by
// the given agents. skillName is validated as a path-safe identifier.
func Remove(installRoot, skillName string, agents []string) error {
	if err := skill.ValidateName(skillName); err != nil {
		return err
	}
	for _, dest := range destinations(installRoot, agents) {
		full := filepath.Join(dest, skillName)
		if err := os.RemoveAll(full); err != nil {
			return err
		}
	}
	return nil
}

// destinations dedupes per family — claude has its own dir, codex+gemini share .agents/.
func destinations(root string, agents []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, a := range agents {
		dir := paths.AgentSkillsDir(root, a)
		if dir == "" || seen[dir] {
			continue
		}
		seen[dir] = true
		out = append(out, dir)
	}
	return out
}

func intersect(a, b []string) []string {
	set := map[string]bool{}
	for _, x := range a {
		set[x] = true
	}
	var out []string
	for _, y := range b {
		if set[y] {
			out = append(out, y)
		}
	}
	return out
}

// CopyTree recursively copies src into dst. Re-exports the internal
// helper for callers (e.g. publish) that need to stage a skill into
// a working tree.
func CopyTree(src, dst string) error {
	return copyDir(src, dst)
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	if info, err := os.Stat(src); err == nil {
		_ = os.Chmod(dst, info.Mode())
	}
	return nil
}
