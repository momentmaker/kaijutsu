// finding.go — `jutsu finding` subcommand group. Privacy boundary:
// this file MUST NOT import net, net/http, or net/url. The v0.7
// quality fingerprinting store is local-only by design; an
// import-list test in finding_test.go enforces this at build time.
package cli

import (
	"database/sql"
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
	"unicode/utf8"

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
	// --db is a persistent flag across the whole `jutsu finding`
	// group so list/accept/dismiss/stats/clear/export all honor the
	// same override without each having its own flag declaration.
	// Resolution order: --db flag > KAIJUTSU_FINDINGS_DB env > default
	// (~/.kaijutsu/findings.db).
	cmd.PersistentFlags().String("db", "", "override the findings DB path (default: $KAIJUTSU_FINDINGS_DB or ~/.kaijutsu/findings.db)")
	// --dev unlocks dev-only subcommands (e.g. seed). The flag is
	// always present in the help; the gated subcommands are Hidden,
	// so end users see nothing unusual. KAIJUTSU_FINDING_SEED=1 is
	// the equivalent env-var path used by tests + CI.
	cmd.PersistentFlags().Bool("dev", false, "unlock dev-only subcommands (seed); see KAIJUTSU_FINDING_SEED")
	cmd.AddCommand(
		newFindingListCmd(),
		newFindingAcceptCmd(),
		newFindingDismissCmd(),
		newFindingPrecisionCmd(),
		newFindingStatsCmd(),
		newFindingClearCmd(),
		newFindingExportCmd(),
		newFindingSeedCmd(),
		newFindingSyncPRCmd(),
	)
	return cmd
}

// openFindingsStore is the shared resolver for every subcommand.
// Honors --db flag (highest priority) > KAIJUTSU_FINDINGS_DB env >
// default ~/.kaijutsu/findings.db. Refuses to create the DB on a
// `jutsu finding *` invocation — the spec contracts that the DB is
// created on the first swarm run that produces findings. Without
// this guard, a curious user running `jutsu finding list` on a
// fresh system would silently leave a 0-row findings.db behind.
//
// The cobra command is passed in so the helper can read the
// inherited --db persistent flag.
func openFindingsStore(cmd *cobra.Command) (*findings.Store, error) {
	var path string
	if f := cmd.Flag("db"); f != nil {
		path = f.Value.String()
	}
	if path == "" {
		var err error
		path, err = findings.DefaultPath()
		if err != nil {
			return nil, err
		}
	}
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("no findings store at %s — run `jutsu swarm <preset>` first to create it", path)
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
			store, err := openFindingsStore(cmd)
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
	// Operate on runes, not bytes — `len(s)` and `s[:n]` count bytes,
	// which corrupts multi-byte UTF-8 characters. Summary text from
	// agents may contain CJK / emoji / diacritics; cutting at a byte
	// boundary would emit invalid UTF-8 to terminals that strict-decode.
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	out := make([]rune, 0, n)
	count := 0
	for _, r := range s {
		if count >= n-1 {
			break
		}
		out = append(out, r)
		count++
	}
	return string(out) + "…"
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
			store, err := openFindingsStore(cmd)
			if err != nil {
				return err
			}
			defer store.Close()

			row, err := findings.GetByID(store, id)
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("finding id %d not found — list ids with `jutsu finding list`", id)
			}
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

