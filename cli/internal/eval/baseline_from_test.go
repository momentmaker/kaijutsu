package eval

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// TestCompareAgainstBaseline_NoRegressionWhenAllStillPass covers
// the no-regression case: prior + current have same passes →
// empty regression list.
func TestCompareAgainstBaseline_NoRegressionWhenAllStillPass(t *testing.T) {
	prior := EvalBaseline{Sides: map[string]map[string]bool{
		"e1": {"with_skill": true, "without_skill": true},
	}}
	current := EvalBaseline{Sides: map[string]map[string]bool{
		"e1": {"with_skill": true, "without_skill": true},
	}}
	got := CompareAgainstBaseline(prior, current)
	if len(got) != 0 {
		t.Errorf("expected no regressions; got %v", got)
	}
}

// TestCompareAgainstBaseline_FlagsRegression covers the load-bearing
// case: prior had a passing side that current now fails →
// regression entry per (eval-id, side).
func TestCompareAgainstBaseline_FlagsRegression(t *testing.T) {
	prior := EvalBaseline{Sides: map[string]map[string]bool{
		"e1": {"with_skill": true},
	}}
	current := EvalBaseline{Sides: map[string]map[string]bool{
		"e1": {"with_skill": false},
	}}
	got := CompareAgainstBaseline(prior, current)
	if len(got) != 1 || got[0].EvalID != "e1" || got[0].Side != "with_skill" {
		t.Errorf("expected regression for e1/with_skill; got %v", got)
	}
}

// TestCompareAgainstBaseline_NewEvalsNotRegression: an eval-id
// added in current but absent in prior is NOT counted as a
// regression (no prior signal to compare against).
func TestCompareAgainstBaseline_NewEvalsNotRegression(t *testing.T) {
	prior := EvalBaseline{Sides: map[string]map[string]bool{
		"e1": {"with_skill": true},
	}}
	current := EvalBaseline{Sides: map[string]map[string]bool{
		"e1":      {"with_skill": true},
		"e2-new":  {"with_skill": false}, // new eval, not in prior
	}}
	got := CompareAgainstBaseline(prior, current)
	if len(got) != 0 {
		t.Errorf("new eval should not count as regression; got %v", got)
	}
}

// TestCompareAgainstBaseline_DroppedEvalCountsAsRegression: an
// eval-id removed from current but passing in prior IS counted —
// otherwise authors could hide failures by deleting evals.
func TestCompareAgainstBaseline_DroppedEvalCountsAsRegression(t *testing.T) {
	prior := EvalBaseline{Sides: map[string]map[string]bool{
		"e1": {"with_skill": true},
		"e2": {"with_skill": true},
	}}
	current := EvalBaseline{Sides: map[string]map[string]bool{
		"e1": {"with_skill": true},
		// e2 removed
	}}
	got := CompareAgainstBaseline(prior, current)
	if len(got) != 1 || got[0].EvalID != "e2" {
		t.Errorf("expected regression for dropped e2; got %v", got)
	}
}

// TestCompareAgainstBaseline_MultipleSides covers the per-side
// granularity: regression on one side only registers one entry.
func TestCompareAgainstBaseline_MultipleSides(t *testing.T) {
	prior := EvalBaseline{Sides: map[string]map[string]bool{
		"e1": {"with_skill": true, "without_skill": true},
	}}
	current := EvalBaseline{Sides: map[string]map[string]bool{
		"e1": {"with_skill": false, "without_skill": true},
	}}
	got := CompareAgainstBaseline(prior, current)
	if len(got) != 1 || got[0].Side != "with_skill" {
		t.Errorf("expected only with_skill regression; got %v", got)
	}
}

// TestCompareAgainstBaseline_DeterministicOrder: regression list
// should be stable across runs given identical inputs. Sort the
// output for the assert; the test pins ordering invariance, not
// any specific natural order.
func TestCompareAgainstBaseline_DeterministicOrder(t *testing.T) {
	prior := EvalBaseline{Sides: map[string]map[string]bool{
		"a": {"x": true, "y": true},
		"b": {"x": true},
	}}
	current := EvalBaseline{Sides: map[string]map[string]bool{
		"a": {"x": false, "y": false},
		"b": {"x": false},
	}}
	a := CompareAgainstBaseline(prior, current)
	b := CompareAgainstBaseline(prior, current)
	sortRegs(a)
	sortRegs(b)
	if !reflect.DeepEqual(a, b) {
		t.Errorf("non-deterministic compare: %v vs %v", a, b)
	}
}

