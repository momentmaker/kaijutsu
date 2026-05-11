package main

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/momentmaker/kaijutsu/cli/internal/cli"
)

// TestInvokedSubcommand_StripsArgsAndFlags pins the v0.16.0 privacy
// contract: positional args, flag values, and content NEVER make it
// into the usage log.
func TestInvokedSubcommand_StripsArgsAndFlags(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"no-args", []string{}, "jutsu"},
		{"version-flag", []string{"--version"}, "jutsu"},
		{"top-level-only", []string{"init"}, "jutsu init"},
		{"nested-finding", []string{"finding", "stats"}, "jutsu finding stats"},
		{"deeply-nested", []string{"agent", "persona", "browse"}, "jutsu agent persona browse"},
		{"swarm-preset", []string{"swarm", "pr-review", "--pr", "42"}, "jutsu swarm pr-review"},
		{"install-with-positional-arg", []string{"install", "decide"}, "jutsu install"},
		{"install-with-flag", []string{"install", "decide", "--yes"}, "jutsu install"},
		{"unknown-top-level", []string{"definitely-not-a-command"}, "jutsu"},
		{"finding-precision-recommend", []string{"finding", "precision", "--recommend"}, "jutsu finding precision"},
		{"user-preset-buckets-to-swarm", []string{"swarm", "my-custom-preset"}, "jutsu swarm"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := invokedSubcommand(c.args)
			if got != c.want {
				t.Errorf("invokedSubcommand(%v) = %q; want %q", c.args, got, c.want)
			}
		})
	}
}

// TestKnownSubcommandPaths_CoversCobraTree is the drift-detection gate.
// If a future change adds a (sub)command to cli.NewRootCmd but forgets
// to extend `knownSubcommandPaths`, the new command silently buckets to
// its parent in the usage log — privacy still holds, but the analytics
// pyramid skews. Catch the drift here so it's a build-time failure, not
// a slow-burn observability bug.
func TestKnownSubcommandPaths_CoversCobraTree(t *testing.T) {
	root := cli.NewRootCmd()
	missing := []string{}
	walk(root, "jutsu", func(path string) {
		if !knownSubcommandPaths[path] {
			missing = append(missing, path)
		}
	})
	if len(missing) > 0 {
		t.Errorf("knownSubcommandPaths is missing %d cobra subcommand path(s):\n  %s\n\nAdd them to cmd/jutsu/main.go to preserve the usage-log shape.",
			len(missing), strings.Join(missing, "\n  "))
	}
}

// walk recursively visits every subcommand in the cobra tree, invoking
// visit with the full dotted path. Hidden commands are skipped — they
// don't appear in normal use, so the usage log wouldn't capture them.
// The root command itself is the seed; we walk into its Commands.
func walk(cmd *cobra.Command, path string, visit func(string)) {
	for _, sub := range cmd.Commands() {
		if sub.Hidden {
			continue
		}
		childPath := path + " " + sub.Name()
		visit(childPath)
		walk(sub, childPath, visit)
	}
}
