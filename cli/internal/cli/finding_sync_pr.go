// finding_sync_pr.go — `jutsu finding sync-pr <pr>` v0.9 subcommand.
//
// Ingests human accept/dismiss decisions from PR comment replies
// back into the local findings DB. v0.9 ships ONLY the reply-
// keyword channel (`accept: <run_id>:<pos>` / `dismiss: ...` lines
// in PR replies). The reaction channel + per-finding line-anchored
// review comments are deferred to v0.9.x.
//
// Default is dry-run: prints the diff of "would mark these
// findings as X" and exits 0 without writing. --apply does the
// real writes. Mirrors `jutsu finding clear` UX.
package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/momentmaker/kaijutsu/cli/internal/findings"
	"github.com/momentmaker/kaijutsu/cli/internal/swarm"
)

func newFindingSyncPRCmd() *cobra.Command {
	var apply bool
	cmd := &cobra.Command{
		Use:   "sync-pr <pr>",
		Short: "Ingest accept/dismiss decisions from PR comment replies into the findings DB (v0.9, reply-keyword channel only)",
		Long: `Reads PR comments via gh, parses replies for accept/dismiss
keywords, and records the actions in the local findings DB.

Grammar — each action line in a reply matches:
  ^[\s>]*(accept|dismiss)\s*:\s*<run_id>:<position>\s*$

Multiple action lines per reply are supported. Run-id scoping rules
(in priority order):
  1. Threaded reply anchored at a kaijutsu-pr-review marker comment
  2. Plain comment with explicit "run-id=<id>" or quoted marker
  3. Otherwise: ignored

Defaults to DRY-RUN. Prints the would-write diff and exits 0
without mutating the DB. Pass --apply to actually record actions.

v0.9 limitation: reply-keyword channel only. The reaction channel
(per-finding line-anchored review comments + GitHub reactions) is a
v0.9.x follow-up — needs the --post-review render mode plumbed.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pr, err := strconv.Atoi(args[0])
			if err != nil || pr <= 0 {
				return UsageError(fmt.Errorf("pr arg %q: must be a positive integer", args[0]))
			}
			ctx := cmd.Context()
			store, err := openFindingsStore(cmd)
			if err != nil {
				return err
			}
			defer store.Close()

			actions, err := collectSyncPRActions(ctx, pr)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			stderr := cmd.ErrOrStderr()
			if len(actions) == 0 {
				fmt.Fprintf(out, "no scoped action lines found in PR #%d's comments\n", pr)
				return nil
			}
			return applySyncPRActions(out, stderr, store, actions, apply)
		},
	}
	cmd.Flags().BoolVar(&apply, "apply", false, "actually write actions to the findings DB (default: dry-run)")
	return cmd
}

// collectSyncPRActions fetches PR issue-comments via gh and walks
// each looking for scoped action lines. v0.9 only honors priority-2
// scoping (explicit run-id reference in the comment body) — not
// priority-1 (threaded reply anchored at a marker) because issue
// comments don't carry in_reply_to_id. The action lines themselves
// already encode the run-id (`accept: <run_id>:<pos>`), so the
// scoping question is mostly redundant for v0.9 — but we still
// require some scope signal in the body to filter out comments that
// happen to contain words matching the action regex by accident.
//
// v0.9.x with --post-review will use line-anchored review comments
// (those DO carry in_reply_to_id) and re-introduce priority-1.
func collectSyncPRActions(ctx context.Context, pr int) ([]swarm.SyncPRAction, error) {
	// Bound the gh subprocess at 30s so a hung gh call can't block
	// CI/gate pipelines indefinitely. Cobra's base context is
	// uncanceled by default — without this, network-stalled gh
	// blocks forever.
	ghCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ghCtx, "gh", "pr", "view", strconv.Itoa(pr), "--json", "comments").Output()
	if err != nil {
		return nil, fmt.Errorf("gh pr view %d: %w", pr, err)
	}
	var v struct {
		Comments []struct {
			Body string `json:"body"`
		} `json:"comments"`
	}
	if err := json.Unmarshal(out, &v); err != nil {
		return nil, fmt.Errorf("parse gh comments: %w", err)
	}
	var actions []swarm.SyncPRAction
	for _, c := range v.Comments {
		// Skip comments without action lines. ParseReplyKeywords is
		// case-insensitive ((?i)), so this also implicitly skips the
		// bot's own marker comments (which never contain accept/
		// dismiss action lines). Cleaner than a separate string-
		// match filter that could disagree with the regex on case.
		parsed := swarm.ParseReplyKeywords(c.Body)
		if len(parsed) == 0 {
			continue
		}
		// Priority-2 scoping: comment body must carry an explicit
		// run-id reference. Without it, drop (priority-3).
		if swarm.IsScopedReply(c.Body, "") == "" {
			continue
		}
		// Action lines already include the run-id per the grammar
		// (`accept: <run_id>:<pos>`); the body-scope check above
		// just filters out unscoped noise. Each action carries its
		// own run-id, so no extra fallback assignment is needed.
		actions = append(actions, parsed...)
	}
	return actions, nil
}

// applySyncPRActions resolves each action to a finding row and
// either prints the would-write diff (dry-run) or writes via
// findings.SetAction (--apply). Idempotent: actions that match the
// current row state are no-ops; conflicts emit a [changed] log.
func applySyncPRActions(out, stderr io.Writer, store *findings.Store, actions []swarm.SyncPRAction, apply bool) error {
	header := "[dry-run] would record:"
	if apply {
		header = "applied:"
	}
	fmt.Fprintln(out, header)

	written, skipped, conflicts := 0, 0, 0
	for _, a := range actions {
		row, err := findings.FindByRunPosition(store, a.RunID, a.Position)
		if errors.Is(err, sql.ErrNoRows) {
			fmt.Fprintf(stderr, "  skip: %s — no row for (run_id=%s, position=%d)\n", a.Raw, a.RunID, a.Position)
			skipped++
			continue
		}
		if err != nil {
			return fmt.Errorf("lookup (%s, %d): %w", a.RunID, a.Position, err)
		}
		// Detect idempotent + conflict cases.
		current := ""
		if row.UserAction != nil {
			current = *row.UserAction
		}
		if current == a.Verb {
			fmt.Fprintf(out, "  noop: id=%d already %s\n", row.ID, a.Verb)
			continue
		}
		if current != "" && current != a.Verb {
			conflicts++
			fmt.Fprintf(out, "  [changed] id=%d %s → %s (sync-pr overrides prior CLI action)\n", row.ID, current, a.Verb)
		} else {
			fmt.Fprintf(out, "  %s id=%d (run_id=%s, position=%d)\n", a.Verb, row.ID, a.RunID, a.Position)
		}
		if apply {
			if err := findings.SetAction(store, row.ID, a.Verb, "synced from PR reply"); err != nil {
				return fmt.Errorf("setAction id=%d: %w", row.ID, err)
			}
			written++
		}
	}
	if apply {
		fmt.Fprintf(out, "\nwrote %d action(s); %d skipped; %d conflict(s) overridden.\n", written, skipped, conflicts)
	} else {
		fmt.Fprintf(out, "\n[dry-run] %d action(s) would write; %d skipped; %d conflict(s). Re-run with --apply.\n", len(actions)-skipped, skipped, conflicts)
	}
	return nil
}
