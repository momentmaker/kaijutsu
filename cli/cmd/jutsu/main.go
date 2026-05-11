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

// invokedSubcommand reconstructs the subcommand path from os.Args using a
// shape heuristic (lowercase + dash only; stop at first flag-starting `-`
// or non-subcommand-shaped token). Doesn't resolve flags or validate
// against the real cobra tree — keeps the logger zero-dependency on
// cobra internals.
//
// Examples:
//   []                                    → "jutsu"
//   ["--version"]                         → "jutsu"
//   ["finding", "stats"]                  → "jutsu finding stats"
//   ["swarm", "pr-review", "--pr", "42"]  → "jutsu swarm pr-review"
//   ["install", "decide"]                 → "jutsu install decide"
//                                           (acceptable noise — the
//                                           occasional positional arg
//                                           gets captured; subcommand
//                                           pattern still dominates)
func invokedSubcommand(args []string) string {
	parts := []string{"jutsu"}
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			break
		}
		if !looksLikeSubcommand(a) {
			break
		}
		parts = append(parts, a)
	}
	return strings.Join(parts, " ")
}

func looksLikeSubcommand(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !(r == '-' || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')) {
			return false
		}
	}
	return true
}
