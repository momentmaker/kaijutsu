// suggest.go — `jutsu suggest <task>`: ranks installed skills by
// keyword match against a free-form task description. Helps fresh
// agents (and humans) discover which skill fits the task at hand
// without scanning `jutsu list` output.
//
// Per AGENTS.md design principle: AutoFormat (TTY → table, pipe →
// JSON). Agents shelling out get JSON; humans see a ranked list.
//
// Ranking is deliberately simple in v0.8.2 — keyword-token match
// against name + description + tags. v0.9 may swap for embedding
// similarity OR LLM-based ranking once we see what queries users +
// agents actually run.
package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/momentmaker/kaijutsu/cli/internal/skill"
)

func newSuggestCmd() *cobra.Command {
	var topN int
	cmd := &cobra.Command{
		Use:   "suggest <task description>",
		Short: "Rank skills by relevance to a task description",
		Long: `Search installed skills (skills/core/ + skills/community/) by keyword
match against a free-form task description. Returns a ranked list with
match score + skill description.

Designed for fresh AI agents discovering which skill fits the current
task without scanning the full ` + "`jutsu list`" + ` output. Compose with
` + "`jutsu install`" + ` to act on the suggestion.

Output auto-formats: TTY → table, pipe / redirect / agent capture → JSON.
` + "`--json`" + ` forces JSON regardless.

v0.8.2: keyword-rank only (token-overlap against name + description +
tags). Future v0.9.x may add embedding similarity or LLM-based ranking
once usage signal informs the right shape.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			task := strings.Join(args, " ")
			if strings.TrimSpace(task) == "" {
				return fmt.Errorf("task description required (e.g. `jutsu suggest \"review my PR\"`)")
			}

			scores := scoreSkills(task)
			if topN > 0 && len(scores) > topN {
				scores = scores[:topN]
			}

			format := AutoFormat(cmd, FormatTable)
			switch format {
			case FormatJSON:
				return renderSuggestJSON(cmd.OutOrStdout(), task, scores)
			default:
				return renderSuggestTable(cmd.OutOrStdout(), task, scores)
			}
		},
	}
	BindFormatFlags(cmd)
	cmd.Flags().IntVar(&topN, "top", 5, "max number of suggestions to return (0 = all)")
	return cmd
}

// suggestion is the ranked-output shape. Stable JSON layout for
// agent consumption; humans see the table form.
type suggestion struct {
	Name        string  `json:"name"`
	Tier        string  `json:"tier"`        // "core" | "community"
	Description string  `json:"description"`
	Score       float64 `json:"score"`       // higher = better match; rough ordinal, not normalized
	Tags        []string `json:"tags,omitempty"`
}

// scoreSkills walks skills/core/ + skills/community/, scores each
// against the task description, and returns suggestions sorted by
// score desc. v0.8.2 ships monorepo-local scan only; v0.9 may add
// remote registry scan + lockfile-aware installed-only filter.
func scoreSkills(task string) []suggestion {
	tokens := tokenize(task)
	if len(tokens) == 0 {
		return nil
	}

	out := []suggestion{}
	cwd, _ := os.Getwd()

	// Resolve the monorepo root: when invoked from the kaijutsu repo
	// itself, scan skills/core/ + skills/community/. From an arbitrary
	// project, fall back to a curated stub for now (proper remote
	// catalog lookup is a v0.9 enhancement).
	root := findMonorepoRoot(cwd)
	if root == "" {
		// No local monorepo — return empty list. v0.9 should query
		// the registry remotely. For v0.8.2, ship the local-scan
		// path; agents in a kaijutsu checkout get full value, agents
		// elsewhere get a "no suggestions" message that hints at
		// installing skills via `jutsu install <name>`.
		return nil
	}

	for _, tier := range []string{"core", "community"} {
		dir := filepath.Join(root, "skills", tier)
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			yaml := filepath.Join(dir, e.Name(), "skill.yaml")
			sk, err := skill.Load(yaml)
			if err != nil {
				continue
			}
			score := scoreSkill(sk, tokens)
			if score <= 0 {
				continue
			}
			out = append(out, suggestion{
				Name:        sk.Name,
				Tier:        tier,
				Description: sk.Description,
				Score:       score,
				Tags:        sk.Tags,
			})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		// Stable tie-break on name for deterministic output.
		return out[i].Name < out[j].Name
	})
	return out
}

