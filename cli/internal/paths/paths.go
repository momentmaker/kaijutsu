// Package paths centralizes filesystem path constants and resolution
// for kaijutsu's install targets.
//
// Codex and Gemini both honor `.agents/skills/` as the cross-tool alias path,
// so a single write to that directory satisfies both agents. Claude Code is
// the only target that needs its own `.claude/skills/` directory.
package paths

import (
	"os"
	"path/filepath"
)

const (
	ClaudeDirName = ".claude"
	AgentsDirName = ".agents"
	CodexDirName  = ".codex"
	GeminiDirName = ".gemini"

	SkillsSubdir = "skills"

	ManifestFile = "kaijutsu.json"
	LockfileFile = "kaijutsu.lock.json"

	GlobalConfigDirName = ".kaijutsu"
	GlobalManifestFile  = "global.json"
	GlobalLockfileFile  = "global.lock.json"
)

// HomeDir returns the user's home directory.
func HomeDir() (string, error) {
	return os.UserHomeDir()
}

// AgentSkillsDir returns the path where the given agent expects skills,
// rooted at root (cwd for project install, ~ for global install).
// Codex and Gemini share `.agents/skills/`.
func AgentSkillsDir(root, agent string) string {
	switch agent {
	case "claude":
		return filepath.Join(root, ClaudeDirName, SkillsSubdir)
	case "codex", "gemini":
		return filepath.Join(root, AgentsDirName, SkillsSubdir)
	default:
		return ""
	}
}
