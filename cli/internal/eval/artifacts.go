package eval

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// IterationDirPrefix is the on-disk prefix for iteration-N
// subdirectories under the workspace. agent-skills-eval upstream
// uses the same name; preserving compat means tools targeting that
// ecosystem can read our artifacts unchanged.
const IterationDirPrefix = "iteration-"

// SideWithSkill / SideWithoutSkill are the canonical artifact-side
// directory names for the SKILL eval shape (Stage 1). Stage 2 adds
// per-shape side names: persona names verbatim, mode names verbatim,
// and `with_skill_in_swarm/` / `without_skill_in_swarm/` for the
// swarm-skill shape.
const (
	SideWithSkill    = "with_skill"
	SideWithoutSkill = "without_skill"
)

// ArtifactPaths bundles the on-disk locations a single eval-run
// writes. Computed once at the top of RunSkill and threaded down so
// each writer doesn't recompute paths.
type ArtifactPaths struct {
	Workspace      string // user-supplied workspace dir
	IterationDir   string // <workspace>/iteration-N
	MetaJSON       string // <iteration>/meta.json
	BenchmarkJSON  string // <iteration>/benchmark.json
	BaselineJSON   string // <iteration>/eval-baseline.json (Stage 3 reads via git show)
	ReportHTML     string // <iteration>/report/index.html
}

// NextIteration scans the workspace for existing iteration-N dirs
// and returns the next integer. Starts at 1 for empty workspaces.
// Concurrent-run collision is prevented by AcquireLock; this helper
// assumes the lock is held.
func NextIteration(workspace string) (int, error) {
	entries, err := os.ReadDir(workspace)
	if err != nil {
		if os.IsNotExist(err) {
			return 1, nil
		}
		return 0, fmt.Errorf("read workspace: %w", err)
	}
	max := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, IterationDirPrefix) {
			continue
		}
		n, err := strconv.Atoi(strings.TrimPrefix(name, IterationDirPrefix))
		if err != nil {
			continue
		}
		if n > max {
			max = n
		}
	}
	return max + 1, nil
}

// PreparePaths computes ArtifactPaths for the given workspace +
// iteration number AND creates the iteration directory + report
// subdir. Idempotent: repeated calls are safe (MkdirAll).
func PreparePaths(workspace string, iteration int) (ArtifactPaths, error) {
	iterDir := filepath.Join(workspace, fmt.Sprintf("%s%d", IterationDirPrefix, iteration))
	reportDir := filepath.Join(iterDir, "report")
	if err := os.MkdirAll(reportDir, 0o755); err != nil {
		return ArtifactPaths{}, fmt.Errorf("create iteration dir: %w", err)
	}
	return ArtifactPaths{
		Workspace:     workspace,
		IterationDir:  iterDir,
		MetaJSON:      filepath.Join(iterDir, "meta.json"),
		BenchmarkJSON: filepath.Join(iterDir, "benchmark.json"),
		BaselineJSON:  filepath.Join(iterDir, "eval-baseline.json"),
		ReportHTML:    filepath.Join(reportDir, "index.html"),
	}, nil
}

// Meta is the on-disk shape of meta.json. Captures run-level
// metadata for reproducibility + report rendering.
type Meta struct {
	StartedAt        time.Time `json:"started_at"`
	FinishedAt       time.Time `json:"finished_at"`
	Workspace        string    `json:"workspace"`
	Iteration        int       `json:"iteration"`
	SkillName        string    `json:"skill_name"`
	Target           string    `json:"target"`
	Judge            string    `json:"judge"`
	JudgeTemplateVer string    `json:"judge_template_version"`
	Sides            int       `json:"sides"`
	EstimatedCostUSD float64   `json:"estimated_cost_usd"`
	ActualCostUSD    float64   `json:"actual_cost_usd"`
	Strict           bool      `json:"strict"`
	Concurrency      int       `json:"concurrency"`
}

// Benchmark is the rolled-up pass/fail snapshot. v0.10.0 carries
// per-skill, per-eval-id, per-side counts; Stage 3 reads this from
// a prior tag's commit to compute regressions.
type Benchmark struct {
	SkillName     string                 `json:"skill_name"`
	Iteration     int                    `json:"iteration"`
	TotalEvals    int                    `json:"total_evals"`
	Results       map[string]EvalResult  `json:"results"` // keyed by eval ID
	Indeterminate int                    `json:"indeterminate_count"`
}

// EvalResult is the per-eval rolled-up status across sides +
// assertions.
type EvalResult struct {
	ID    string                  `json:"id"`
	Name  string                  `json:"name"`
	Sides map[string]SideResult   `json:"sides"` // e.g. with_skill / without_skill
}

// SideResult is the verdict for one (eval, side) pair across all
// assertions.
type SideResult struct {
	Pass          bool                  `json:"pass"`
	Indeterminate bool                  `json:"indeterminate"`
	Output        string                `json:"-"` // written to disk separately, not in benchmark.json
	DurationMS    int64                 `json:"duration_ms"`
	CostUSD       float64               `json:"cost_usd"`
	Assertions    []AssertionResult     `json:"assertions"`
}

