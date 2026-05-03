// Package cli wires the cobra command tree for the jutsu binary.
package cli

import "github.com/spf13/cobra"

// Version is set at build time via -ldflags. Defaults to a dev marker.
var Version = "0.1.0-dev"

// NewRootCmd builds the root cobra command with all subcommands attached.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "jutsu",
		Short:         "Open skills for AI coding agents",
		Long:          "kaijutsu's CLI. Install, list, and manage AI agent skills across Claude, Codex, and Gemini.",
		Version:       Version,
		SilenceUsage:  true,
		SilenceErrors: false,
	}
	root.AddCommand(
		newInitCmd(),
		newInstallCmd(),
		newListCmd(),
		newRemoveCmd(),
		newUpgradeCmd(),
	)
	return root
}