func sortRegs(rs []BaselineRegression) {
	sort.Slice(rs, func(i, j int) bool {
		if rs[i].EvalID != rs[j].EvalID {
			return rs[i].EvalID < rs[j].EvalID
		}
		return rs[i].Side < rs[j].Side
	})
}

// gitInit creates a temp git repo with one initial commit so we can
// commit + tag baseline.json fixtures and then resolve them via
// `git show <ref>:<path>`. Skips if `git` isn't on PATH.
func gitInit(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "--quiet"},
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "test"},
		{"config", "commit.gpgsign", "false"},
		{"config", "init.defaultBranch", "main"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, out)
		}
	}
	// Seed an initial commit so HEAD is valid.
	seed := filepath.Join(dir, ".gitkeep")
	if err := os.WriteFile(seed, []byte(""), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	for _, args := range [][]string{
		{"add", ".gitkeep"},
		{"commit", "-q", "-m", "init"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, out)
		}
	}
	return dir
}

func gitCommitFile(t *testing.T, dir, relPath, body, msg string) {
	t.Helper()
	abs := filepath.Join(dir, relPath)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(abs, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	for _, args := range [][]string{
		{"add", relPath},
		{"commit", "-q", "-m", msg},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, out)
		}
	}
}

func gitTag(t *testing.T, dir, name string) {
	t.Helper()
	// Use annotated tag with -m to play nice with hosts that have
	// `tag.forceSignAnnotated` or require tag messages via local
	// git config.
	cmd := exec.Command("git", "-c", "tag.gpgsign=false", "tag", "-a", name, "-m", name)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git tag %s: %v (%s)", name, err, out)
	}
}

// TestLoadBaselineFromGitRef_FoundAtTag covers the happy path: the
// file was committed at a prior tag, `git show <tag>:<path>`
// resolves, JSON parses → returns (*EvalBaseline, true, nil).
func TestLoadBaselineFromGitRef_FoundAtTag(t *testing.T) {
	dir := gitInit(t)
	body := `{"skill_name":"x","iteration":1,"sides":{"e1":{"with_skill":true}}}`
	relPath := ".kaijutsu/eval-runs/iteration-1/eval-baseline.json"
	gitCommitFile(t, dir, relPath, body, "seed baseline")
	gitTag(t, dir, "v0.9.0")

	bl, found, err := LoadBaselineFromGitRef(context.Background(), dir, "v0.9.0", relPath)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !found {
		t.Fatal("expected found=true at tag where file exists")
	}
	if bl == nil || bl.SkillName != "x" || !bl.Sides["e1"]["with_skill"] {
		t.Errorf("baseline content not parsed: %+v", bl)
	}
}

// TestLoadBaselineFromGitRef_AbsentAtTag covers first-run case: the
// tag exists but doesn't contain the path → returns (nil, false, nil)
// (NOT an error). This is the load-bearing locale-independence
// check: previously the implementation matched English git stderr
// substrings.
func TestLoadBaselineFromGitRef_AbsentAtTag(t *testing.T) {
	dir := gitInit(t)
	gitTag(t, dir, "v0.9.0") // tag with no baseline.json

	bl, found, err := LoadBaselineFromGitRef(context.Background(), dir, "v0.9.0", ".kaijutsu/eval-runs/iteration-1/eval-baseline.json")
	if err != nil {
		t.Fatalf("absent should not error; got: %v", err)
	}
	if found {
		t.Error("expected found=false for missing path at tag")
	}
	if bl != nil {
		t.Error("expected nil baseline on absent")
	}
}

