// dream.go — `jutsu dream` subcommand group. v0.8.3 ships:
//   - jutsu dream list   — show recent dream sessions in the graveyard
//   - jutsu dream clear  — delete dream sessions older than N days
//                          (mirrors `jutsu finding clear` UX: dry-run
//                          default, --yes required for destructive op)
//
// Auto-write of dream sessions into ~/.kaijutsu/dreams/ ships in
// swarm_recorder.go (called from runSwarmPipeline). The format is:
//   ~/.kaijutsu/dreams/<codebase-fp>-<topic-slug>-<YYYYMMDD-HHMMSS>.md
//   directory mode 0700, file mode 0600. Same privacy boundary as
//   findings.db.
package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

// newDreamCmd builds the `jutsu dream` group. Subcommands cover
// reading + cleaning the dream graveyard. The dream-AS-skill
// invocation lives at skills/core/dream/ (markdown). The dream-AS-
// swarm-preset invocation lives at `jutsu swarm dream`. This
// `jutsu dream` group is for managing the graveyard ARTIFACTS the
// other two paths produce.
func newDreamCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dream",
		Short: "Manage the local dream session graveyard (~/.kaijutsu/dreams/)",
		Long: `Read + clean past dream session output stored in
~/.kaijutsu/dreams/ as markdown files. Each file represents one
` + "`/dream`" + ` or ` + "`jutsu swarm dream`" + ` invocation, scoped to a
codebase fingerprint + topic slug + timestamp.

Dream session writes are automatic — the swarm dream preset writes
to the graveyard via the v0.8.3 hook. This subcommand group provides
read + cleanup operations.`,
	}
	cmd.AddCommand(
		newDreamListCmd(),
		newDreamClearCmd(),
	)
	return cmd
}

// --- list ---

func newDreamListCmd() *cobra.Command {
	var allCodebases bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List dream sessions in the graveyard for the current codebase",
		Long: `Lists dream session files in ~/.kaijutsu/dreams/ scoped to the
current cwd's codebase fingerprint by default. Pass --all-codebases
to ignore scope.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := dreamGraveyardDir()
			if err != nil {
				return err
			}
			fp, err := resolveCodebaseFP("")
			if err != nil {
				return err
			}
			entries, err := listDreamFiles(dir, fp, allCodebases)
			if err != nil {
				return err
			}
			renderDreamList(cmd.OutOrStdout(), entries, fp, allCodebases)
			return nil
		},
	}
	cmd.Flags().BoolVar(&allCodebases, "all-codebases", false, "list dreams across every recorded codebase")
	return cmd
}

// renderDreamList takes any io.Writer (cobra's OutOrStdout returns
// io.Writer; tests pass *bytes.Buffer).
func renderDreamList(w io.Writer, entries []dreamEntry, fp string, allCodebases bool) {
	if !allCodebases {
		fmt.Fprintf(w, "current codebase fp: %s\n\n", fp)
	}
	if len(entries) == 0 {
		fmt.Fprintln(w, "no dream sessions recorded yet — run `jutsu swarm dream <topic>` first")
		return
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "AGE\tCODEBASE FP\tTOPIC SLUG\tFILE")
	now := time.Now()
	for _, e := range entries {
		age := durationShort(now.Sub(e.Timestamp))
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", age, e.CodebaseFP, e.TopicSlug, filepath.Base(e.Path))
	}
	tw.Flush()
}

// --- clear ---

func newDreamClearCmd() *cobra.Command {
	var (
		olderThanS   string
		yes          bool
		dryRun       bool
		allCodebases bool
	)
	cmd := &cobra.Command{
		Use:   "clear",
		Short: "Delete dream sessions older than N days (destructive — defaults to dry-run)",
		Long: `DESTRUCTIVE: deletes dream session files from
~/.kaijutsu/dreams/.

Defaults to dry-run when neither --yes nor --dry-run is passed —
prints the count of files that WOULD be deleted and exits 0. Pass
--yes to actually delete.

Examples:
  jutsu dream clear                          # dry-run; current codebase
  jutsu dream clear --older-than 365d --yes  # delete >1yr old, current codebase
  jutsu dream clear --all-codebases --yes    # nuke everything

Mirrors the ` + "`jutsu finding clear`" + ` UX.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := dreamGraveyardDir()
			if err != nil {
				return err
			}
			fp := ""
			if !allCodebases {
				fp, err = resolveCodebaseFP("")
				if err != nil {
					return err
				}
			}
			var olderThan time.Duration
			if olderThanS != "" {
				olderThan, err = parseDurationSpec(olderThanS)
				if err != nil {
					return fmt.Errorf("--older-than %q: %w", olderThanS, err)
				}
			}

			entries, err := listDreamFiles(dir, fp, allCodebases)
			if err != nil {
				return err
			}
			cutoff := time.Now().Add(-olderThan)
			toDelete := make([]dreamEntry, 0, len(entries))
			for _, e := range entries {
				if olderThan > 0 && e.Timestamp.After(cutoff) {
					continue
				}
				toDelete = append(toDelete, e)
			}

			out := cmd.OutOrStdout()
			effectiveDry := dryRun || !yes
			if effectiveDry {
				fmt.Fprintf(out, "[dry-run] would delete %d dream session file(s); pass --yes to actually delete\n", len(toDelete))
				return nil
			}
			deleted := 0
			for _, e := range toDelete {
				if err := os.Remove(e.Path); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: %v\n", err)
					continue
				}
				deleted++
			}
			fmt.Fprintf(out, "deleted %d dream session file(s)\n", deleted)
			return nil
		},
	}
	cmd.Flags().StringVar(&olderThanS, "older-than", "", "age threshold (e.g. 90d, 12h, 30m); empty = all in scope")
	cmd.Flags().BoolVar(&yes, "yes", false, "actually delete (without this, defaults to dry-run)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print would-delete count and exit (default when --yes is unset)")
	cmd.Flags().BoolVar(&allCodebases, "all-codebases", false, "ignore cwd scope (delete across all codebases)")
	return cmd
}

