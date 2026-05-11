package main

import (
	"os"
	"time"

	"github.com/momentmaker/kaijutsu/cli/internal/cli"
	"github.com/momentmaker/kaijutsu/cli/internal/usage"
)

func main() {
	start := time.Now()

	// ExecuteC returns the cobra command that was matched (or the root
	// command if parsing failed before a subcommand match). We log via
	// c.CommandPath() — cobra's own canonical path — so the usage log
	// stays in sync with the actual command tree by construction. No
	// allowlist to drift, no arg/flag values ever in scope.
	root := cli.NewRootCmd()
	matched, err := root.ExecuteC()
	code := cli.ExitCode(err)

	// Local-only usage log. Records {cobra-resolved subcommand path,
	// exit, ms}. No content, no flag values, no args — cobra returns
	// names only, never positional input. See cli/internal/usage/log.go.
	// Opt-out: KAIJUTSU_USAGE_LOG=0. Telemetry failures are silent.
	cmdPath := "jutsu"
	if matched != nil {
		cmdPath = matched.CommandPath()
	}
	usage.Append(cmdPath, code, time.Since(start))

	os.Exit(code)
}
