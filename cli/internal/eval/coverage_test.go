package eval

import (
	"os"
	"path/filepath"
	"testing"
)

// TestEvalCoverage_CoreSkillsHaveEvalsJson is the v0.10 Stage 3
// gate per the plan: every skill in the allowlist below MUST ship
// an evals.json with non-empty evals[]. Adding a skill to the
// allowlist without authoring evals → CI fails.
//
// v0.10.x candidate: move the allowlist into
// `skills/core/.eval-coverage.yaml` so the source-of-truth lives
// next to the skills, not in this test file.
func TestEvalCoverage_CoreSkillsHaveEvalsJson(t *testing.T) {
	allowlist := []string{
		"dream",
		"pr-review",
		"scope-check",
	}
	repoRoot := repoRootForCoverageTest(t)
	for _, name := range allowlist {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(repoRoot, "skills", "core", name, "evals", "evals.json")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("eval-covered skill %q missing evals.json at %s: %v", name, path, err)
			}
			s, err := Parse(data)
			if err != nil {
				t.Fatalf("evals.json for %q does not parse: %v", name, err)
			}
			if len(s.Evals) == 0 {
				t.Errorf("evals.json for %q has empty evals[] array", name)
			}
		})
	}
}

// repoRootForCoverageTest finds the kaijutsu repo root by walking
// up from the test's cwd until it finds the cli/go.mod marker.
// Tests run from inside cli/internal/eval; coverage check needs
// access to skills/ which lives at the repo root.
func repoRootForCoverageTest(t *testing.T) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	dir := cwd
	for i := 0; i < 10; i++ {
		if _, err := os.Stat(filepath.Join(dir, "cli", "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("could not find repo root from %s", cwd)
	return ""
}
