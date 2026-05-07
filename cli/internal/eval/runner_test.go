package eval

import (
	"context"
	"strings"
	"testing"

	"github.com/momentmaker/kaijutsu/cli/internal/swarm"
)

// fakeTarget is a deterministic target agent. Returns a pre-set
// output map keyed by exact prompt match (or a default).
type fakeTarget struct {
	outputs map[string]string
	def     string
	err     error
	name    swarm.AgentName
}

func (f *fakeTarget) Run(ctx context.Context, prompt string, budget float64) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	if v, ok := f.outputs[prompt]; ok {
		return v, nil
	}
	return f.def, nil
}

func (f *fakeTarget) Name() swarm.AgentName { return f.name }

// fakeJudgeAlways is a JudgeAgent that always returns the canned
// pass/fail JSON. Tests drive specific verdicts.
type fakeJudgeAlways struct {
	pass bool
}

func (f *fakeJudgeAlways) Run(ctx context.Context, prompt string, budget float64) (string, error) {
	if f.pass {
		return `{"pass": true, "reason": "fake pass"}`, nil
	}
	return `{"pass": false, "reason": "fake fail"}`, nil
}

// TestRunner_WithSkillVsWithoutSkillHappyPath covers the canonical
// shape: SKILL.md in context for with_skill side, NOT in context for
// without_skill side. Output differs; judge verdicts mirror the
// difference. Pinned via the fake's prompt-keyed outputs.
func TestRunner_WithSkillVsWithoutSkillHappyPath(t *testing.T) {
	suite := &Suite{
		SkillName: "demo",
		Evals: []Eval{
			{ID: "e1", Name: "E1", Prompt: "task X", Assertions: []string{"is correct"}},
		},
	}
	target := &fakeTarget{
		outputs: map[string]string{
			"task X":                     "without-skill output",
			"SKILL_BODY\n\n---\n\ntask X": "with-skill output",
		},
		name: swarm.AgentClaude,
	}
	judge := &fakeJudgeAlways{pass: true}
	res, err := RunSkill(context.Background(), suite, RunOpts{
		Target:       target,
		Judge:        judge,
		JudgeName:    swarm.AgentClaude,
		BaselineSide: true,
		SkillBody:    "SKILL_BODY",
	})
	if err != nil {
		t.Fatalf("RunSkill: %v", err)
	}
	er := res.Bench.Results["e1"]
	if len(er.Sides) != 2 {
		t.Fatalf("want 2 sides, got %d", len(er.Sides))
	}
	if er.Sides[SideWithSkill].Output != "with-skill output" {
		t.Errorf("with_skill output wrong: %q", er.Sides[SideWithSkill].Output)
	}
	if er.Sides[SideWithoutSkill].Output != "without-skill output" {
		t.Errorf("without_skill output wrong: %q", er.Sides[SideWithoutSkill].Output)
	}
}

// TestRunner_NoBaselineSide_SkipsWithoutSkill covers the
// `--no-baseline-side` path: only the with_skill side runs.
func TestRunner_NoBaselineSide_SkipsWithoutSkill(t *testing.T) {
	suite := &Suite{
		SkillName: "demo",
		Evals: []Eval{
			{ID: "e1", Name: "E1", Prompt: "p", Assertions: []string{"a"}},
		},
	}
	target := &fakeTarget{def: "out", name: swarm.AgentClaude}
	judge := &fakeJudgeAlways{pass: true}
	res, err := RunSkill(context.Background(), suite, RunOpts{
		Target:       target,
		Judge:        judge,
		JudgeName:    swarm.AgentClaude,
		BaselineSide: false,
	})
	if err != nil {
		t.Fatalf("RunSkill: %v", err)
	}
	if _, ok := res.Bench.Results["e1"].Sides[SideWithoutSkill]; ok {
		t.Error("without_skill side should not be present")
	}
	if _, ok := res.Bench.Results["e1"].Sides[SideWithSkill]; !ok {
		t.Error("with_skill side missing")
	}
}

