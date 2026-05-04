package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/momentmaker/kaijutsu/cli/internal/fetch"
	"github.com/momentmaker/kaijutsu/cli/internal/manifest"
	"github.com/spf13/cobra"
)

func newInfoCmd() *cobra.Command {
	var registryPath string
	cmd := &cobra.Command{
		Use:   "info <skill>",
		Short: "Show metadata + agent compat + signed status for a skill",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			cwd, _ := os.Getwd()
			localReg := resolveLocalRegistry(registryPath, cwd)

			var (
				l   *loaded
				err error
			)
			if localReg != "" {
				l, err = loadLocal(localReg, name)
			} else {
				m, _ := loadOrInitManifest(filepath.Join(cwd, "kaijutsu.json"), false)
				if m == nil {
					m = manifest.New([]string{"claude"})
				}
				l, err = loadRemote(cmd.Context(), cmd.ErrOrStderr(), fetch.New(), registryDefault(m), name, "")
			}
			if err != nil {
				return err
			}
			defer l.cleanup()

			printInfo(cmd.OutOrStdout(), l)
			return nil
		},
	}
	cmd.Flags().StringVar(&registryPath, "registry", "", "use a local kaijutsu monorepo checkout instead of fetching remotely")
	return cmd
}

func printInfo(out io.Writer, l *loaded) {
	sk := l.skill
	fmt.Fprintf(out, "%s@%s\n", sk.Name, sk.Version)
	fmt.Fprintf(out, "  license:     %s\n", sk.License)
	fmt.Fprintf(out, "  layout:      %s\n", sk.Layout)
	fmt.Fprintf(out, "  description: %s\n", sk.Description)
	if sk.Author != "" {
		fmt.Fprintf(out, "  author:      %s\n", sk.Author)
	}
	if sk.Homepage != "" {
		fmt.Fprintf(out, "  homepage:    %s\n", sk.Homepage)
	}
	if sk.Repository != "" {
		fmt.Fprintf(out, "  repository:  %s\n", sk.Repository)
	}
	if len(sk.Tags) > 0 {
		fmt.Fprintf(out, "  tags:        %v\n", sk.Tags)
	}
	fmt.Fprintf(out, "  agents:      %v\n", sk.Agents)
	fmt.Fprintf(out, "  source:      %s\n", l.source)
	fmt.Fprintf(out, "  ref:         %s\n", l.ref)
	if l.path != "" {
		fmt.Fprintf(out, "  path:        %s\n", l.path)
	}
	fmt.Fprintf(out, "  permissions: bash=%v network=%v fs-write=%v\n", sk.Permissions.Bash, sk.Permissions.Network, sk.Permissions.FsWrite)
	if sk.Trust != nil && sk.Trust.ExpectedSigner != "" {
		fmt.Fprintf(out, "  expected-signer: %s\n", sk.Trust.ExpectedSigner)
	}
	if l.hash != "" {
		fmt.Fprintf(out, "  integrity:   %s\n", l.hash)
	}
}
