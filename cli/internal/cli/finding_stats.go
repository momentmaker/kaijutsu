// finding_stats.go — `jutsu finding stats` v0.14.0 preset usage tracker.
//
// Reports how often each preset / persona / provider has been
// running findings over a time window. Source enrichment (built-in
// vs user) lives here, not in the findings package, so findings/
// stays free of the swarm import + the abstraction-leak the plan
// doc-review flagged.
//
// v0.13 and earlier called the precision/weight report `stats`;
// that was renamed to `precision` in v0.14.0 to free the name for
// this command.
//
// Spec: docs/specs/2026-05-09-v0.14.0-persona-browse-and-preset-usage.md
package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/momentmaker/kaijutsu/cli/internal/findings"
	"github.com/momentmaker/kaijutsu/cli/internal/swarm"
)

// defaultSinceWindow is the cobra default for `--since` when the
// flag is not passed. Matches the spec's "90-day default".
const defaultSinceWindow = "90d"

// statsRow is the cli-layer enriched row: findings.StatsRow + a
// derived Source column (only meaningful when --by preset).
type statsRow struct {
	Group  string `json:"group"`
	Count  int64  `json:"count"`
	Source string `json:"source,omitempty"` // built-in | user | unknown | "" (when --by != preset)
}

func newFindingStatsCmd() *cobra.Command {
	var (
		by            string
		since         string
		sourceFilter  string
		codebaseOverr string
		allCodebases  bool
		jsonOut       bool
	)
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Preset / persona / provider usage counts over a time window",
		Long: `Counts findings rows over the --since window, grouped by --by axis.

Defaults: 90-day window, group-by preset.

Source filter (--source) is meaningful only when --by preset:
  built-in   only counts presets currently registered as built-ins
  user       only counts user-defined presets (collapses project + home)
  all        no filter (default)

Source resolution is derived from the LIVE swarm registry at query
time; a preset that's been unregistered since its findings were
recorded surfaces as source=unknown. Distinguishing user:project vs
user:home is impossible without a schema migration; v0.14 collapses
both under "user".

The empty string ("") for --since means all-time. A bare 0 / 0d /
0s is rejected (use empty string for all-time).

Output auto-flips to JSON when stdout is piped; force with --json.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			byEnum, err := parseStatsByFlag(by)
			if err != nil {
				return err
			}
			dur, err := findings.ParseSinceDuration(since)
			if err != nil {
				return err
			}
			if err := validateSourceFilter(sourceFilter); err != nil {
				return err
			}

			store, err := openFindingsStore(cmd)
			if err != nil {
				return err
			}
			defer store.Close()

			opts := findings.StatsOpts{
				By:           byEnum,
				Since:        dur,
				AllCodebases: allCodebases,
			}
			if !allCodebases {
				fp, err := resolveCodebaseFP(codebaseOverr)
				if err != nil {
					return err
				}
				opts.CodebaseFP = fp
			}

			raw, err := findings.Stats(store, opts)
			if err != nil {
				return err
			}

			rows := enrichWithSource(raw, byEnum)
			rows = applySourceFilter(rows, sourceFilter)

			out := cmd.OutOrStdout()
			if jsonOut || !isStdoutTTY(out) {
				return renderStatsJSON(out, rows)
			}
			renderStatsTable(out, rows, byEnum, since, opts)
			return nil
		},
	}
	cmd.Flags().StringVar(&by, "by", string(findings.StatsByPreset), "group-by axis: preset | persona | provider")
	cmd.Flags().StringVar(&since, "since", defaultSinceWindow, `time window (e.g. 7d, 4w, 3mo, 1y, 30s, 5m, 2h). "" = all-time`)
	cmd.Flags().StringVar(&sourceFilter, "source", "all", "source filter (preset only): built-in | user | all")
	cmd.Flags().StringVar(&codebaseOverr, "codebase", "", "override codebase fingerprint")
	cmd.Flags().BoolVar(&allCodebases, "all-codebases", false, "stats across every recorded codebase")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "force JSON output regardless of pipe detection")
	return cmd
}

// parseStatsByFlag converts the `--by` string into the findings enum
// or returns a friendly cobra error.
func parseStatsByFlag(s string) (findings.StatsBy, error) {
	switch findings.StatsBy(s) {
	case findings.StatsByPreset, findings.StatsByPersona, findings.StatsByProvider:
		return findings.StatsBy(s), nil
	}
	return "", fmt.Errorf("--by %q invalid (want preset | persona | provider)", s)
}

// validateSourceFilter rejects unknown values up-front so the user
// sees an immediate cobra error instead of a silent zero-row table.
func validateSourceFilter(s string) error {
	switch s {
	case "all", "built-in", "user":
		return nil
	}
	return fmt.Errorf("--source %q invalid (want all | built-in | user)", s)
}

// enrichWithSource computes the source attribution for each row.
// Only meaningful when by == preset; for persona / provider axes,
// Source stays empty.
//
// Resolution rule (matches spec Decision #8):
//   - registry.Find(name) succeeds AND UserSource(name) == ""  → built-in
//   - registry.Find(name) succeeds AND UserSource(name) != ""  → user
//   - registry.Find(name) errors                                → unknown
func enrichWithSource(raw []findings.StatsRow, by findings.StatsBy) []statsRow {
	out := make([]statsRow, 0, len(raw))
	if by != findings.StatsByPreset {
		for _, r := range raw {
			out = append(out, statsRow{Group: r.Group, Count: r.Count})
		}
		return out
	}
	reg := swarm.DefaultRegistry()
	for _, r := range raw {
		row := statsRow{Group: r.Group, Count: r.Count}
		if _, err := reg.Find(r.Group); err != nil {
			row.Source = "unknown"
		} else if reg.UserSource(r.Group) != "" {
			row.Source = "user"
		} else {
			row.Source = "built-in"
		}
		out = append(out, row)
	}
	return out
}

// applySourceFilter narrows the rows by --source. "all" passes
// everything through. Filter on rows whose Source is empty (i.e.
// non-preset axes) is a no-op match: the filter is documented as
// preset-only.
func applySourceFilter(rows []statsRow, filter string) []statsRow {
	if filter == "" || filter == "all" {
		return rows
	}
	out := rows[:0]
	for _, r := range rows {
		if r.Source == "" || r.Source == filter {
			out = append(out, r)
		}
	}
	return out
}

func renderStatsJSON(out io.Writer, rows []statsRow) error {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	if rows == nil {
		rows = []statsRow{}
	}
	return enc.Encode(rows)
}

func renderStatsTable(out io.Writer, rows []statsRow, by findings.StatsBy, sinceLabel string, opts findings.StatsOpts) {
	if len(rows) == 0 {
		fmt.Fprintln(out, "(no recorded runs in window)")
		return
	}
	// Stable secondary sort: count desc preserved by SQL; tie-break
	// alphabetical for determinism in JSON / TTY.
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Count != rows[j].Count {
			return rows[i].Count > rows[j].Count
		}
		return rows[i].Group < rows[j].Group
	})
	header := []string{string(by), "count"}
	if by == findings.StatsByPreset {
		header = append(header, "source")
	}
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, joinTab(header))
	for _, r := range rows {
		if by == findings.StatsByPreset {
			fmt.Fprintf(tw, "%s\t%d\t%s\n", r.Group, r.Count, r.Source)
		} else {
			fmt.Fprintf(tw, "%s\t%d\n", r.Group, r.Count)
		}
	}
	_ = tw.Flush()
	scope := "current codebase"
	if opts.AllCodebases {
		scope = "all codebases"
	}
	window := sinceLabel
	if window == "" {
		window = "all-time"
	}
	fmt.Fprintf(out, "\n%d row(s) — window=%s, scope=%s\n", len(rows), window, scope)
}

func joinTab(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	out := parts[0]
	for _, p := range parts[1:] {
		out += "\t" + p
	}
	return out
}