// TestStrictStateless_RegressionExits1: with_skill fails what
// without_skill passes → HasRegression flag set. Pinned via judge
// driving different verdicts per side via prompt content.
func TestStrictStateless_RegressionExits1(t *testing.T) {
	suite := &Suite{
		SkillName: "demo",
		Evals: []Eval{
			{ID: "regress", Name: "R", Prompt: "p", Assertions: []string{"a"}},
		},
	}
	// Different outputs per side; judge uses output presence as
	// signal: "good" → pass, "bad" → fail.
	target := &fakeTarget{
		outputs: map[string]string{
			"p":             "good output",
			"SKILL\n\n---\n\np": "bad output",
		},
		name: swarm.AgentClaude,
	}
	judge := &smartJudge{}
	res, err := RunSkill(context.Background(), suite, RunOpts{
		Target:       target,
		Judge:        judge,
		JudgeName:    swarm.AgentClaude,
		BaselineSide: true,
		Strict:       true,
		SkillBody:    "SKILL",
	})
	if err != nil {
		t.Fatalf("RunSkill: %v", err)
	}
	if !res.HasRegression {
		t.Errorf("expected regression flag set; with_skill=%+v, without_skill=%+v",
			res.Bench.Results["regress"].Sides[SideWithSkill],
			res.Bench.Results["regress"].Sides[SideWithoutSkill])
	}
}

// smartJudge passes when "good" appears in output, fails when "bad"
// does. Used for stateless-strict regression tests.
type smartJudge struct{}

func (s *smartJudge) Run(ctx context.Context, prompt string, budget float64) (string, error) {
	// Prompt contains the OUTPUT placeholder substitution; look for
	// the keywords there.
	if strings.Contains(prompt, "good output") {
		return `{"pass": true, "reason": "good"}`, nil
	}
	return `{"pass": false, "reason": "bad"}`, nil
}

// TestStrictStateless_NoRegressionExits0 covers the no-regression
// path: both sides pass → HasRegression unset.
func TestStrictStateless_NoRegressionExits0(t *testing.T) {
	suite := &Suite{
		SkillName: "demo",
		Evals:     []Eval{{ID: "ok", Name: "OK", Prompt: "p", Assertions: []string{"a"}}},
	}
	target := &fakeTarget{def: "good output", name: swarm.AgentClaude}
	judge := &smartJudge{}
	res, err := RunSkill(context.Background(), suite, RunOpts{
		Target:       target,
		Judge:        judge,
		JudgeName:    swarm.AgentClaude,
		BaselineSide: true,
		Strict:       true,
		SkillBody:    "SKILL",
	})
	if err != nil {
		t.Fatalf("RunSkill: %v", err)
	}
	if res.HasRegression {
		t.Error("expected no regression; both sides pass")
	}
}

// TestRunner_MultiAssertionAllPass covers the AND-aggregate
// invariant: 3 assertions all pass → side passes.
func TestRunner_MultiAssertionAllPass(t *testing.T) {
	suite := &Suite{
		SkillName: "demo",
		Evals: []Eval{
			{ID: "multi", Name: "M", Prompt: "p", Assertions: []string{"a1", "a2", "a3"}},
		},
	}
	target := &fakeTarget{def: "out", name: swarm.AgentClaude}
	judge := &fakeJudgeAlways{pass: true}
	res, err := RunSkill(context.Background(), suite, RunOpts{
		Target: target, Judge: judge, JudgeName: swarm.AgentClaude, BaselineSide: false,
	})
	if err != nil {
		t.Fatalf("RunSkill: %v", err)
	}
	side := res.Bench.Results["multi"].Sides[SideWithSkill]
	if !side.Pass {
		t.Errorf("3-assertion all-pass should result in pass=true; got side=%+v", side)
	}
	if len(side.Assertions) != 3 {
		t.Errorf("got %d assertion results, want 3", len(side.Assertions))
	}
}

