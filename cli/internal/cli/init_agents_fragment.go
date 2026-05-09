// init_agents_fragment.go — writes (or updates in place) a kaijutsu-
// managed block in AGENTS.md teaching fresh AI agents how to discover
// + use the jutsu CLI. Per AGENTS.md design principle: agents should
// be able to self-discover the kaijutsu surface without reading
// project-specific docs.
//
// Idempotent: re-running `jutsu init` (or future `jutsu init --update`)
// finds the marker block and replaces its content. User edits BETWEEN
// markers get overwritten. User edits OUTSIDE markers preserved.
package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	// agentsFragmentMarkerStart is the FULL start marker emitted on
	// write (carries current jutsu version for traceability). On
	// READ / detection we match the version-agnostic PREFIX so that
	// blocks written by an older jutsu get found + replaced cleanly
	// during upgrade — not mistaken for "corrupt state."
	agentsFragmentMarkerStart       = "<!-- kaijutsu:start name=jutsu-cli version=0.14.0 -->"
	agentsFragmentMarkerStartPrefix = "<!-- kaijutsu:start name=jutsu-cli"
	agentsFragmentMarkerEnd         = "<!-- kaijutsu:end -->"
)

// agentsFragmentBody is the content placed between the markers.
// Edits here ship to all `jutsu init` invocations from now on. Keep
// it tight — fresh agents read this once.
const agentsFragmentBody = `This project uses **kaijutsu** (` + "`jutsu`" + ` CLI) for AI-agent skills + multi-agent swarm reviews.

> NOTE: kaijutsu manages this block. Re-running ` + "`jutsu init`" + ` overwrites content BETWEEN markers. Edits OUTSIDE the markers are preserved.

## Quick reference

- ` + "`jutsu describe`" + ` — JSON catalog of every command + flag (designed for fresh agents to ingest at session start)
- ` + "`jutsu suggest \"<task>\"`" + ` — keyword-rank installed skills against a task description (` + "`--json`" + ` for machine output)
- ` + "`jutsu list`" + ` — installed skills
- ` + "`jutsu install <skill>`" + ` / ` + "`jutsu remove <skill>`" + ` — manage skills
- ` + "`jutsu swarm <preset> --help`" + ` — multi-agent presets (pr-review, doc-review, brainstorm, refactor-plan, security-audit, dream)
- ` + "`jutsu finding list`" + ` — past swarm findings + accept/dismiss workflow

## When to use kaijutsu

When a task maps to an installed skill, prefer the skill over inventing your own approach. Common mappings:
- PR review → ` + "`pr-review`" + `
- Idea exploration / "should we build X" → ` + "`dream`" + `
- Stuck on a bug → ` + "`unstuck`" + `
- Multi-agent QA on a markdown artifact → ` + "`doc-review`" + `

Agent-first design: ` + "`jutsu`" + ` outputs JSON when stdout is a pipe (you, when shelling out) and pretty markdown/table when it's a TTY (humans). No flag needed for the auto-flip; ` + "`--json`" + ` forces JSON.

(See [AGENTS.md design principle](https://github.com/momentmaker/kaijutsu/blob/main/AGENTS.md#design-principle-agent-first-human-friendly) for the full agent-first lens.)

Skills compose via ` + "`deps.skills`" + `. Run ` + "`jutsu info <skill>`" + ` for full metadata.`

// writeAgentsFragment idempotently writes or updates the kaijutsu
// fragment in `<cwd>/AGENTS.md`. Returns the action taken (one of:
// "created", "appended", "replaced", "unchanged") and any error.
//
// Behavior:
//   - File missing             → create with fragment + trailing newline
//   - Markers present          → replace content between markers
//   - Markers absent           → append fresh block separated by blank line
//   - Only one marker (corrupt) → error with repair hint (refuse to write)
func writeAgentsFragment(cwd string) (string, error) {
	path := filepath.Join(cwd, "AGENTS.md")

	existing, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		// Fresh file — create with just the fragment.
		body := wrapFragment()
		if err := os.WriteFile(path, []byte(body+"\n"), 0o644); err != nil {
			return "", fmt.Errorf("create AGENTS.md: %w", err)
		}
		return "created", nil
	}
	if err != nil {
		return "", fmt.Errorf("read AGENTS.md: %w", err)
	}

	content := string(existing)
	// Match by version-AGNOSTIC prefix so blocks from older jutsu
	// versions get found + replaced (the version pin is metadata,
	// not the matching contract).
	startIdx := strings.Index(content, agentsFragmentMarkerStartPrefix)
	endIdx := strings.Index(content, agentsFragmentMarkerEnd)

	if startIdx == -1 && endIdx == -1 {
		// No markers — append fresh block at end.
		separator := "\n\n"
		if strings.HasSuffix(content, "\n\n") {
			separator = ""
		} else if strings.HasSuffix(content, "\n") {
			separator = "\n"
		}
		updated := content + separator + wrapFragment() + "\n"
		if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
			return "", fmt.Errorf("append to AGENTS.md: %w", err)
		}
		return "appended", nil
	}

	if startIdx == -1 || endIdx == -1 {
		// Corrupt state — refuse to touch the file.
		return "", fmt.Errorf("AGENTS.md has only one of the kaijutsu marker tags (%q / %q). Repair manually or delete both markers and re-run `jutsu init`",
			agentsFragmentMarkerStart, agentsFragmentMarkerEnd)
	}

	if endIdx < startIdx {
		return "", fmt.Errorf("AGENTS.md kaijutsu markers are out of order (end before start). Repair manually")
	}

	// Replace content between markers (inclusive of the markers
	// themselves so the version pin in the start tag stays current).
	endIdxAbs := endIdx + len(agentsFragmentMarkerEnd)
	updated := content[:startIdx] + wrapFragment() + content[endIdxAbs:]

	if updated == content {
		return "unchanged", nil
	}
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		return "", fmt.Errorf("update AGENTS.md: %w", err)
	}
	return "replaced", nil
}

// wrapFragment assembles the marker pair around the fragment body.
// Centralized so the marker tags + body live in lockstep.
func wrapFragment() string {
	return agentsFragmentMarkerStart + "\n" + agentsFragmentBody + "\n" + agentsFragmentMarkerEnd
}
