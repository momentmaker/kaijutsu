// finding.go — `jutsu finding` subcommand group. Privacy boundary:
// this file MUST NOT import net, net/http, or net/url. The v0.7
// quality fingerprinting store is local-only by design; an
// import-list test in finding_test.go enforces this at build time.
package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/momentmaker/kaijutsu/cli/internal/findings"
)

// newFindingCmd builds the `jutsu finding` group covering the v0.7
// quality fingerprinting CLI surface: list, accept, dismiss, stats,
// clear, export.
func newFindingCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "finding",
		Short: "List, action, and inspect swarm findings (v0.7 quality fingerprinting)",
		Long: `Quality fingerprinting captures every swarm finding into a local
SQLite store at ~/.kaijutsu/findings.db. Accepting or dismissing a
finding feeds per-(provider, persona, preset, codebase) precision
math that the synthesizer uses to weight future swarm runs.

All commands operate on the resolved codebase fingerprint of the
current cwd unless --codebase is passed. Local-only by design — no
network calls.`,
	}
	cmd.AddCommand(
		newFindingListCmd(),
		newFindingAcceptCmd(),
		newFindingDismissCmd(),
		newFindingStatsCmd(),
		newFindingClearCmd(),
		newFindingExportCmd(),
	)
	return cmd
}

// openFindingsStore is the shared resolver for every subcommand. It
// honors KAIJUTSU_FINDINGS_DB so tests can scope to a temp file.
// Returns a clear error when the DB doesn't exist yet (ran no swarm
// runs) so the user gets actionable feedback instead of an SQLite
// "no such table" leak.
func openFindingsStore() (*findings.Store, error) {
	path, err := findings.DefaultPath()
	if err != nil {
		return nil, err
	}
	return findings.Open(path)
}

// resolveCodebaseFP returns the cwd's fingerprint unless --codebase
// override is set. Used by every subcommand that defaults to the
// current codebase.
func resolveCodebaseFP(override string) (string, error) {
	if override != "" {
		return override, nil
	}
	cwd, err := projectRoot()
	if err != nil {
		return "", err
	}
	return findings.Fingerprint(cwd), nil
}

// --- list ---

func newFindingListCmd() *cobra.Command {
	var (
		runID         string
		pendingOnly   bool
		allCodebases  bool
		codebaseOverr string
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List findings for the current codebase (or a specific run)",
		Long: `Defaults to the most recent swarm run for the current codebase.
--run <id>            scope to one swarm cache key
--pending             only unactioned findings
--all-codebases       ignore cwd scope (every codebase ever recorded)
--codebase <fp>       override the auto-detected codebase fingerprint`,
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := openFindingsStore()
			if err != nil {
				return err
			}
			defer store.Close()

			fp, err := resolveCodebaseFP(codebaseOverr)
			if err != nil {
				return err
			}
			rows, err := findings.ListFindings(store, findings.ListOpts{
				CodebaseFP:   fp,
				RunID:        runID,
				PendingOnly:  pendingOnly,
				AllCodebases: allCodebases,
			})
			if err != nil {
				return err
			}
			renderFindingList(cmd.OutOrStdout(), rows, fp, allCodebases)
			return nil
		},
	}
	cmd.Flags().StringVar(&runID, "run", "", "filter by swarm run cache key")
	cmd.Flags().BoolVar(&pendingOnly, "pending", false, "only show unactioned findings")
	cmd.Flags().BoolVar(&allCodebases, "all-codebases", false, "ignore current cwd's codebase scope")
	cmd.Flags().StringVar(&codebaseOverr, "codebase", "", "override codebase fingerprint")
	return cmd
}

func renderFindingList(out io.Writer, rows []findings.Row, fp string, allCodebases bool) {
	if !allCodebases {
		fmt.Fprintf(out, "current codebase fp: %s\n\n", fp)
	}
	if len(rows) == 0 {
		fmt.Fprintln(out, "no findings recorded yet — run `jutsu swarm <preset>` first")
		return
	}
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tSTATE\tSEVERITY\tPROVIDER\tPERSONA\tFILE:LINES\tSUMMARY")
	for _, r := range rows {
		state := "pending"
		if r.UserAction != nil {
			state = *r.UserAction
		}
		fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\t%s:%s\t%s\n",
			r.ID, state, r.Severity, r.Provider, r.Persona,
			r.File, r.LineRange, truncate(r.Summary, 80))
	}
	tw.Flush()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

// --- accept / dismiss ---

func newFindingAcceptCmd() *cobra.Command  { return newFindingActionCmd("accept", "accepted") }
func newFindingDismissCmd() *cobra.Command { return newFindingActionCmd("dismiss", "dismissed") }