// TestRunner_MultiAssertionAnyFailResultsInFail covers the
// AND-aggregate fail case: 1-of-3 fails → side fails.
func TestRunner_MultiAssertionAnyFailResultsInFail(t *testing.T) {
	suite := &Suite{
		SkillName: "demo",
		Evals: []Eval{
			{ID: "multi", Name: "M", Prompt: "p", Assertions: []string{"a1", "a2", "a3"}},
		},
	}
	target := &fakeTarget{def: "out", name: swarm.AgentClaude}
	// Judge: pass first 2, fail 3rd.
	judge := &counterJudge{verdicts: []bool{true, true, false}}
	res, err := RunSkill(context.Background(), suite, RunOpts{
		Target: target, Judge: judge, JudgeName: swarm.AgentClaude, BaselineSide: false,
	})
	if err != nil {
		t.Fatalf("RunSkill: %v", err)
	}
	side := res.Bench.Results["multi"].Sides[SideWithSkill]
	if side.Pass {
		t.Error("any-fail should result in pass=false")
	}
}

type counterJudge struct {
	verdicts []bool
	idx      int
}

func (c *counterJudge) Run(ctx context.Context, prompt string, budget float64) (string, error) {
	if c.idx >= len(c.verdicts) {
		return `{"pass": true, "reason": "default"}`, nil
	}
	v := c.verdicts[c.idx]
	c.idx++
	if v {
		return `{"pass": true, "reason": "ok"}`, nil
	}
	return `{"pass": false, "reason": "no"}`, nil
}

// TestCostEstimate_AbortAboveCap covers the pre-flight guard. Big
// suite × tight cap → abort before dispatch. Verifies the error
// names the cap.
func TestCostEstimate_AbortAboveCap(t *testing.T) {
	suite := &Suite{
		SkillName: "demo",
		Evals:     make([]Eval, 100), // 100 evals — guarantees a healthy estimate
	}
	for i := range suite.Evals {
		suite.Evals[i] = Eval{
			ID:         "e" + itoa(i),
			Name:       "E",
			Prompt:     "p",
			Assertions: []string{"a"},
		}
	}
	target := &fakeTarget{def: "out", name: swarm.AgentClaude}
	judge := &fakeJudgeAlways{pass: true}
	_, err := RunSkill(context.Background(), suite, RunOpts{
		Target:       target,
		Judge:        judge,
		JudgeName:    swarm.AgentClaude,
		BaselineSide: true,
		MaxCostUSD:   0.01, // tighter than 100-eval estimate
	})
	if err == nil {
		t.Fatal("expected pre-flight cost abort")
	}
	if !strings.Contains(err.Error(), "exceeds --max-cost cap") {
		t.Errorf("error should mention cap; got: %v", err)
	}
}

// TestRunner_FilterIncludeExclude covers the eval-id filter path.
func TestRunner_FilterIncludeExclude(t *testing.T) {
	suite := &Suite{
		SkillName: "demo",
		Evals: []Eval{
			{ID: "auth-bug", Name: "A", Prompt: "p", Assertions: []string{"a"}},
			{ID: "race", Name: "R", Prompt: "p", Assertions: []string{"a"}},
			{ID: "auth-perm", Name: "P", Prompt: "p", Assertions: []string{"a"}},
		},
	}
	target := &fakeTarget{def: "out", name: swarm.AgentClaude}
	judge := &fakeJudgeAlways{pass: true}
	res, err := RunSkill(context.Background(), suite, RunOpts{
		Target: target, Judge: judge, JudgeName: swarm.AgentClaude, BaselineSide: false,
		IncludeIDs: []string{"auth-*"},
	})
	if err != nil {
		t.Fatalf("RunSkill: %v", err)
	}
	if _, ok := res.Bench.Results["race"]; ok {
		t.Error("race should be excluded by include filter")
	}
	if _, ok := res.Bench.Results["auth-bug"]; !ok {
		t.Error("auth-bug should match include glob")
	}
	if _, ok := res.Bench.Results["auth-perm"]; !ok {
		t.Error("auth-perm should match include glob")
	}
}
