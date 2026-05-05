package swarm

import (
	"os/exec"
)

// AgentName is one of the CLI agents kaijutsu orchestrates.
type AgentName string

const (
	AgentClaude AgentName = "claude"
	AgentCodex  AgentName = "codex"
	AgentGemini AgentName = "gemini"
)

// AllAgents is the canonical order — used for stable iteration and
// table-column ordering.
var AllAgents = []AgentName{AgentClaude, AgentCodex, AgentGemini}

// Available reports whether the named CLI is installed AND has a
// usable session. Auth probe per agent:
//   - claude: binary on PATH (the `-p` invocation will fail loudly
//     with a clear message if there's no auth).
//   - codex:  `codex auth status` exits 0.
//   - gemini: binary on PATH (similar to claude — `-p` errors if no auth).
//
// We deliberately don't probe deeper for claude/gemini because their
// auth state lives in user-config dirs that change across versions and
// re-running a stale probe regularly produces false negatives.
func Available(name AgentName) bool {
	switch name {
	case AgentClaude:
		_, err := exec.LookPath("claude")
		return err == nil
	case AgentCodex:
		if _, err := exec.LookPath("codex"); err != nil {
			return false
		}
		// codex has a clean auth-status command; use it.
		return exec.Command("codex", "auth", "status").Run() == nil
	case AgentGemini:
		_, err := exec.LookPath("gemini")
		return err == nil
	}
	return false
}

// AvailableAgents returns the subset of AllAgents that pass Available.
func AvailableAgents() []AgentName {
	var out []AgentName
	for _, a := range AllAgents {
		if Available(a) {
			out = append(out, a)
		}
	}
	return out
}