// --- graveyard helpers ---

// dreamEntry is one parsed dream session file in the graveyard.
type dreamEntry struct {
	Path       string    // absolute filesystem path
	CodebaseFP string    // 16-char hex (or local-fs:hex / local-git:hex prefix variants)
	TopicSlug  string    // bounded slug from the topic input
	Timestamp  time.Time // parsed from filename suffix
}

// dreamGraveyardDir returns the canonical graveyard path. Honors
// KAIJUTSU_DREAM_DIR for tests / multi-tenant override (mirrors the
// KAIJUTSU_FINDINGS_DB pattern from v0.7).
func dreamGraveyardDir() (string, error) {
	if override := os.Getenv("KAIJUTSU_DREAM_DIR"); override != "" {
		return override, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, ".kaijutsu", "dreams"), nil
}

// listDreamFiles enumerates files under the graveyard dir, parses
// their filenames into dreamEntry records, and filters by codebase
// fp unless allCodebases is set. Returns sorted by Timestamp desc
// (newest first) — what `dream list` and `dream clear` both want.
//
// Caller-supplied fp is sanitized internally so local-fs: / local-git:
// prefixes match the underscore-form embedded in graveyard filenames.
func listDreamFiles(dir, fp string, allCodebases bool) ([]dreamEntry, error) {
	wantFP := SanitizeFP(fp)
	stat, err := os.Stat(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil // no graveyard yet — empty list, not an error
	}
	if err != nil {
		return nil, err
	}
	if !stat.IsDir() {
		return nil, fmt.Errorf("graveyard path %q is not a directory", dir)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	out := []dreamEntry{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		parsed := parseDreamFilename(e.Name())
		if parsed == nil {
			continue // unrecognized filename, skip silently
		}
		if !allCodebases && parsed.CodebaseFP != wantFP {
			continue
		}
		parsed.Path = filepath.Join(dir, e.Name())
		out = append(out, *parsed)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Timestamp.After(out[j].Timestamp)
	})
	return out, nil
}

// parseDreamFilename extracts (codebase-fp, topic-slug, timestamp)
// from a graveyard filename. Format: <fp>-<slug>-<YYYYMMDD-HHMMSS>.md
// where fp may carry a "local-fs:" or "local-git:" prefix (parse-
// time we strip the colon since it's not filesystem-safe — see
// SanitizeFP). Returns nil for unrecognized names so callers can
// skip silently.
func parseDreamFilename(name string) *dreamEntry {
	if !strings.HasSuffix(name, ".md") {
		return nil
	}
	stem := strings.TrimSuffix(name, ".md")
	// Timestamp is the trailing 15 chars: "YYYYMMDD-HHMMSS".
	// Find by scanning right-to-left for a hyphen with the right
	// shape.
	if len(stem) < 16 {
		return nil
	}
	tsCandidate := stem[len(stem)-15:]
	t, err := time.Parse("20060102-150405", tsCandidate)
	if err != nil {
		return nil
	}
	rest := stem[:len(stem)-16] // strip "-YYYYMMDD-HHMMSS"
	// rest is "<fp>-<slug>". FP forms:
	//   - 16-hex-chars (no internal hyphen, common case)
	//   - "local-fs_<16-hex>" (one internal hyphen)
	//   - "local-git_<16-hex>" (one internal hyphen)
	// Detect the local-* prefix variants explicitly; otherwise split
	// at the first hyphen.
	var fp, slug string
	switch {
	case strings.HasPrefix(rest, "local-fs_"), strings.HasPrefix(rest, "local-git_"):
		// FP = "local-{fs|git}_<16-hex>", which contains one internal
		// hyphen + an underscore. Find the FIRST hyphen AFTER the
		// underscore — that's the slug boundary.
		under := strings.Index(rest, "_")
		if under < 0 {
			return nil
		}
		nextDash := strings.Index(rest[under:], "-")
		if nextDash <= 0 {
			return nil
		}
		fp = rest[:under+nextDash]
		slug = rest[under+nextDash+1:]
	default:
		dash := strings.Index(rest, "-")
		if dash <= 0 || dash == len(rest)-1 {
			return nil
		}
		fp = rest[:dash]
		slug = rest[dash+1:]
	}
	if slug == "" {
		return nil
	}
	return &dreamEntry{
		CodebaseFP: fp,
		TopicSlug:  slug,
		Timestamp:  t,
	}
}

