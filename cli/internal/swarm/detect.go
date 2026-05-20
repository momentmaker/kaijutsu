package swarm

import (
	"os"
	"os/exec"
	"strings"
	"sync"
)

// AgentName is one of the CLI agents kaijutsu orchestrates.
type AgentName string

const (
	AgentClaude      AgentName = "claude"
	AgentCodex       AgentName = "codex"
	AgentAntigravity AgentName = "antigravity"
)

// AllAgents is the canonical order — used for stable iteration and
// table-column ordering.
var AllAgents = []AgentName{AgentClaude, AgentCodex, AgentAntigravity}

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
//   - antigravity: `agy` binary on PATH (similar to claude — `-p` errors if no auth).
//
// We deliberately don't probe deeper for claude/antigravity because their
// auth state lives in user-config dirs that change across versions and
// re-running a stale probe regularly produces false negatives.
//
// Result is cached for the lifetime of the process. Auth state can
// change mid-run (e.g., user runs `claude /logout` in another shell)
// but jutsu invocations are short-lived enough that probing once is
// the right tradeoff.
func Available(name AgentName) bool {
	// v0.9 KAIJUTSU_DISABLE_AGENTS=<comma-list> gate. Test affordance
	// + escape hatch for users who want to force a specific provider
	// subset without uninstalling CLIs. Short-circuits BEFORE the
	// availability cache so toggling the env-var mid-run takes effect
	// immediately.
	if isAgentDisabledByEnv(name) {
		return false
	}
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

// isAgentDisabledByEnv reports whether the agent name appears in
// the KAIJUTSU_DISABLE_AGENTS env-var (comma-separated). Empty
// env-var returns false. Whitespace-tolerant per item.
func isAgentDisabledByEnv(name AgentName) bool {
	v := os.Getenv("KAIJUTSU_DISABLE_AGENTS")
	if v == "" {
		return false
	}
	for _, item := range strings.Split(v, ",") {
		if AgentName(strings.TrimSpace(item)) == name {
			return true
		}
	}
	return false
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
	case AgentAntigravity:
		_, err := exec.LookPath("agy")
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
