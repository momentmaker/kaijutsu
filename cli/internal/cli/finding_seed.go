// finding_seed.go — `jutsu finding seed` dev-only subcommand. v0.9
// Stage 1 affordance for weighter calibration tests + the spec's
// verification recipe step 2 (adaptive lens-weighting demo). Gated
// behind --dev flag (at `finding` group level) OR
// KAIJUTSU_FINDING_SEED=1 env-var; production CLI rejects.
package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/momentmaker/kaijutsu/cli/internal/findings"
)

// devEnvVar — when set to "1", unlocks dev-only subcommands.
const devEnvVar = "KAIJUTSU_FINDING_SEED"

// seedDevGate reports whether the dev-only `seed` subcommand is
// allowed to run. Either path opens the gate:
//   - --dev flag passed at the `finding` group level
//   - KAIJUTSU_FINDING_SEED=1 environment variable
//
// The error returned matches cobra's standard "unknown command"
// wording so production users see the same message they'd get if
// the subcommand wasn't registered at all.
func seedDevGate(cmd *cobra.Command) error {
	if os.Getenv(devEnvVar) == "1" {
		return nil
	}
	// InheritedFlags() surfaces persistent flags from every ancestor
	// in one place — covers both the direct parent's PersistentFlag
	// AND any future re-parenting that moves the flag declaration up
	// or down the cobra tree. Robust against the parent-layout shifts
	// the previous parent-walk attempted to defend against.
	if f := cmd.InheritedFlags().Lookup("dev"); f != nil && f.Value.String() == "true" {
		return nil
	}
	// Defensive: also check the local flag set in case `seed` ever
	// gains its own --dev (e.g. promoted from inherited to local).
	if f := cmd.Flags().Lookup("dev"); f != nil && f.Value.String() == "true" {
		return nil
	}
	return fmt.Errorf("unknown command \"seed\" for \"jutsu finding\"")
}