// scoreSkill computes a rough match score for a skill against a task's
// token list. Hits in name carry more weight than tags than description.
// v0.8.2 ships this simple TF-style ranking; v0.9 may switch to
// embedding similarity or LLM ranking.
func scoreSkill(sk *skill.Skill, tokens []string) float64 {
	name := strings.ToLower(sk.Name)
	desc := strings.ToLower(sk.Description)
	tagBlob := strings.ToLower(strings.Join(sk.Tags, " "))

	score := 0.0
	for _, t := range tokens {
		if t == "" {
			continue
		}
		if strings.Contains(name, t) {
			score += 3.0 // name match weighted highest
		}
		if strings.Contains(tagBlob, t) {
			score += 2.0 // tag match
		}
		// Description match — count occurrences for repeated-token bonus.
		score += float64(strings.Count(desc, t))
	}
	return score
}

// tokenize splits the task description into lowercase keyword tokens.
// Strips punctuation + common stopwords. Tokens shorter than 3 chars
// dropped (avoid false positives on "to", "is", etc.).
func tokenize(task string) []string {
	stopwords := map[string]bool{
		"the": true, "and": true, "for": true, "with": true,
		"that": true, "this": true, "from": true, "into": true,
		"have": true, "has": true, "had": true, "are": true,
		"was": true, "were": true, "will": true, "would": true,
		"should": true, "could": true, "can": true, "but": true,
		"not": true, "you": true, "your": true, "want": true,
		"need": true, "how": true, "what": true, "when": true,
		"where": true, "why": true, "all": true, "any": true,
		"my": true, "me": true, "we": true, "do": true, "is": true,
		"it": true, "its": true, "to": true, "of": true, "in": true,
		"on": true, "or": true, "at": true, "be": true, "an": true,
		"if": true, "so": true, "as": true,
	}
	out := []string{}
	for _, raw := range strings.FieldsFunc(strings.ToLower(task), func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9') && r != '-'
	}) {
		t := strings.Trim(raw, "-")
		// Min length 2 — drops single-char noise like "a", "i" but
		// keeps short meaningful tokens like "pr", "go", "ci", "ai"
		// that are real skill / domain identifiers.
		if len(t) < 2 {
			continue
		}
		if stopwords[t] {
			continue
		}
		out = append(out, t)
	}
	return out
}

// findMonorepoRoot walks up from cwd looking for the kaijutsu repo
// markers (skills/core/ + cli/go.mod). Returns the root path or
// empty string. Bounded depth so we don't walk to filesystem root.
func findMonorepoRoot(start string) string {
	dir := start
	for i := 0; i < 8; i++ {
		hasSkillsCore := dirExists(filepath.Join(dir, "skills", "core"))
		hasCli := fileExists(filepath.Join(dir, "cli", "go.mod"))
		if hasSkillsCore && hasCli {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
	return ""
}

func dirExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// --- output formats ---

func renderSuggestJSON(w io.Writer, task string, scores []suggestion) error {
	payload := struct {
		SchemaVersion int          `json:"schema_version"`
		Task          string       `json:"task"`
		Count         int          `json:"count"`
		Suggestions   []suggestion `json:"suggestions"`
	}{
		SchemaVersion: 1,
		Task:          task,
		Count:         len(scores),
		Suggestions:   scores,
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(payload)
}

func renderSuggestTable(w io.Writer, task string, scores []suggestion) error {
	if len(scores) == 0 {
		fmt.Fprintf(w, "no skills match task: %q\n", task)
		fmt.Fprintln(w, "(if you're outside the kaijutsu monorepo, install skills with `jutsu install <name>` first; v0.9 will query the registry remotely)")
		return nil
	}
	fmt.Fprintf(w, "task: %s\n\n", task)
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "SCORE\tTIER\tNAME\tDESCRIPTION")
	for _, s := range scores {
		desc := s.Description
		if len(desc) > 80 {
			desc = desc[:79] + "…"
		}
		fmt.Fprintf(tw, "%.1f\t%s\t%s\t%s\n", s.Score, s.Tier, s.Name, desc)
	}
	return tw.Flush()
}