// TestLoadBaselineFromGitRef_BadRef covers the real-error path: a
// ref that doesn't exist at all should surface as an error rather
// than be silently treated as first-run.
func TestLoadBaselineFromGitRef_BadRef(t *testing.T) {
	dir := gitInit(t)
	_, found, err := LoadBaselineFromGitRef(context.Background(), dir, "nonexistent-ref-zzz", "anything.json")
	if err == nil {
		t.Fatal("expected error for nonexistent ref")
	}
	if found {
		t.Error("expected found=false on bad ref")
	}
}

// TestLoadBaselineFromGitRef_BadJSON covers the parse-error path:
// file exists but is not valid JSON → returns parse error.
func TestLoadBaselineFromGitRef_BadJSON(t *testing.T) {
	dir := gitInit(t)
	relPath := "broken.json"
	gitCommitFile(t, dir, relPath, "{ this is not json", "broken")
	gitTag(t, dir, "v0.9.0")

	_, _, err := LoadBaselineFromGitRef(context.Background(), dir, "v0.9.0", relPath)
	if err == nil {
		t.Fatal("expected parse error for malformed JSON")
	}
	if !strings.Contains(err.Error(), "parse baseline") {
		t.Errorf("error should mention parsing; got: %v", err)
	}
}

// TestLoadBaselineFromGitRef_EmptyArgs covers defensive guard:
// empty ref or empty path → (nil, false, nil), no git call.
func TestLoadBaselineFromGitRef_EmptyArgs(t *testing.T) {
	for _, tc := range []struct {
		ref, path string
	}{
		{"", "x.json"},
		{"v1.0", ""},
		{"", ""},
	} {
		bl, found, err := LoadBaselineFromGitRef(context.Background(), "/tmp", tc.ref, tc.path)
		if err != nil || found || bl != nil {
			t.Errorf("empty(%q,%q): got (%v,%v,%v); want (nil,false,nil)", tc.ref, tc.path, bl, found, err)
		}
	}
}

// TestLoadBaselineFromGitRef_RequiresRepoRoot covers the cmd.Dir
// portability fix: empty repoRoot is rejected up front rather than
// relying on the process cwd (which is unsafe when the user runs
// `jutsu eval` from a subdirectory).
func TestLoadBaselineFromGitRef_RequiresRepoRoot(t *testing.T) {
	_, _, err := LoadBaselineFromGitRef(context.Background(), "", "v0.9.0", "x.json")
	if err == nil {
		t.Fatal("expected error when repoRoot is empty")
	}
	if !strings.Contains(err.Error(), "repoRoot") {
		t.Errorf("error should mention repoRoot; got: %v", err)
	}
}

// TestLoadBaselineFromGitRef_WorksFromSubdir covers the cmd.Dir
// portability fix: caller's process cwd is irrelevant when
// repoRoot is set explicitly.
func TestLoadBaselineFromGitRef_WorksFromSubdir(t *testing.T) {
	dir := gitInit(t)
	body := `{"skill_name":"x","iteration":1,"sides":{}}`
	relPath := "baseline.json"
	gitCommitFile(t, dir, relPath, body, "seed")
	gitTag(t, dir, "v0.9.0")

	// Chdir to a totally unrelated dir; repoRoot arg should still
	// pin git to `dir`.
	other := t.TempDir()
	prev, _ := os.Getwd()
	if err := os.Chdir(other); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(prev)

	bl, found, err := LoadBaselineFromGitRef(context.Background(), dir, "v0.9.0", relPath)
	if err != nil || !found || bl == nil {
		t.Errorf("repoRoot pinning broken from subdir: err=%v found=%v bl=%v", err, found, bl)
	}
}

// TestHasFailingEvals covers the --accept-baseline gate: any
// failure in the baseline → true; all-pass → false; empty → false.
func TestHasFailingEvals(t *testing.T) {
	cases := []struct {
		name string
		bl   EvalBaseline
		want bool
	}{
		{"all pass", EvalBaseline{Sides: map[string]map[string]bool{"e": {"s": true}}}, false},
		{"any fail", EvalBaseline{Sides: map[string]map[string]bool{"e": {"s": false}}}, true},
		{"mix", EvalBaseline{Sides: map[string]map[string]bool{"e": {"a": true, "b": false}}}, true},
		{"empty", EvalBaseline{Sides: map[string]map[string]bool{}}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := HasFailingEvals(tc.bl); got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}