func newFindingActionCmd(verb, action string) *cobra.Command {
	var (
		reason         string
		crossCodebase  bool
		codebaseOverr  string
	)
	cmd := &cobra.Command{
		Use:   verb + " <id>",
		Short: fmt.Sprintf("Mark finding <id> as %s", action),
		Long: fmt.Sprintf(`Records the user's %s decision against finding <id>. The id is the
leftmost column of jutsu finding list.

Cross-codebase guard: refuses when the finding's codebase doesn't
match the cwd's resolved fp; pass --cross-codebase to override (use
when batch-actioning across repos from a single shell session).`, action),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("id %q: must be an integer (see leftmost column of `jutsu finding list`)", args[0])
			}
			store, err := openFindingsStore()
			if err != nil {
				return err
			}
			defer store.Close()

			row, err := findings.GetByID(store, id)
			if err != nil {
				return fmt.Errorf("finding id %d: %w", id, err)
			}
			if !crossCodebase {
				cwdFP, err := resolveCodebaseFP(codebaseOverr)
				if err != nil {
					return err
				}
				if row.CodebaseFP != cwdFP {
					return fmt.Errorf(
						"finding %d belongs to codebase %s but cwd resolves to %s — pass --cross-codebase to override",
						id, row.CodebaseFP, cwdFP,
					)
				}
			}
			if err := findings.SetAction(store, id, action, reason); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s finding %d (%s)\n", action, id, row.Persona)
			return nil
		},
	}
	cmd.Flags().StringVar(&reason, "reason", "", "optional rationale stored with the action")
	cmd.Flags().BoolVar(&crossCodebase, "cross-codebase", false, "skip cwd-codebase guard")
	cmd.Flags().StringVar(&codebaseOverr, "codebase", "", "override codebase fingerprint")
	return cmd
}

// --- stats ---

func newFindingStatsCmd() *cobra.Command {
	var (
		allCodebases  bool
		codebaseOverr string
	)
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Per-(provider, persona, preset) precision report for current codebase",
		Long: `Renders a precision table from accept/dismiss history.
Tuples with fewer than 10 actioned findings show as "(insufficient
data, default weight: 0.7)" — matching the v0.7 bootstrap state.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := openFindingsStore()
			if err != nil {
				return err
			}
			defer store.Close()

			fp, err := resolveCodebaseFP(codebaseOverr)
			if err != nil {
				return err
			}
			stats, err := findings.StatsByTuple(store, fp, allCodebases)
			if err != nil {
				return err
			}
			renderStats(cmd.OutOrStdout(), stats, fp, allCodebases)
			return nil
		},
	}
	cmd.Flags().BoolVar(&allCodebases, "all-codebases", false, "stats across every recorded codebase")
	cmd.Flags().StringVar(&codebaseOverr, "codebase", "", "override codebase fingerprint")
	return cmd
}

func renderStats(out io.Writer, stats []findings.TupleStats, fp string, allCodebases bool) {
	if !allCodebases {
		fmt.Fprintf(out, "current codebase fp: %s\n\n", fp)
	}
	if len(stats) == 0 {
		fmt.Fprintln(out, "no findings recorded yet — run `jutsu swarm <preset>` first")
		return
	}
	// Sort: precision desc, but bootstrap tuples (Actioned < 10) drop
	// to the bottom so the user's eyes land on the meaningful rows.
	sort.SliceStable(stats, func(i, j int) bool {
		ai, aj := stats[i].Actioned() >= 10, stats[j].Actioned() >= 10
		if ai != aj {
			return ai
		}
		return stats[i].Precision() > stats[j].Precision()
	})
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "PROVIDER\tPERSONA\tPRESET\tPRECISION\tWEIGHT\tACCEPT/DISMISS\tPENDING")
	for _, t := range stats {
		actioned := t.Actioned()
		precision := "—"
		weight := "—"
		switch {
		case actioned == 0:
			precision = "(no data)"
			weight = "1.00 (cold)"
		case actioned < 10:
			precision = "(insufficient data)"
			weight = "0.70 (bootstrap)"
		default:
			p := t.Precision()
			precision = fmt.Sprintf("%.2f", p)
			// Mirror Weighter's clamp-to-floor — final weight matches
			// what synthesizer will use in Stage 4.
			w := p
			if w < 0.05 {
				w = 0.05
			}
			weight = fmt.Sprintf("%.2f (mature)", w)
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%d/%d\t%d\n",
			t.Provider, t.Persona, t.Preset, precision, weight,
			t.Accepted, t.Dismissed, t.Pending)
	}
	tw.Flush()
}

// --- clear ---

func newFindingClearCmd() *cobra.Command {
	var (
		olderThanS    string
		yes           bool
		dryRun        bool
		codebaseOverr string
		allCodebases  bool
	)
	cmd := &cobra.Command{
		Use:   "clear",
		Short: "Delete findings for current codebase (destructive — defaults to dry-run)",
		Long: `DESTRUCTIVE: deletes findings rows from the local store.

Defaults to dry-run when neither --yes nor --dry-run is passed —
prints the count of rows that WOULD be deleted and exits 0 without
mutating. Pass --yes to actually delete.

Examples:
  jutsu finding clear                         # dry-run; current codebase only
  jutsu finding clear --older-than 365d --yes # delete rows older than 1 year
  jutsu finding clear --all-codebases --yes   # nuke everything

Runs VACUUM after deletion to reclaim disk.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := openFindingsStore()
			if err != nil {
				return err
			}
			defer store.Close()

			opts := findings.ClearOpts{}
			if !allCodebases {
				fp, err := resolveCodebaseFP(codebaseOverr)
				if err != nil {
					return err
				}
				opts.CodebaseFP = fp
			}
			if olderThanS != "" {
				d, err := parseDurationSpec(olderThanS)
				if err != nil {
					return fmt.Errorf("--older-than %q: %w", olderThanS, err)
				}
				opts.OlderThan = d
			}

			out := cmd.OutOrStdout()
			// Default to dry-run when neither flag is passed — explicit
			// affirmative required for destructive ops per spec.
			effectiveDry := dryRun || !yes
			if effectiveDry {
				n, err := findings.CountClearable(store, opts)
				if err != nil {
					return err
				}
				fmt.Fprintf(out, "[dry-run] would delete %d row(s); pass --yes to actually delete\n", n)
				return nil
			}

			n, err := findings.Clear(store, opts)
			if err != nil {
				// Clear returns count + error when VACUUM fails after
				// successful delete — surface both pieces of info.
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %v\n", err)
			}
			fmt.Fprintf(out, "deleted %d row(s)\n", n)
			return nil
		},
	}
	cmd.Flags().StringVar(&olderThanS, "older-than", "", "age threshold (e.g. 90d, 12h, 30m)")
	cmd.Flags().BoolVar(&yes, "yes", false, "actually delete (without this, defaults to dry-run)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print would-delete count and exit (default when --yes is unset)")
	cmd.Flags().StringVar(&codebaseOverr, "codebase", "", "override codebase fingerprint")
	cmd.Flags().BoolVar(&allCodebases, "all-codebases", false, "ignore cwd scope (delete across all codebases)")
	return cmd
}

