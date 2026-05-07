// sync_pr.go — v0.9 PR-comment auto-detection: ingest human
// accept/dismiss decisions from GitHub comments back into the
// findings DB.
//
// v0.9 ships the REPLY-KEYWORD channel only. The reaction-channel
// (per-finding line-anchored review comments + emoji reactions) is
// deferred to v0.9.x — needs the --post-review render mode + the
// reaction GET endpoint plumbed.
//
// Grammar (regex-formal):
//
//	^[\s>]*(accept|dismiss)\s*:\s*([A-Za-z0-9]+:[0-9]+)\s*$
//
// Notes:
//   - case-insensitive on the action verb
//   - leading whitespace OR `>`-quoting tolerated
//   - one id per line; multiple action lines per reply OK
//   - id format: <run_id>:<position>
//   - whitespace around the colon between verb and id tolerated
//   - run-id grammar `[A-Za-z0-9]+` MATCHES the v0.9 timestamp
//     format (NewRunID returns "20060102T150405Z"). Future
//     hyphenated/UUID-style run-ids would need the regex relaxed
//     to `[A-Za-z0-9_-]+` — a v0.9.x patch when that change lands.
package swarm

import (
	"regexp"
	"strconv"
	"strings"
)

// SyncPRAction is one parsed accept/dismiss line from a reply
// comment. v0.9 sync-pr emits these to the cli layer which then
// writes via findings.SetAction.
type SyncPRAction struct {
	Verb     string // "accepted" or "dismissed"
	RunID    string // <run_id> portion of the id
	Position int    // 0-indexed within the run
	Raw      string // original line (for stderr logging)
}

// replyKeywordRegex implements the spec grammar. Capture group 1 is
// the verb; group 2 is the id (run_id:position). Multi-line + case-
// insensitive flags applied.
var replyKeywordRegex = regexp.MustCompile(`(?im)^[\s>]*(accept|dismiss)\s*:\s*([A-Za-z0-9]+):([0-9]+)\s*$`)

// ParseReplyKeywords scans a comment body and returns every
// matching action line. Empty body or no matches → empty slice.
//
// Caller is responsible for run-id scoping (the priority-3 rule
// from the spec): replies that DON'T anchor at a kaijutsu marker
// AND don't contain an explicit run-id reference must be ignored.
// This function parses ALL action lines; the caller filters by
// scoping context.
func ParseReplyKeywords(body string) []SyncPRAction {
	if body == "" {
		return nil
	}
	matches := replyKeywordRegex.FindAllStringSubmatch(body, -1)
	if len(matches) == 0 {
		return nil
	}
	out := make([]SyncPRAction, 0, len(matches))
	for _, m := range matches {
		// m[1] = verb (mixed case), m[2] = run_id, m[3] = position.
		// Regex guarantees [0-9]+ for the position group, so Atoi
		// can't fail in practice; we still propagate-skip rather
		// than panic on the impossible case.
		pos, err := strconv.Atoi(m[3])
		if err != nil {
			continue
		}
		verb := strings.ToLower(m[1])
		var action string
		switch verb {
		case "accept":
			action = "accepted"
		case "dismiss":
			action = "dismissed"
		default:
			continue
		}
		out = append(out, SyncPRAction{
			Verb:     action,
			RunID:    m[2],
			Position: pos,
			Raw:      strings.TrimSpace(m[0]),
		})
	}
	return out
}

// IsScopedReply reports whether a reply comment qualifies for
// keyword parsing per spec §2 priority rules:
//
//  1. Threaded reply (in_reply_to_id chain anchored at a kaijutsu
//     marker comment) — caller passes anchorRunID = the ancestor's
//     run-id.
//  2. Plain comment containing an explicit run-id reference
//     ("run-id=<id>" text OR a quoted kaijutsu marker line).
//  3. Otherwise: ignore.
//
// Returns the run-id the reply attaches to, or "" if priority-3
// applies (drop the reply).
func IsScopedReply(body, anchorRunID string) string {
	if anchorRunID != "" {
		return anchorRunID
	}
	// Priority 2: "run-id=<id>" or quoted "<!-- kaijutsu-pr-review:run-id=<id>"
	for _, marker := range []string{
		"<!-- kaijutsu-pr-review:run-id=",
		"run-id=",
	} {
		idx := strings.Index(body, marker)
		if idx < 0 {
			continue
		}
		rest := body[idx+len(marker):]
		// Delimiters are whitespace ONLY — hyphens are valid run-id
		// chars (UUID-style ids may use them; current timestamp ids
		// don't). Stop at first space/tab/newline.
		end := strings.IndexAny(rest, " \t\n")
		if end < 0 {
			end = len(rest)
		}
		runID := strings.TrimSpace(rest[:end])
		if runID != "" {
			return runID
		}
	}
	return ""
}
