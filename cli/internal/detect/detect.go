// Package detect identifies which AI agents are installed on the user's
// machine by checking for their config directories under $HOME.
package detect

import (
	"os"
	"path/filepath"

	"github.com/momentmaker/kaijutsu/cli/internal/paths"
)

// Active returns the list of agent names whose user-config dirs exist
// under home. Returns names in stable order: claude, codex, gemini.
func Active() []string {
	return ActiveAt(paths.HomeDir())
}

// ActiveAt is Active scoped to a specific home directory (testable).
func ActiveAt(home string) []string {
	out := []string{}
	for _, agent := range []string{"claude", "codex", "gemini"} {
		dir := agentDir(agent)
		if dirExists(filepath.Join(home, dir)) {
			out = append(out, agent)
		}
	}
	return out
}

func agentDir(agent string) string {
	switch agent {
	case "claude":
		return paths.ClaudeDirName
	case "codex":
		return paths.CodexDirName
	case "gemini":
		return paths.GeminiDirName
	}
	return ""
}

func dirExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}
