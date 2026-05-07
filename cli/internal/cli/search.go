package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/momentmaker/kaijutsu/cli/internal/fetch"
	"github.com/momentmaker/kaijutsu/cli/internal/manifest"
	"github.com/momentmaker/kaijutsu/cli/internal/registry"
	"github.com/momentmaker/kaijutsu/cli/internal/skill"
	"github.com/momentmaker/kaijutsu/cli/internal/source"
	"github.com/spf13/cobra"
)

func newSearchCmd() *cobra.Command {
	var registryPath string
	var full bool
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search the registry by name, description, or tags",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.ToLower(args[0])
			cwd, _ := os.Getwd()
			localReg := resolveLocalRegistry(registryPath, cwd)

			out := cmd.OutOrStdout()
			matches := []searchHit{}
			if localReg != "" {
				matches = append(matches, searchLocal(localReg, query)...)
			} else {
				m, _ := loadOrInitManifest(filepath.Join(cwd, "kaijutsu.json"), false)
				if m == nil {
					m = manifest.New([]string{"claude"})
				}
				remoteHits, err := searchRemote(cmd.Context(), m, query)
				if err != nil {
					return err
				}
				matches = append(matches, remoteHits...)
			}

			if len(matches) == 0 {
				fmt.Fprintln(out, "no skills match")
				return nil
			}
			sort.Slice(matches, func(i, j int) bool { return matches[i].name < matches[j].name })
			// v0.9.1 readability fix: truncate description to 80 chars
			// for the table view; add a footer with match count + hint
			// for full descriptions via `jutsu info <name>`. The 80-char
			// cap keeps `jutsu search "."` (matches everything) from
			// scrolling off the screen with multi-sentence
			// descriptions. Pass --full to suppress truncation.
			descCap := 80
			if full {
				descCap = 0 // 0 disables truncation
			}
			for _, m := range matches {
				desc := m.description
				if descCap > 0 && len(desc) > descCap {
					desc = desc[:descCap-1] + "…"
				}
				fmt.Fprintf(out, "%-25s  %s\n", m.name, desc)
			}
			fmt.Fprintf(out, "\n%d skill(s) match. `jutsu info <name>` for full description.\n", len(matches))
			return nil
		},
	}
	cmd.Flags().StringVar(&registryPath, "registry", "", "search a local kaijutsu monorepo checkout")
	cmd.Flags().BoolVar(&full, "full", false, "show full descriptions (no truncation)")
	return cmd
}

type searchHit struct {
	name        string
	description string
	source      string
}

func searchLocal(registryPath, query string) []searchHit {
	out := []searchHit{}
	root := filepath.Join(registryPath, "skills", "core")
	entries, err := os.ReadDir(root)
	if err != nil {
		return out
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		yamlPath := filepath.Join(root, e.Name(), "skill.yaml")
		sk, err := skill.Load(yamlPath)
		if err != nil {
			continue
		}
		if matches(sk, query) {
			out = append(out, searchHit{name: sk.Name, description: sk.Description, source: "local"})
		}
	}
	return out
}

func searchRemote(ctx context.Context, m *manifest.Manifest, query string) ([]searchHit, error) {
	defReg, err := source.Parse(registryDefault(m))
	if err != nil {
		return nil, err
	}
	fetcher := fetch.New()
	sha, err := fetcher.DefaultBranchSHA(ctx, defReg)
	if err != nil {
		return nil, fmt.Errorf("resolve default registry: %w", err)
	}

	out := []searchHit{}

	// Search core skills via tarball — single fetch, parse all skill.yaml.
	data, _, err := fetcher.Tarball(ctx, defReg, sha)
	if err != nil {
		return nil, err
	}
	tmp, err := os.MkdirTemp("", "kaijutsu-search-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	top, err := fetch.Extract(data, tmp)
	if err != nil {
		return nil, err
	}
	hits := searchLocal(top, query)
	for i := range hits {
		hits[i].source = defReg.String()
	}
	out = append(out, hits...)

	// Search third-party index entries by name match against the index itself.
	body, err := fetcher.GetFile(ctx, defReg, sha, "registry/index.json")
	if err == nil {
		idx, ierr := registry.LoadFromBytes(body)
		if ierr == nil {
			for name, entry := range idx.Skills {
				if strings.Contains(strings.ToLower(name), query) {
					out = append(out, searchHit{name: name, description: "(third-party — run `jutsu info` for details)", source: entry.Source})
				}
			}
		}
	}
	return out, nil
}

func matches(sk *skill.Skill, q string) bool {
	if strings.Contains(strings.ToLower(sk.Name), q) {
		return true
	}
	if strings.Contains(strings.ToLower(sk.Description), q) {
		return true
	}
	for _, t := range sk.Tags {
		if strings.Contains(strings.ToLower(t), q) {
			return true
		}
	}
	return false
}
