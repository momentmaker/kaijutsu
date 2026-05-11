package main

import (
	"os"
	"strings"
	"time"

	"github.com/momentmaker/kaijutsu/cli/internal/cli"
	"github.com/momentmaker/kaijutsu/cli/internal/usage"
)

func main() {
	start := time.Now()
	err := cli.NewRootCmd().Execute()
	code := cli.ExitCode(err)

	// Local-only usage log. Records {subcommand path, exit, ms}. No flags,
	// no args, no content. See cli/internal/usage/log.go for the contract.
	// Opt-out: KAIJUTSU_USAGE_LOG=0. Telemetry failures are silent — never
	// break the user's actual command on a write fail.
	usage.Append(invokedSubcommand(os.Args[1:]), code, time.Since(start))

	os.Exit(code)
}

// invokedSubcommand reconstructs the subcommand path from os.Args.
//
// Privacy contract: NEVER capture positional args, flag values, or
// content — only the subcommand path. To honor that, this walks only
// through nested subcommand names from a closed allowlist (top-level +
// known nested subcommands like `agent persona browse`, `finding stats`).
// Any token that is not a known subcommand name stops the walk. This is
// stricter than a shape heuristic, which captured args like `install
// some-skill` and quietly leaked them to the log.
//
// Examples:
//   []                                    → "jutsu"
//   ["--version"]                         → "jutsu"
//   ["finding", "stats"]                  → "jutsu finding stats"
//   ["swarm", "pr-review", "--pr", "42"]  → "jutsu swarm pr-review"
//   ["install", "decide"]                 → "jutsu install"
//   ["agent", "persona", "browse"]        → "jutsu agent persona browse"
//
// Maintenance: when adding a new top-level command or nested subcommand,
// extend `knownSubcommandPaths` below. CI doesn't enforce this, but the
// fallout is purely usage-log shape — the actual cobra dispatch is
// unaffected.
func invokedSubcommand(args []string) string {
	parts := []string{"jutsu"}
	currentPath := "jutsu"
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			break
		}
		candidate := currentPath + " " + a
		if !knownSubcommandPaths[candidate] {
			break
		}
		parts = append(parts, a)
		currentPath = candidate
	}
	return strings.Join(parts, " ")
}

// knownSubcommandPaths is the closed allowlist of subcommand paths
// recognized by the usage logger. Update on every new (sub)command.
// Mirrors cli.NewRootCmd's AddCommand tree.
var knownSubcommandPaths = map[string]bool{
	// top-level
	"jutsu init":      true,
	"jutsu install":   true,
	"jutsu list":      true,
	"jutsu remove":    true,
	"jutsu upgrade":   true,
	"jutsu lint":      true,
	"jutsu search":    true,
	"jutsu info":      true,
	"jutsu publish":   true,
	"jutsu swarm":     true,
	"jutsu agent":     true,
	"jutsu finding":   true,
	"jutsu describe":  true,
	"jutsu suggest":   true,
	"jutsu dream":     true,
	"jutsu eval":      true,
	"jutsu autopilot": true,
	"jutsu usage":     true,
	// nested under agent
	"jutsu agent list":           true,
	"jutsu agent doctor":         true,
	"jutsu agent add":            true,
	"jutsu agent enable":         true,
	"jutsu agent disable":        true,
	"jutsu agent remove":         true,
	"jutsu agent test":           true,
	"jutsu agent migrate":        true,
	"jutsu agent persona":        true,
	"jutsu agent persona browse": true,
	// nested under finding
	"jutsu finding list":      true,
	"jutsu finding accept":    true,
	"jutsu finding dismiss":   true,
	"jutsu finding precision": true,
	"jutsu finding stats":     true,
	"jutsu finding clear":     true,
	"jutsu finding export":    true,
	"jutsu finding sync-pr":   true,
	// nested under swarm (built-in presets; user presets are dynamic
	// and intentionally bucket to "jutsu swarm")
	"jutsu swarm pr-review":         true,
	"jutsu swarm doc-review":        true,
	"jutsu swarm brainstorm":        true,
	"jutsu swarm refactor-plan":     true,
	"jutsu swarm security-audit":    true,
	"jutsu swarm dream":             true,
	"jutsu swarm reverse":           true,
	"jutsu swarm test-gap":          true,
	"jutsu swarm bug-repro":         true,
	"jutsu swarm code-archaeology":  true,
	"jutsu swarm validate":          true,
	// nested under eval
	"jutsu eval skill":       true,
	"jutsu eval persona":     true,
	"jutsu eval preset":      true,
	"jutsu eval swarm-skill": true,
	// nested under autopilot
	"jutsu autopilot init":   true,
	"jutsu autopilot status": true,
	"jutsu autopilot resume": true,
	"jutsu autopilot run":    true,
	"jutsu autopilot abort":  true,
	// nested under dream
	"jutsu dream list":  true,
	"jutsu dream clear": true,
	// nested under usage
	"jutsu usage stats": true,
}
