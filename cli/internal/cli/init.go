package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/momentmaker/kaijutsu/cli/internal/detect"
	"github.com/momentmaker/kaijutsu/cli/internal/manifest"
	"github.com/momentmaker/kaijutsu/cli/internal/paths"
	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	var skipAgentsMD bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a kaijutsu project (writes kaijutsu.json + kaijutsu.lock.json + AGENTS.md fragment)",
		Long: `Bootstraps a kaijutsu project in the current directory.

Writes:
  kaijutsu.json       — project manifest (which agents to install for)
  kaijutsu.lock.json  — pinned skill versions (empty initially)
  AGENTS.md           — adds a kaijutsu-managed block teaching fresh
                        AI agents how to discover and use the jutsu CLI
                        (idempotent re-runs replace content between
                        markers; user edits outside markers preserved)

Pass --skip-agents-md to skip the AGENTS.md write (e.g. if you
manage agent docs separately).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			manifestPath := filepath.Join(cwd, paths.ManifestFile)
			lockPath := filepath.Join(cwd, paths.LockfileFile)

			if _, err := os.Stat(manifestPath); err == nil {
				return UsageError(errors.New("kaijutsu.json already exists in this directory"))
			}

			active := detect.Active()
			if len(active) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "warning: no AI agent config dirs detected (~/.claude, ~/.codex, ~/.antigravity); defaulting to [claude]")
				active = []string{"claude"}
			}

			m := manifest.New(active)
			if err := m.Save(manifestPath); err != nil {
				return err
			}
			lf := manifest.NewLockfile(active)
			if err := lf.Save(lockPath); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created kaijutsu.json with agents: %v\n", active)

			if !skipAgentsMD {
				action, ferr := writeAgentsFragment(cwd)
				if ferr != nil {
					// Don't fail the whole init — the kaijutsu.json /
					// lockfile already shipped. Surface the error so
					// user can repair AGENTS.md manually.
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: AGENTS.md fragment write failed: %v\n", ferr)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "AGENTS.md fragment %s\n", action)
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&skipAgentsMD, "skip-agents-md", false, "skip writing the kaijutsu fragment to AGENTS.md")
	return cmd
}
