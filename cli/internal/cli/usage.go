// usage.go — v0.16.0 `jutsu usage stats` subcommand.
//
// Reads the local usage log at ~/.kaijutsu/usage.jsonl (written by every
// jutsu invocation; see cli/internal/usage/log.go) and surfaces the
// frequency table. Local-only. The whole point: answer "what does the
// maintainer actually use this CLI for?" before shipping anything new.
//
// Per the recurring "ship zero, instrument first" finding from 6
// consecutive dream passes. Without this, every future feature proposal
// is theater.
package cli

import (
	"fmt"
	"io"
	"sort"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/momentmaker/kaijutsu/cli/internal/usage"
)

func newUsageCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "usage",
		Short: "Inspect local jutsu usage log (~/.kaijutsu/usage.jsonl)",
		Long: `Local-only command-invocation telemetry. Every jutsu run appends one
line to ~/.kaijutsu/usage.jsonl with {ts, cmd, exit, ms}. No flags, no
args, no content. Opt-out via KAIJUTSU_USAGE_LOG=0.

The data is your own: it never leaves disk, never syncs anywhere. Use it
to see which commands you actually invoke, how often, and how often they
fail. The recurring "ship zero, instrument first" dream-pass advice
exists because we couldn't answer this question without it.`,
	}
	cmd.AddCommand(newUsageStatsCmd())
	return cmd
}

func newUsageStatsCmd() *cobra.Command {
	var sinceStr string
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Frequency table of jutsu subcommand invocations",
		Long: `Reads ~/.kaijutsu/usage.jsonl and prints invocations grouped by
subcommand path. Default window: all-time. Use --since to narrow.

Empty log → "(no usage recorded yet)" + exit 0.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			entries, err := usage.Read()
			if err != nil {
				return err
			}
			window, err := parseUsageSince(sinceStr)
			if err != nil {
				return UsageError(err)
			}
			if window > 0 {
				cutoff := time.Now().UTC().Add(-window)
				kept := entries[:0]
				for _, e := range entries {
					if e.TS.After(cutoff) || e.TS.Equal(cutoff) {
						kept = append(kept, e)
					}
				}
				entries = kept
			}
			renderUsageStats(cmd.OutOrStdout(), entries, sinceStr)
			return nil
		},
	}
	cmd.Flags().StringVar(&sinceStr, "since", "", `time window (e.g. 7d, 4w, 30s, 5m, 2h). "" = all-time`)
	return cmd
}

// parseUsageSince thin-wraps findings.ParseSinceDuration so the usage
// subcommand accepts the same shorthand as `jutsu finding stats --since`.
// Defined here (not aliased) to avoid pulling the findings package into
// the cli root just for one helper.
func parseUsageSince(s string) (time.Duration, error) {
	if s == "" {
		return 0, nil
	}
	// Reuse the existing parser — already pinned by tests in the findings
	// package and accepts Nd/Nw/Nmo/Ny plus stdlib durations.
	d, err := parseUsageShorthand(s)
	if err != nil {
		return 0, err
	}
	return d, nil
}

// parseUsageShorthand mirrors findings.ParseSinceDuration without the
// findings import dep. Mirror keeps usage cleanly separable from the
// findings.db corpus.
func parseUsageShorthand(s string) (time.Duration, error) {
	type unit struct {
		suf string
		d   time.Duration
	}
	for _, u := range []unit{
		{"mo", 30 * 24 * time.Hour},
		{"d", 24 * time.Hour},
		{"w", 7 * 24 * time.Hour},
		{"y", 365 * 24 * time.Hour},
	} {
		if len(s) > len(u.suf) && s[len(s)-len(u.suf):] == u.suf {
			var n int
			if _, err := fmt.Sscanf(s, "%d", &n); err != nil {
				return 0, fmt.Errorf("usage: --since %q: bad count: %w", s, err)
			}
			if n <= 0 {
				return 0, fmt.Errorf("usage: --since %q must be positive", s)
			}
			return time.Duration(n) * u.d, nil
		}
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("usage: --since %q: %w (try 7d, 4w, 3mo, 1y, 30s, 5m, 2h)", s, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("usage: --since %q must be positive", s)
	}
	return d, nil
}

type usageRow struct {
	cmd       string
	count     int
	successes int
	failures  int
	totalMS   int64
	lastUsed  time.Time
}

func renderUsageStats(w io.Writer, entries []usage.Entry, sinceLabel string) {
	if len(entries) == 0 {
		fmt.Fprintln(w, "(no usage recorded yet — run a few jutsu commands first)")
		return
	}
	byCmd := map[string]*usageRow{}
	for _, e := range entries {
		row, ok := byCmd[e.Cmd]
		if !ok {
			row = &usageRow{cmd: e.Cmd}
			byCmd[e.Cmd] = row
		}
		row.count++
		row.totalMS += e.MS
		if e.Exit == 0 {
			row.successes++
		} else {
			row.failures++
		}
		if e.TS.After(row.lastUsed) {
			row.lastUsed = e.TS
		}
	}
	rows := make([]*usageRow, 0, len(byCmd))
	for _, r := range byCmd {
		rows = append(rows, r)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].count != rows[j].count {
			return rows[i].count > rows[j].count
		}
		return rows[i].cmd < rows[j].cmd
	})
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "CMD\tCOUNT\tOK/FAIL\tAVG_MS\tLAST")
	now := time.Now().UTC()
	for _, r := range rows {
		avg := int64(0)
		if r.count > 0 {
			avg = r.totalMS / int64(r.count)
		}
		fmt.Fprintf(tw, "%s\t%d\t%d/%d\t%d\t%s ago\n",
			r.cmd, r.count, r.successes, r.failures, avg, humanAgo(now.Sub(r.lastUsed)))
	}
	_ = tw.Flush()
	window := sinceLabel
	if window == "" {
		window = "all-time"
	}
	fmt.Fprintf(w, "\n%d cmd(s) · %d invocation(s) · window=%s\n", len(rows), len(entries), window)
}

func humanAgo(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	default:
		return fmt.Sprintf("%dmo", int(d.Hours()/(24*30)))
	}
}
