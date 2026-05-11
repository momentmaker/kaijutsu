package main

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"

	"github.com/momentmaker/kaijutsu/cli/internal/cli"
)

// TestCommandPath_StripsArgsAndFlags pins the v0.16.0 privacy contract:
// positional args, flag values, and content NEVER reach the usage log.
//
// Implementation uses cobra's CommandPath(), so this is really a contract
// test on cobra — but the contract is what we depend on, so we pin it.
func TestCommandPath_StripsArgsAndFlags(t *testing.T) {
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
		{"swarm-preset-with-flag", []string{"swarm", "pr-review", "--diff-from-branch", "main"}, "jutsu swarm pr-review"},
		{"install-with-positional-arg", []string{"install", "decide"}, "jutsu install"},
		{"install-with-flag", []string{"install", "decide", "--yes"}, "jutsu install"},
		{"finding-precision-with-recommend", []string{"finding", "precision", "--recommend"}, "jutsu finding precision"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := cli.NewRootCmd()
			// Silence cobra's stdout/stderr so the test doesn't pollute
			// test output with help/error text. We only care about the
			// resolved CommandPath, not what the RunE printed.
			var sink bytes.Buffer
			root.SetOut(&sink)
			root.SetErr(&sink)
			root.SetArgs(c.args)

			// Many subcommands' RunE will fail (missing creds, missing
			// args, etc.) — that's fine. ExecuteC still returns the
			// command cobra matched, which is all we're testing here.
			matched, _ := root.ExecuteC()
			got := pathOf(matched)
			if got != c.want {
				t.Errorf("CommandPath for args=%v: got %q, want %q", c.args, got, c.want)
			}
		})
	}
}

func pathOf(c *cobra.Command) string {
	if c == nil {
		return "jutsu"
	}
	return c.CommandPath()
}