// AssertionResult captures one judge call's verdict.
type AssertionResult struct {
	Assertion     string  `json:"assertion"`
	Pass          bool    `json:"pass"`
	Indeterminate bool    `json:"indeterminate"`
	Reason        string  `json:"reason"`
	JudgeStage    int     `json:"judge_stage"`
	CostUSD       float64 `json:"cost_usd"`
}

// EvalBaseline is the minimal pass/fail snapshot persisted to
// eval-baseline.json. Stage 3 reads this from a prior tag via
// `git show <ref>:.kaijutsu/eval-runs/iteration-N/eval-baseline.json`.
// Keeping the schema small makes the diff resilient to cosmetic
// changes (timing fluctuations, judge reasoning rewording).
type EvalBaseline struct {
	SkillName string                       `json:"skill_name"`
	Iteration int                          `json:"iteration"`
	// Per-(eval-id, side) pass/fail. Stable shape across the whole
	// life of the v0.10 schema.
	Sides map[string]map[string]bool `json:"sides"`
}

// WriteMeta writes meta.json. Caller fills the Meta struct.
func WriteMeta(paths ArtifactPaths, meta Meta) error {
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal meta: %w", err)
	}
	return os.WriteFile(paths.MetaJSON, data, 0o644)
}

// WriteBenchmark writes benchmark.json with the rolled-up results.
func WriteBenchmark(paths ArtifactPaths, b Benchmark) error {
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal benchmark: %w", err)
	}
	return os.WriteFile(paths.BenchmarkJSON, data, 0o644)
}

// WriteBaseline writes eval-baseline.json — the minimal shape Stage
// 3 reads via git show. Distilled from the Benchmark.
func WriteBaseline(paths ArtifactPaths, b Benchmark) error {
	bl := EvalBaseline{
		SkillName: b.SkillName,
		Iteration: b.Iteration,
		Sides:     make(map[string]map[string]bool, len(b.Results)),
	}
	for id, res := range b.Results {
		sideMap := make(map[string]bool, len(res.Sides))
		for side, sr := range res.Sides {
			sideMap[side] = sr.Pass
		}
		bl.Sides[id] = sideMap
	}
	data, err := json.MarshalIndent(bl, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal baseline: %w", err)
	}
	return os.WriteFile(paths.BaselineJSON, data, 0o644)
}

// WriteSideArtifacts writes per-side output + grading + timing for
// one (eval, side). Directory layout:
//
//	<iteration>/eval-<id>/<side>/output.txt
//	<iteration>/eval-<id>/<side>/grading.json
//	<iteration>/eval-<id>/<side>/timing.json
//
// Path-traversal guard: eval IDs and side names are sanitized to
// `[a-zA-Z0-9_-]` before joining. Malicious or malformed inputs
// can't escape the iteration directory. Parser-level validation is
// the first line of defense; this is the second.
func WriteSideArtifacts(paths ArtifactPaths, evalID, side string, sr SideResult) error {
	safeID := sanitizePathComponent(evalID)
	safeSide := sanitizePathComponent(side)
	if safeID == "" || safeSide == "" {
		return fmt.Errorf("invalid path component (id=%q, side=%q)", evalID, side)
	}
	dir := filepath.Join(paths.IterationDir, "eval-"+safeID, safeSide)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "output.txt"), []byte(sr.Output), 0o644); err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	gradingData, err := json.MarshalIndent(struct {
		Pass          bool              `json:"pass"`
		Indeterminate bool              `json:"indeterminate"`
		Assertions    []AssertionResult `json:"assertions"`
	}{
		Pass:          sr.Pass,
		Indeterminate: sr.Indeterminate,
		Assertions:    sr.Assertions,
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal grading: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "grading.json"), gradingData, 0o644); err != nil {
		return fmt.Errorf("write grading: %w", err)
	}
	timingData, err := json.MarshalIndent(struct {
		DurationMS int64   `json:"duration_ms"`
		CostUSD    float64 `json:"cost_usd"`
	}{sr.DurationMS, sr.CostUSD}, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal timing: %w", err)
	}
	return os.WriteFile(filepath.Join(dir, "timing.json"), timingData, 0o644)
}

// sanitizePathComponent strips a string to `[a-zA-Z0-9_-]` for safe
// use as a filesystem path component. Empty input or input with
// only sanitized-out chars returns empty — caller treats that as
// an invalid component.
func sanitizePathComponent(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') ||
			r == '_' || r == '-' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// SortedEvalIDs returns evals' IDs in stable order. Used by
// renderers (HTML report, CLI summary) so output is deterministic
// across runs given the same input.
func SortedEvalIDs(b Benchmark) []string {
	ids := make([]string, 0, len(b.Results))
	for id := range b.Results {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
