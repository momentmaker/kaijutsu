package swarm

import (
	"os/exec"
	"sync"
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

// availabilityCache memoizes Available() results within a single
// process. The codex probe shells out (~1s per call) and there are
// multiple call sites per `jutsu swarm` invocation (Available,
// AvailableAgents, pickSynthesizer). Without caching, those add up.
var (
	availabilityCache   = map[AgentName]bool{}
	availabilityCacheMu sync.Mutex
	availabilityCached  = map[AgentName]bool{}
)

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
//
// Result is cached for the lifetime of the process. Auth state can
// change mid-run (e.g., user runs `claude /logout` in another shell)
// but jutsu invocations are short-lived enough that probing once is
// the right tradeoff.
func Available(name AgentName) bool {
	availabilityCacheMu.Lock()
	if availabilityCached[name] {
		v := availabilityCache[name]
		availabilityCacheMu.Unlock()
		return v
	}
	availabilityCacheMu.Unlock()

	v := probeAvailable(name)

	availabilityCacheMu.Lock()
	availabilityCache[name] = v
	availabilityCached[name] = true
	availabilityCacheMu.Unlock()
	return v
}

// ResetAvailabilityCache clears the cached probes. Exposed for tests
// + future commands that explicitly re-check (e.g. `jutsu doctor`).
func ResetAvailabilityCache() {
	availabilityCacheMu.Lock()
	defer availabilityCacheMu.Unlock()
	availabilityCache = map[AgentName]bool{}
	availabilityCached = map[AgentName]bool{}
}

func probeAvailable(name AgentName) bool {
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