// parseDurationSpec accepts Go duration literals (24h, 30m) plus a
// "Nd" days extension since the spec's example flag --older-than 90d
// is days-shaped. Avoids dragging in a heavier parser.
func parseDurationSpec(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, errors.New("empty")
	}
	if strings.HasSuffix(s, "d") {
		days, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err != nil {
			return 0, fmt.Errorf("days prefix invalid: %w", err)
		}
		if days < 0 {
			return 0, errors.New("negative days")
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}

// --- export ---

func newFindingExportCmd() *cobra.Command {
	var (
		codebaseOverr string
		allCodebases  bool
	)
	cmd := &cobra.Command{
		Use:   "export <path>",
		Short: "Write portable JSON dump for current codebase",
		Long: `Writes a portable JSON file with top-level schema_version: 1 so
future jutsu versions can import the data. Local file write only —
this command does NOT make network calls.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := args[0]
			store, err := openFindingsStore()
			if err != nil {
				return err
			}
			defer store.Close()

			fp, err := resolveCodebaseFP(codebaseOverr)
			if err != nil {
				return err
			}
			rows, err := findings.AllForCodebase(store, fp, allCodebases)
			if err != nil {
				return err
			}
			payload := struct {
				SchemaVersion int             `json:"schema_version"`
				ExportedAt    time.Time       `json:"exported_at"`
				CodebaseFP    string          `json:"codebase_fp,omitempty"`
				AllCodebases  bool            `json:"all_codebases,omitempty"`
				Findings      []findings.Row  `json:"findings"`
			}{
				SchemaVersion: 1,
				ExportedAt:    time.Now().UTC(),
				CodebaseFP:    fp,
				AllCodebases:  allCodebases,
				Findings:      rows,
			}
			if allCodebases {
				payload.CodebaseFP = ""
			}
			f, err := os.Create(path)
			if err != nil {
				return fmt.Errorf("create %s: %w", path, err)
			}
			defer f.Close()
			enc := json.NewEncoder(f)
			enc.SetIndent("", "  ")
			if err := enc.Encode(payload); err != nil {
				return fmt.Errorf("encode: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "exported %d row(s) → %s\n", len(rows), path)
			return nil
		},
	}
	cmd.Flags().StringVar(&codebaseOverr, "codebase", "", "override codebase fingerprint")
	cmd.Flags().BoolVar(&allCodebases, "all-codebases", false, "export every recorded codebase")
	return cmd
}