// newFindingPrecisionCmd renders the per-(provider, persona, preset)
// precision/weight report (v0.7 quality fingerprinting). v0.13 and
// earlier called this `jutsu finding stats`; v0.14 renamed to
// `precision` so that `stats` could host preset usage tracking
// without overloading semantics.
func newFindingPrecisionCmd() *cobra.Command {
	var (
		allCodebases  bool
		codebaseOverr string
	)
	cmd := &cobra.Command{
		Use:   "precision",
		Short: "Per-(provider, persona, preset) precision/weight report for current codebase",
		Long: `Renders a precision table from accept/dismiss history.
Tuples with fewer than 10 actioned findings show as "(insufficient
data, default weight: 0.7)" — matching the v0.7 bootstrap state.

Renamed from "stats" in v0.14.0; "stats" now reports preset usage
counts (run frequency) over a time window.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := openFindingsStore(cmd)
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
			renderPrecision(cmd.OutOrStdout(), stats, fp, allCodebases)
			return nil
		},
	}
	cmd.Flags().BoolVar(&allCodebases, "all-codebases", false, "report across every recorded codebase")
	cmd.Flags().StringVar(&codebaseOverr, "codebase", "", "override codebase fingerprint")
	return cmd
}

func renderPrecision(out io.Writer, stats []findings.TupleStats, fp string, allCodebases bool) {
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
			// what the synthesizer uses. Reference the canonical
			// constant so a future tuning of the floor stays in sync
			// across this rendering and the actual weight algorithm.
			w := p
			if w < findings.PrecisionFloor {
				w = findings.PrecisionFloor
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
			store, err := openFindingsStore(cmd)
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
			// --dry-run + --yes combined: dry-run wins (spec phrasing
			// is "defaults to dry-run unless --yes is set"; explicit
			// --dry-run is always honored).
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
				if errors.Is(err, findings.ErrVacuumFailed) {
					// Vacuum failed AFTER successful delete — rows are
					// gone, only disk reclaim missed. Warn + exit 0.
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: %v\n", err)
				} else {
					// Begin / Exec / Commit / RowsAffected failed —
					// rows are NOT deleted. Surface the error so the
					// shell sees a non-zero exit instead of mistaking
					// "deleted 0 rows" for success.
					return fmt.Errorf("clear: %w", err)
				}
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
			return 0, errors.New("negative duration not allowed")
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, err
	}
	// time.ParseDuration accepts "-30m" / "-1h"; for `--older-than`
	// that resolves to a future cutoff and DELETE FROM findings WHERE
	// created_at < <future> matches everything. Reject so the
	// destructive op stays an opt-in.
	if d < 0 {
		return 0, errors.New("negative duration not allowed")
	}
	return d, nil
}

// --- export ---

// formatJSON keeps the v0.7 single-object envelope; formatJSONL emits
// one row per line for stream-piping. v0.15 adds jsonl + stdout.
const (
	formatJSON  = "json"
	formatJSONL = "jsonl"
)

func newFindingExportCmd() *cobra.Command {
	var (
		codebaseOverr string
		allCodebases  bool
		since         string
		format        string
	)
	cmd := &cobra.Command{
		Use:   "export [path]",
		Short: "Write portable JSON / JSONL dump for current codebase",
		Long: `Exports findings for piping or archival. Local-only — no network calls.

Formats:
  --format json (default)  v0.7-stable single-object envelope with
                           schema_version: 1; round-trippable via future
                           import paths. Always written to a path.
  --format jsonl           one JSON object per line, no envelope. Pipe
                           to ripgrep / jq / your own LLM. Path optional;
                           omit it (or pass "-") to stream to stdout.

Filter:
  --since <duration>       only export rows with created_at >= now - X
                           (e.g. 7d, 4w, 3mo, 1y, 30s, 5m, 2h). Empty =
                           all-time. Zero / negative = error.

Examples:
  jutsu finding export /tmp/all.json                 # back-compat: v0.7 single-file JSON
  jutsu finding export --format jsonl                # stream JSONL to stdout
  jutsu finding export --format jsonl - > x.jsonl    # explicit stdout
  jutsu finding export --since 7d --format jsonl | rg severity
  jutsu finding export --all-codebases --format jsonl | jq .`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if format != formatJSON && format != formatJSONL {
				return UsageError(fmt.Errorf("--format %q invalid (want %s | %s)", format, formatJSON, formatJSONL))
			}
			dur, err := findings.ParseSinceDuration(since)
			if err != nil {
				return UsageError(err)
			}

			// Resolve output target. JSONL allows stdout (no path or "-");
			// JSON keeps the v0.7 contract of writing to a named file.
			var path string
			if len(args) == 1 {
				path = args[0]
			}
			toStdout := format == formatJSONL && (path == "" || path == "-")
			if format == formatJSON && (path == "" || path == "-") {
				return UsageError(fmt.Errorf("--format json requires a file path argument (use --format jsonl to stream to stdout)"))
			}

			store, err := openFindingsStore(cmd)
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
			rows = filterRowsSince(rows, dur)

			if toStdout {
				if err := findings.ExportJSONL(cmd.OutOrStdout(), rows); err != nil {
					return err
				}
				return nil
			}

			f, err := os.Create(path)
			if err != nil {
				return fmt.Errorf("create %s: %w", path, err)
			}
			defer f.Close()

			if format == formatJSONL {
				if err := findings.ExportJSONL(f, rows); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "exported %d row(s) → %s\n", len(rows), path)
				return nil
			}

			payload := struct {
				SchemaVersion int            `json:"schema_version"`
				ExportedAt    time.Time      `json:"exported_at"`
				CodebaseFP    string         `json:"codebase_fp,omitempty"`
				AllCodebases  bool           `json:"all_codebases,omitempty"`
				Findings      []findings.Row `json:"findings"`
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
	cmd.Flags().StringVar(&since, "since", "", `time window (e.g. 7d, 4w, 3mo, 1y, 30s, 5m, 2h). "" = all-time`)
	cmd.Flags().StringVar(&format, "format", formatJSON, "output format: json | jsonl")
	return cmd
}

// filterRowsSince drops rows whose CreatedAt is older than now-window.
// window == 0 (all-time) returns rows unchanged.
func filterRowsSince(rows []findings.Row, window time.Duration) []findings.Row {
	if window <= 0 {
		return rows
	}
	cutoff := time.Now().UTC().Add(-window)
	out := rows[:0]
	for _, r := range rows {
		if r.CreatedAt.After(cutoff) || r.CreatedAt.Equal(cutoff) {
			out = append(out, r)
		}
	}
	return out
}
