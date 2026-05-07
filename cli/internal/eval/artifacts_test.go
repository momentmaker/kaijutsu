package eval

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestNextIteration_EmptyWorkspaceStartsAt1 covers the cold-start
// path: brand-new workspace → iteration-1.
func TestNextIteration_EmptyWorkspaceStartsAt1(t *testing.T) {
	tmp := t.TempDir()
	n, err := NextIteration(tmp)
	if err != nil {
		t.Fatalf("NextIteration: %v", err)
	}
	if n != 1 {
		t.Errorf("got %d, want 1", n)
	}
}

// TestNextIteration_MissingWorkspaceReturns1 covers the case where
// the workspace dir doesn't exist yet (first run after init). No
// error; returns 1.
func TestNextIteration_MissingWorkspaceReturns1(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "does-not-exist")
	n, err := NextIteration(tmp)
	if err != nil {
		t.Fatalf("expected nil err for missing dir; got: %v", err)
	}
	if n != 1 {
		t.Errorf("got %d, want 1", n)
	}
}

// TestNextIteration_Monotonic covers the increment path: existing
// iteration-1, iteration-2, iteration-5 → next is 6.
func TestNextIteration_Monotonic(t *testing.T) {
	tmp := t.TempDir()
	for _, n := range []int{1, 2, 5} {
		if err := os.MkdirAll(filepath.Join(tmp, "iteration-"+itoa(n)), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}
	got, err := NextIteration(tmp)
	if err != nil {
		t.Fatalf("NextIteration: %v", err)
	}
	if got != 6 {
		t.Errorf("got %d, want 6 (max=5 → 6)", got)
	}
}

// itoa avoids strconv import for the single test helper.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(buf[pos:])
}

// TestPreparePaths_CreatesIterationAndReportDirs covers the
// directory-creation invariant: PreparePaths is idempotent and
// produces the iteration + report subdirs.
func TestPreparePaths_CreatesIterationAndReportDirs(t *testing.T) {
	tmp := t.TempDir()
	paths, err := PreparePaths(tmp, 1)
	if err != nil {
		t.Fatalf("PreparePaths: %v", err)
	}
	if _, err := os.Stat(paths.IterationDir); err != nil {
		t.Errorf("iteration dir missing: %v", err)
	}
	if _, err := os.Stat(filepath.Dir(paths.ReportHTML)); err != nil {
		t.Errorf("report dir missing: %v", err)
	}
	// Repeat call is idempotent.
	if _, err := PreparePaths(tmp, 1); err != nil {
		t.Errorf("repeat call should be idempotent: %v", err)
	}
}

// TestArtifacts_IterationLayoutMatches covers the full write path:
// meta + benchmark + side artifacts → expected file tree.
func TestArtifacts_IterationLayoutMatches(t *testing.T) {
	tmp := t.TempDir()
	paths, err := PreparePaths(tmp, 1)
	if err != nil {
		t.Fatalf("PreparePaths: %v", err)
	}
	meta := Meta{
		StartedAt:        time.Now(),
		FinishedAt:       time.Now().Add(time.Second),
		Workspace:        tmp,
		Iteration:        1,
		SkillName:        "demo",
		Target:           "claude",
		Judge:            "claude",
		JudgeTemplateVer: JudgeTemplateVersion,
		Sides:            2,
	}
	if err := WriteMeta(paths, meta); err != nil {
		t.Fatalf("WriteMeta: %v", err)
	}
	bench := Benchmark{
		SkillName:  "demo",
		Iteration:  1,
		TotalEvals: 1,
		Results: map[string]EvalResult{
			"smoke": {
				ID:   "smoke",
				Name: "Smoke",
				Sides: map[string]SideResult{
					"with_skill": {
						Pass:       true,
						Output:     "with-skill output",
						DurationMS: 100,
						CostUSD:    0.01,
						Assertions: []AssertionResult{{Assertion: "a", Pass: true, Reason: "r", JudgeStage: 1}},
					},
					"without_skill": {
						Pass:       false,
						Output:     "without-skill output",
						DurationMS: 100,
						CostUSD:    0.01,
						Assertions: []AssertionResult{{Assertion: "a", Pass: false, Reason: "r", JudgeStage: 1}},
					},
				},
			},
		},
	}
	if err := WriteBenchmark(paths, bench); err != nil {
		t.Fatalf("WriteBenchmark: %v", err)
	}
	if err := WriteBaseline(paths, bench); err != nil {
		t.Fatalf("WriteBaseline: %v", err)
	}
	for side, sr := range bench.Results["smoke"].Sides {
		if err := WriteSideArtifacts(paths, "smoke", side, sr); err != nil {
			t.Fatalf("WriteSideArtifacts %s: %v", side, err)
		}
	}

	// Verify file tree.
	for _, want := range []string{
		paths.MetaJSON,
		paths.BenchmarkJSON,
		paths.BaselineJSON,
		filepath.Join(paths.IterationDir, "eval-smoke", "with_skill", "output.txt"),
		filepath.Join(paths.IterationDir, "eval-smoke", "with_skill", "grading.json"),
		filepath.Join(paths.IterationDir, "eval-smoke", "with_skill", "timing.json"),
		filepath.Join(paths.IterationDir, "eval-smoke", "without_skill", "output.txt"),
	} {
		if _, err := os.Stat(want); err != nil {
			t.Errorf("missing expected artifact: %s (%v)", want, err)
		}
	}
}

