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
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize a kaijutsu project (writes kaijutsu.json + kaijutsu.lock.json)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			manifestPath := filepath.Join(cwd, paths.ManifestFile)
			lockPath := filepath.Join(cwd, paths.LockfileFile)

			if _, err := os.Stat(manifestPath); err == nil {
				return errors.New("kaijutsu.json already exists in this directory")
			}

			active := detect.Active()
			if len(active) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "warning: no AI agent config dirs detected (~/.claude, ~/.codex, ~/.gemini); defaulting to [claude]")
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
			return nil
		},
	}
}
