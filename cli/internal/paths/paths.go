// Package paths centralizes filesystem path constants and resolution
// for kaijutsu's install targets.
//
// Codex and Antigravity both honor `.agents/skills/` as the cross-tool alias path,
// so a single write to that directory satisfies both agents. Claude Code is
// the only target that needs its own `.claude/skills/` directory.
package paths

import (
	"os"
	"path/filepath"
)

const (
	ClaudeDirName      = ".claude"
	AgentsDirName      = ".agents"
	CodexDirName       = ".codex"
	// AntigravityDirName: agy stores its CLI-specific state under
	// `~/.gemini/antigravity-cli/` rather than its own top-level dir,
	// reusing the Gemini base dir from gemini-cli (verified against
	// agy 1.0.0 on macOS). If agy moves to a dedicated top-level dir
	// (e.g. ~/.antigravity/) in a future release, update this constant.
	AntigravityDirName = ".gemini/antigravity-cli"

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
// Codex and Antigravity share `.agents/skills/`.
func AgentSkillsDir(root, agent string) string {
	switch agent {
	case "claude":
		return filepath.Join(root, ClaudeDirName, SkillsSubdir)
	case "codex", "antigravity":
		return filepath.Join(root, AgentsDirName, SkillsSubdir)
	default:
		return ""
	}
}