func newFindingSeedCmd() *cobra.Command {
	var (
		provider, persona, preset, codebaseFP, lens string
		accepts, dismisses                          int
	)
	cmd := &cobra.Command{
		Use:    "seed",
		Hidden: true, // omit from help — discoverable via --dev only
		// SilenceUsage prevents cobra from dumping the seed subcommand's
		// flags + Use line when seedDevGate returns the gated error —
		// otherwise the gating leaks the subcommand's existence to a
		// production user via the help dump.
		SilenceUsage: true,
		Short:        "DEV ONLY — seed synthetic actioned findings for weighter calibration testing",
		Long: `Inserts synthetic accepted/dismissed findings for the given
(provider, persona, preset, codebase, [lens]) tuple, with synthetic
run_ids prefixed "seed-<unix-ns>". Used by the v0.9 verification
recipe step 2 to bring the weighter into mature mode without running
multiple swarm dispatches.

Gated by EITHER --dev flag at the finding group level OR
KAIJUTSU_FINDING_SEED=1 env-var. Production CLI rejects with the
"unknown command" cobra error.

NOT for production use. The seeded rows are real DB writes; if you
run this against ~/.kaijutsu/findings.db you will pollute your real
weighter signal. Pass --db /tmp/test.db (or set
KAIJUTSU_FINDINGS_DB=...) to scope the writes to a throwaway file.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := seedDevGate(cmd); err != nil {
				return err
			}
			if accepts < 0 || dismisses < 0 {
				return UsageError(fmt.Errorf("--accepts and --dismisses must be >= 0"))
			}
			if provider == "" || persona == "" || preset == "" || codebaseFP == "" {
				return UsageError(fmt.Errorf("--provider, --persona, --preset, --codebase are required"))
			}

			// seed bypasses openFindingsStore's "DB must exist" guard
			// because it's the one subcommand that's allowed to create
			// the DB on first call (the gate already established this
			// is a dev/test invocation, not a production browse).
			path, err := resolveSeedDBPath(cmd)
			if err != nil {
				return err
			}
			store, err := findings.Open(path)
			if err != nil {
				return fmt.Errorf("open seed DB: %w", err)
			}
			defer store.Close()

			n, err := seedActioned(store, provider, persona, preset, codebaseFP, lens, accepts, dismisses)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "seeded %d row(s) for (%s, %s, %s, %s, lens=%q)\n",
				n, provider, persona, preset, codebaseFP, lens)
			return nil
		},
	}
	cmd.Flags().StringVar(&provider, "provider", "", "provider name (e.g. claude / codex / antigravity)")
	cmd.Flags().StringVar(&persona, "persona", "", "persona name (e.g. honest-persona)")
	cmd.Flags().StringVar(&preset, "preset", "", "preset name (e.g. dream / pr-review)")
	cmd.Flags().StringVar(&codebaseFP, "codebase", "", "codebase fingerprint")
	cmd.Flags().StringVar(&lens, "lens", "", "optional lens name (dream-only)")
	cmd.Flags().IntVar(&accepts, "accepts", 0, "number of synthetic accepted findings to insert")
	cmd.Flags().IntVar(&dismisses, "dismisses", 0, "number of synthetic dismissed findings to insert")
	return cmd
}

// resolveSeedDBPath returns the same path openFindingsStore would
// resolve, but doesn't require the file to exist — seed is the one
// subcommand that's allowed to create the DB on first call.
func resolveSeedDBPath(cmd *cobra.Command) (string, error) {
	if f := cmd.Flag("db"); f != nil && f.Value.String() != "" {
		return f.Value.String(), nil
	}
	return findings.DefaultPath()
}

// seedActioned inserts accepts + dismisses synthetic rows for the
// tuple. run_id format "seed-<unix-ns>-<i>" so the rows are
// distinguishable from real swarm runs in the audit trail. summary
// carries the lens prefix when lens != "" so backfill + recorder
// behavior stays consistent with the v0.9 schema invariants.
//
// action_at increments per-row by 1 nanosecond so the weighter's
// `ORDER BY action_at DESC LIMIT WindowSize` produces deterministic
// window selection when seed counts exceed the window or interleave
// with real action data. Without per-row monotonic action_at, SQLite
// tie-break on equal timestamps is undefined.
func seedActioned(s *findings.Store, provider, persona, preset, fp, lens string, accepts, dismisses int) (int, error) {
	base := time.Now()
	tx, err := s.DB().Begin()
	if err != nil {
		return 0, fmt.Errorf("begin: %w", err)
	}
	stmt, err := tx.Prepare(`INSERT INTO findings(
		run_id, codebase_fp, preset, provider, persona,
		severity, file, line_range, summary, lens,
		user_action, action_at
	) VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		_ = tx.Rollback()
		return 0, fmt.Errorf("prepare: %w", err)
	}
	defer stmt.Close()

	// lens="" → SQL NULL (matches recorder behavior for non-dream
	// rows). Non-empty lens → string value, and the summary carries
	// the [lens:<name>] prefix so backfill + recorder stay consistent
	// with the v0.9 schema invariants.
	summary := "seeded finding"
	var lensVal any
	if lens != "" {
		summary = "[lens:" + lens + "] seeded finding"
		lensVal = lens
	}

	written := 0
	runID := fmt.Sprintf("seed-%d", base.UnixNano())
	for i := 0; i < accepts; i++ {
		if _, err := stmt.Exec(
			fmt.Sprintf("%s-%d", runID, i), fp, preset, provider, persona,
			"info", "seed.go", "1", summary, lensVal,
			"accepted", base.Add(time.Duration(i)),
		); err != nil {
			_ = tx.Rollback()
			return 0, fmt.Errorf("insert accepted: %w", err)
		}
		written++
	}
	for i := 0; i < dismisses; i++ {
		if _, err := stmt.Exec(
			fmt.Sprintf("%s-d%d", runID, i), fp, preset, provider, persona,
			"info", "seed.go", "1", summary, lensVal,
			"dismissed", base.Add(time.Duration(accepts+i)),
		); err != nil {
			_ = tx.Rollback()
			return 0, fmt.Errorf("insert dismissed: %w", err)
		}
		written++
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit: %w", err)
	}
	return written, nil
}