// TestWriteSideArtifacts_RejectsPathTraversal covers the
// sanitization invariant. Malicious eval ID containing path-
// traversal sequences must not escape the iteration dir.
func TestWriteSideArtifacts_RejectsPathTraversal(t *testing.T) {
	tmp := t.TempDir()
	paths, err := PreparePaths(tmp, 1)
	if err != nil {
		t.Fatalf("PreparePaths: %v", err)
	}
	cases := []struct {
		evalID string
		side   string
	}{
		{"../escape", "with_skill"},
		{"good", "../escape"},
		{"/abs/path", "with_skill"},
		{"foo/bar", "with_skill"},
	}
	for _, tc := range cases {
		// IDs containing only sanitization-rejected chars (e.g. "/")
		// produce empty safeID and trigger the explicit error.
		err := WriteSideArtifacts(paths, tc.evalID, tc.side, SideResult{Pass: true})
		// Some traversal attempts produce sanitized but non-empty
		// strings (e.g. "../escape" → "escape"); those write
		// successfully to a sanitized location. Verify the file
		// landed inside the iteration directory regardless.
		if err == nil {
			// Walk filesystem to confirm no file landed outside iter.
			_ = filepath.Walk(tmp, func(p string, info os.FileInfo, e error) error {
				if e != nil || info.IsDir() {
					return nil
				}
				rel, _ := filepath.Rel(tmp, p)
				if !filepath.IsLocal(rel) {
					t.Errorf("file landed outside workspace: %s (eval=%q side=%q)", p, tc.evalID, tc.side)
				}
				return nil
			})
		}
	}
}

// TestSanitizePathComponent covers the sanitizer directly.
func TestSanitizePathComponent(t *testing.T) {
	cases := map[string]string{
		"normal-id":    "normal-id",
		"snake_case":   "snake_case",
		"alphaNum123":  "alphaNum123",
		"../escape":    "escape",
		"/abs":         "abs",
		"foo/bar":      "foobar",
		"":             "",
		"!!!":          "",
		"name with sp": "namewithsp",
	}
	for in, want := range cases {
		if got := sanitizePathComponent(in); got != want {
			t.Errorf("sanitize(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestWriteBaseline_DistillsToPassFailOnly covers the
// eval-baseline.json shape: drop reasoning + cost + timing; keep
// per-(eval-id, side) pass/fail. Stable across cosmetic changes.
func TestWriteBaseline_DistillsToPassFailOnly(t *testing.T) {
	tmp := t.TempDir()
	paths, err := PreparePaths(tmp, 1)
	if err != nil {
		t.Fatalf("PreparePaths: %v", err)
	}
	bench := Benchmark{
		SkillName: "x",
		Iteration: 1,
		Results: map[string]EvalResult{
			"e1": {ID: "e1", Sides: map[string]SideResult{
				"with_skill":    {Pass: true},
				"without_skill": {Pass: false},
			}},
		},
	}
	if err := WriteBaseline(paths, bench); err != nil {
		t.Fatalf("WriteBaseline: %v", err)
	}
	data, err := os.ReadFile(paths.BaselineJSON)
	if err != nil {
		t.Fatalf("read baseline: %v", err)
	}
	var bl EvalBaseline
	if err := json.Unmarshal(data, &bl); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if bl.Sides["e1"]["with_skill"] != true || bl.Sides["e1"]["without_skill"] != false {
		t.Errorf("baseline sides wrong: %+v", bl.Sides)
	}
}
