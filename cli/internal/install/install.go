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
		if err := os.RemoveAll(fullDest); err != nil {
			return err
		}
		if err := copyDir(srcDir, fullDest); err != nil {
			return fmt.Errorf("copy to %s: %w", fullDest, err)
		}
	}
	return nil
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