// SanitizeFP replaces filesystem-unsafe chars in a codebase
// fingerprint. v0.7's Fingerprint function returns either 16-hex-
// chars (the common case) or "local-fs:<hex>" / "local-git:<hex>"
// for non-git or no-remote cwds. The colon breaks filesystem paths
// on Windows + makes parsing harder; swap to underscore.
func SanitizeFP(fp string) string {
	return strings.ReplaceAll(fp, ":", "_")
}

// SlugifyTopic produces a stable, bounded, filesystem-safe slug
// from a free-form topic string. Lowercase + non-alphanumeric → -
// + collapse repeated - + trim + 50-char cap.
func SlugifyTopic(topic string) string {
	var b strings.Builder
	prevDash := true // suppress leading dash
	count := 0
	for _, r := range strings.ToLower(topic) {
		if count >= 50 {
			break
		}
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
			prevDash = false
			count++
		case r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
			count++
		default:
			if !prevDash {
				b.WriteByte('-')
				prevDash = true
				count++
			}
		}
	}
	out := strings.TrimRight(b.String(), "-")
	if out == "" {
		return "untitled"
	}
	return out
}

// WriteDreamSession writes a dream session output to the graveyard.
// File mode 0600, dir mode 0700 (created on demand). Header carries
// the lens_order so the lens-rotation rule (Stage 3 of v0.8 spec)
// can find the previous LEAD lens on repeat-dreams within 7 days.
//
// Returns the absolute path written + any error.
func WriteDreamSession(topic, codebaseFP, body string, lensOrder []string, mode string, full bool) (string, error) {
	dir, err := dreamGraveyardDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create graveyard dir: %w", err)
	}
	now := time.Now().UTC()
	fp := SanitizeFP(codebaseFP)
	slug := SlugifyTopic(topic)
	filename := fmt.Sprintf("%s-%s-%s.md", fp, slug, now.Format("20060102-150405"))
	path := filepath.Join(dir, filename)

	// Quote the topic as a YAML double-quoted scalar so colons /
	// hashes / brackets inside the topic don't break the header
	// parse downstream. Escape backslashes + double quotes per
	// YAML 1.2 double-quoted-scalar rules.
	yamlQuotedTopic := `"` +
		strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(topic, `\`, `\\`), `"`, `\"`), "\n", " ") +
		`"`
	header := fmt.Sprintf(
		"---\ntopic: %s\ncodebase_fp: %s\nlens_order: [%s]\nmode: %s\nfull: %v\ncreated_at: %s\n---\n\n",
		yamlQuotedTopic,
		codebaseFP,
		strings.Join(lensOrder, ", "),
		mode,
		full,
		now.Format(time.RFC3339),
	)
	content := header + body + "\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return "", fmt.Errorf("write dream session: %w", err)
	}
	return path, nil
}

// FindRecentDreamForTopic looks for a graveyard file matching the
// (codebase_fp, topic_slug) within the last `within` duration. Used
// by the lens-rotation rule on repeat-dreams. Returns nil if no
// match. Compares slugs case-insensitively; the slug normalization
// in SlugifyTopic should already produce stable matches but this
// belt-and-suspenders against future format edits.
func FindRecentDreamForTopic(topic, codebaseFP string, within time.Duration) (*dreamEntry, error) {
	dir, err := dreamGraveyardDir()
	if err != nil {
		return nil, err
	}
	// listDreamFiles sanitizes the fp internally so local-fs: /
	// local-git: prefixes match the underscore-form in filenames.
	entries, err := listDreamFiles(dir, codebaseFP, false)
	if err != nil {
		return nil, err
	}
	wantSlug := strings.ToLower(SlugifyTopic(topic))
	cutoff := time.Now().Add(-within)
	for _, e := range entries {
		if !strings.EqualFold(e.TopicSlug, wantSlug) {
			continue
		}
		if e.Timestamp.Before(cutoff) {
			continue
		}
		// Use a copy to return a stable pointer.
		match := e
		return &match, nil
	}
	return nil, nil
}

// durationShort renders a duration as a compact human string
// (e.g. "5d", "12h", "3m") for the dream list table. Long-tail
// precision is fine; 24h+ collapses to days.
func durationShort(d time.Duration) string {
	if d < 0 {
		d = -d
	}
	if d >= 24*time.Hour {
		days := int(d / (24 * time.Hour))
		return fmt.Sprintf("%dd", days)
	}
	if d >= time.Hour {
		return fmt.Sprintf("%dh", int(d/time.Hour))
	}
	if d >= time.Minute {
		return fmt.Sprintf("%dm", int(d/time.Minute))
	}
	return "<1m"
}
