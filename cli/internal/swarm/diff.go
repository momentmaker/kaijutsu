package swarm

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// PRContext captures the inputs Stage 1 needs to feed agents: the PR
// number (when running against a real PR), the head commit SHA, and
// the actual diff text.
type PRContext struct {
	PR   int    // 0 when running on uncommitted/branch diff
	SHA  string // resolved head SHA
	Diff string
}

// FetchPRContext resolves the PR diff via gh. If pr == 0, it
// auto-detects the current branch's PR. If no PR exists, callers
// should fall back to FetchBranchDiff.
func FetchPRContext(ctx context.Context, pr int) (*PRContext, error) {
	if _, err := exec.LookPath("gh"); err != nil {
		return nil, fmt.Errorf("gh CLI not on PATH; install from https://cli.github.com/ or run with --diff-from-branch")
	}
	if pr == 0 {
		// Try to find a PR for the current branch.
		out, err := exec.CommandContext(ctx, "gh", "pr", "view", "--json", "number,headRefOid").Output()
		if err != nil {
			return nil, fmt.Errorf("no PR detected for current branch and --pr not provided: %w", err)
		}
		n, sha := parsePRView(string(out))
		if n == 0 {
			return nil, fmt.Errorf("could not parse gh pr view output")
		}
		pr = n
		diff, err := exec.CommandContext(ctx, "gh", "pr", "diff", strconv.Itoa(pr)).Output()
		if err != nil {
			return nil, fmt.Errorf("gh pr diff %d: %w", pr, err)
		}
		return &PRContext{PR: pr, SHA: sha, Diff: string(diff)}, nil
	}
	diff, err := exec.CommandContext(ctx, "gh", "pr", "diff", strconv.Itoa(pr)).Output()
	if err != nil {
		return nil, fmt.Errorf("gh pr diff %d: %w", pr, err)
	}
	shaOut, _ := exec.CommandContext(ctx, "gh", "pr", "view", strconv.Itoa(pr), "--json", "headRefOid", "--jq", ".headRefOid").Output()
	return &PRContext{PR: pr, SHA: strings.TrimSpace(string(shaOut)), Diff: string(diff)}, nil
}

// FetchBranchDiff returns the diff between the current branch and a
// base ref (default: origin/main, falling back to main). Used when
// no PR exists yet.
func FetchBranchDiff(ctx context.Context, base string) (*PRContext, error) {
	if base == "" {
		base = "origin/main"
		if exec.CommandContext(ctx, "git", "rev-parse", "--verify", base).Run() != nil {
			base = "main"
		}
	}
	diffCmd := exec.CommandContext(ctx, "git", "diff", base+"...HEAD")
	diff, err := diffCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git diff %s...HEAD: %w", base, err)
	}
	shaOut, _ := exec.CommandContext(ctx, "git", "rev-parse", "HEAD").Output()
	return &PRContext{SHA: strings.TrimSpace(string(shaOut)), Diff: string(diff)}, nil
}

// parsePRView extracts (number, headRefOid) from `gh pr view --json`
// output. Stays string-parser-only to avoid pulling encoding/json
// across this seam — the format is deterministic.
func parsePRView(s string) (int, string) {
	var n int
	var sha string
	for _, key := range []string{"number"} {
		if v := jsonScalar(s, key); v != "" {
			fmt.Sscanf(v, "%d", &n)
		}
	}
	sha = jsonScalar(s, "headRefOid")
	return n, sha
}

// jsonScalar pulls a scalar value (string or number) for a top-level
// key out of compact JSON. It does not handle nested structures —
// adequate for `gh pr view --json number,headRefOid`.
func jsonScalar(s, key string) string {
	needle := "\"" + key + "\":"
	i := strings.Index(s, needle)
	if i < 0 {
		return ""
	}
	rest := strings.TrimSpace(s[i+len(needle):])
	if strings.HasPrefix(rest, "\"") {
		end := strings.Index(rest[1:], "\"")
		if end < 0 {
			return ""
		}
		return rest[1 : end+1]
	}
	end := strings.IndexAny(rest, ",}\n")
	if end < 0 {
		return strings.TrimSpace(rest)
	}
	return strings.TrimSpace(rest[:end])
}
