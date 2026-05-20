// Package detect identifies which AI agents are installed on the user's
// machine by checking for their config directories under $HOME.
package detect

import (
	"os"
	"path/filepath"

	"github.com/momentmaker/kaijutsu/cli/internal/paths"
)

// Active returns the list of agent names whose user-config dirs exist
// under $HOME. Returns names in stable order: claude, codex, antigravity.
// If $HOME cannot be determined, returns an empty slice.
func Active() []string {
	home, err := paths.HomeDir()
	if err != nil {
		return nil
	}
	return ActiveAt(home)
}

// ActiveAt is Active scoped to a specific home directory (testable).
func ActiveAt(home string) []string {
	out := []string{}
	for _, agent := range []string{"claude", "codex", "antigravity"} {
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
	case "antigravity":
		return paths.AntigravityDirName
	}
	return ""
}

func dirExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}
