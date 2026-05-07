// baseline_from.go — v0.10 Stage 3 stateful --strict comparison.
//
// `--baseline-from <git-ref>` reads a prior tag's
// `eval-baseline.json` artifact via `git show <ref>:<path>` and
// compares the current run's per-(eval-id, side) pass/fail map
// against it. Regression = any eval-id that was passing in baseline
// but is failing now. First-run-with-coverage policy: missing prior
// baseline → exit 0 with warning. --accept-baseline gates seeding a
// new baseline-of-record on a tag where any eval failed.
package eval

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// BaselineRegression represents one (eval-id, side) pair where the
// prior baseline passed but the current run failed.
type BaselineRegression struct {
	EvalID string
	Side   string
}

// LoadBaselineFromGitRef fetches the eval-baseline.json artifact at
// a prior git ref. Returns:
//   - (*EvalBaseline, true, nil) when the file exists at <ref>:<path>
//   - (nil, false, nil) when the file is absent (first-run case)
//   - (nil, false, err) when git itself errored or the JSON is bad
//
// `repoRoot` pins the working directory for every git invocation —
// callers must pass the project root (typically from `projectRoot()`)
// rather than relying on the process cwd, which may be anywhere
// when `jutsu eval` is invoked from a subdirectory.
//
// `path` is relative to `repoRoot` with forward slashes (e.g.
// `.kaijutsu/eval-runs/iteration-N/eval-baseline.json`); callers
// must normalize via `filepath.ToSlash` on Windows. Stage 3 CI
// passes the most-recent prior tag's path; ad-hoc users can supply
// any ref + path combo.
//
// Existence is probed via `git rev-parse --verify <ref>^{commit}`
// then `git cat-file -e <ref>:<path>` (exit codes only, locale-
// independent) rather than parsing `git show` stderr — the v0.10
// adversarial pr-review caught a substring match on English error
// text that broke under non-English git locales.
func LoadBaselineFromGitRef(ctx context.Context, repoRoot, ref, path string) (*EvalBaseline, bool, error) {
	if ref == "" || path == "" {
		return nil, false, nil
	}
	if repoRoot == "" {
		return nil, false, fmt.Errorf("repoRoot required (use projectRoot() at the call site)")
	}
	// Two-step probe so we can distinguish "ref doesn't exist"
	// (real error) from "ref exists but path is absent there"
	// (first-run case) using exit codes only — no parsing of
	// localized stderr text.
	//
	//   1. `git rev-parse --verify <ref>^{commit}` — does the ref
	//      resolve to a commit? Non-zero exit = real error.
	//   2. `git cat-file -e <ref>:<path>` — given a valid ref,
	//      does the path exist at it? Non-zero exit = missing
	//      path (first-run case).
	verify := exec.CommandContext(ctx, "git", "rev-parse", "--verify", ref+"^{commit}")
	verify.Dir = repoRoot
	var verifyErr bytes.Buffer
	verify.Stderr = &verifyErr
	if err := verify.Run(); err != nil {
		return nil, false, fmt.Errorf("git rev-parse %s: %w (%s)", ref, err, strings.TrimSpace(verifyErr.String()))
	}
	spec := ref + ":" + path
	probe := exec.CommandContext(ctx, "git", "cat-file", "-e", spec)
	probe.Dir = repoRoot
	probe.Stderr = io.Discard
	if err := probe.Run(); err != nil {
		// Ref is known valid; non-zero here means path missing
		// at that ref — first-run case.
		return nil, false, nil
	}
	// Object exists → fetch contents.
	cmd := exec.CommandContext(ctx, "git", "show", spec)
	cmd.Dir = repoRoot
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		return nil, false, fmt.Errorf("git show %s: %w (%s)", spec, err, strings.TrimSpace(errBuf.String()))
	}
	var bl EvalBaseline
	if err := json.Unmarshal(out.Bytes(), &bl); err != nil {
		return nil, false, fmt.Errorf("parse baseline at %s: %w", spec, err)
	}
	return &bl, true, nil
}

// CompareAgainstBaseline returns the regression list: every
// (eval-id, side) where prior.Sides[eval-id][side] == true (passed)
// but current.Sides[eval-id][side] != true (failed or absent).
//
// Side notes:
//   - New eval-ids in current that weren't in prior → not a
//     regression (no prior signal to compare against).
//   - Eval-ids dropped in current but present in prior → counted as
//     regression for visibility (the eval got removed, possibly
//     hiding a failure).
//   - Sides absent in current → counted as regression.
func CompareAgainstBaseline(prior, current EvalBaseline) []BaselineRegression {
	var regressions []BaselineRegression
	for evalID, priorSides := range prior.Sides {
		currentSides, ok := current.Sides[evalID]
		if !ok {
			// Whole eval missing in current run.
			for side, passed := range priorSides {
				if passed {
					regressions = append(regressions, BaselineRegression{EvalID: evalID, Side: side})
				}
			}
			continue
		}
		for side, passed := range priorSides {
			if !passed {
				continue
			}
			if !currentSides[side] {
				regressions = append(regressions, BaselineRegression{EvalID: evalID, Side: side})
			}
		}
	}
	return regressions
}

// HasFailingEvals reports whether any (eval-id, side) in the
// baseline is failing. Used by Stage 3's --accept-baseline gate:
// when --accept-baseline is omitted AND the current run has
// failures AND no prior baseline exists, refuse to seed a broken
// baseline.
func HasFailingEvals(bl EvalBaseline) bool {
	for _, sides := range bl.Sides {
		for _, passed := range sides {
			if !passed {
				return true
			}
		}
	}
	return false
}
