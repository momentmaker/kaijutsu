package swarm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// MarkerPrefix identifies kaijutsu-pr-review comments so re-runs can
// find prior comments and edit-in-place. Format embeds the SHA so a
// new commit produces a fresh marker even if the comment is edited.
const markerPrefix = "<!-- kaijutsu-pr-review"

// Marker builds the closing-line HTML comment marker for a run.
func Marker(runID, sha string) string {
	return fmt.Sprintf("%s:run-id=%s sha=%s -->", markerPrefix, runID, sha)
}

// PostOrUpdateComment posts the synthesis markdown as a PR comment,
// or edits an existing kaijutsu-pr-review comment if one is present.
// When a prior comment exists, its body is appended to a collapsible
// "Previous reviews" footer so the timeline isn't lost.
//
// The current SHA is expected to be embedded in body's marker line
// already (via swarm.Marker); this function does not re-marker.
func PostOrUpdateComment(ctx context.Context, pr int, body string) error {
	prior, priorBody, err := findPriorComment(ctx, pr)
	if err != nil {
		return err
	}
	finalBody := body
	if priorBody != "" {
		finalBody = body + "\n\n" + buildHistoryFooter(priorBody)
	}
	if prior == 0 {
		return createComment(ctx, pr, finalBody)
	}
	return updateComment(ctx, prior, finalBody)
}

// findPriorComment returns the comment ID of the latest kaijutsu-
// pr-review comment on the PR (0 if none) plus its raw body.
func findPriorComment(ctx context.Context, pr int) (id int64, body string, err error) {
	out, err := exec.CommandContext(ctx, "gh", "pr", "view", strconv.Itoa(pr), "--json", "comments").Output()
	if err != nil {
		return 0, "", fmt.Errorf("gh pr view %d: %w", pr, err)
	}
	var v struct {
		Comments []struct {
			ID     int64  `json:"id"`
			URL    string `json:"url"`
			Body   string `json:"body"`
			Author struct {
				Login string `json:"login"`
			} `json:"author"`
		} `json:"comments"`
	}
	if err := json.Unmarshal(out, &v); err != nil {
		return 0, "", fmt.Errorf("parse gh comments: %w", err)
	}
	for i := len(v.Comments) - 1; i >= 0; i-- {
		c := v.Comments[i]
		if strings.Contains(c.Body, markerPrefix) {
			id := c.ID
			if id == 0 {
				id = idFromCommentURL(c.URL)
			}
			return id, c.Body, nil
		}
	}
	return 0, "", nil
}

// commentURLRe parses /issues/comments/<id> from a GH comment URL.
var commentURLRe = regexp.MustCompile(`/comments/(\d+)`)

func idFromCommentURL(url string) int64 {
	m := commentURLRe.FindStringSubmatch(url)
	if len(m) < 2 {
		return 0
	}
	n, _ := strconv.ParseInt(m[1], 10, 64)
	return n
}

func createComment(ctx context.Context, pr int, body string) error {
	c := exec.CommandContext(ctx, "gh", "pr", "comment", strconv.Itoa(pr), "--body-file", "-")
	c.Stdin = bytes.NewBufferString(body)
	out, err := c.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gh pr comment: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func updateComment(ctx context.Context, commentID int64, body string) error {
	if commentID == 0 {
		return fmt.Errorf("cannot edit prior comment: id is zero (gh JSON did not include id field)")
	}
	// gh api PATCH on the issues comment endpoint. Repo is
	// auto-detected from the cwd.
	c := exec.CommandContext(ctx, "gh", "api",
		fmt.Sprintf("repos/{owner}/{repo}/issues/comments/%d", commentID),
		"-X", "PATCH",
		"-f", "body=@-",
	)
	c.Stdin = bytes.NewBufferString(body)
	out, err := c.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gh api PATCH comment %d: %w: %s", commentID, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// buildHistoryFooter wraps the previous body in a collapsible
// <details> block. Strips the prior history footer from the prior
// body so we don't nest details forever.
func buildHistoryFooter(priorBody string) string {
	stripped := stripHistoryFooter(priorBody)
	priorMarker := extractMarker(stripped)
	heading := fmt.Sprintf("Previous review (sha %s)", priorMarker)
	if priorMarker == "" {
		heading = "Previous review"
	}
	return fmt.Sprintf("<details><summary>%s</summary>\n\n%s\n\n</details>",
		heading, strings.TrimSpace(stripped))
}

var historyFooterRe = regexp.MustCompile(`(?s)\n*<details><summary>Previous review.*?</details>\s*$`)

func stripHistoryFooter(body string) string {
	return historyFooterRe.ReplaceAllString(body, "")
}

var markerRe = regexp.MustCompile(`kaijutsu-pr-review:run-id=([^ ]+) sha=([^ ]+)`)

// extractMarker returns the SHA from a previous comment's marker line
// (empty string if no marker found).
func extractMarker(body string) string {
	m := markerRe.FindStringSubmatch(body)
	if len(m) < 3 {
		return ""
	}
	return m[2]
}

// NewRunID returns a deterministic-ish run id based on the current
// time. Embeds enough info to disambiguate same-SHA re-runs.
func NewRunID() string {
	return time.Now().UTC().Format("20060102T150405Z")
}
