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

	"github.com/momentmaker/kaijutsu/cli/internal/findings"
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
			window, err := findings.ParseSinceDuration(sinceStr)
			if err != nil {
				return UsageError(err)
			}
			if window > 0 {
				cutoff := time.Now().UTC().Add(-window)
				// Allocate a fresh slice rather than reusing the input
				// backing array via `entries[:0]`. Safe today (Read
				// returns owned memory) but rules out silent breakage
				// if Read ever returns cached / shared data.
				kept := make([]usage.Entry, 0, len(entries))
				for _, e := range entries {
					// `!Before(cutoff)` is the idiomatic single-call
					// form for "at or after cutoff".
					if !e.TS.Before(cutoff) {
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
