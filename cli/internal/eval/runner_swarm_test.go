package eval

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/momentmaker/kaijutsu/cli/internal/swarm"
)

// fakeResolver maps a name → preset agent map. Used by the swarm-
// shape runner tests to control which agent each persona/mode
// resolves to.
type fakeResolver struct {
	agents map[string]TargetAgent
	err    error
}

func (f *fakeResolver) Resolve(ctx context.Context, name string) (TargetAgent, error) {
	if f.err != nil {
		return nil, f.err
	}
	a, ok := f.agents[name]
	if !ok {
		return nil, errors.New("no agent for name " + name)
	}
	return a, nil
}

// TestRunPersonaEval_TwoPersonasHeadToHead covers Stage 2 plan
// success criterion: two personas dispatched against the same
// prompt; per-persona artifact dirs + grading.
func TestRunPersonaEval_TwoPersonasHeadToHead(t *testing.T) {
	suite := &Suite{
		SkillName: "demo",
		Evals: []Eval{
			{ID: "shim", Name: "shim", Prompt: "p"}, // upstream evals[] non-empty
		},
		Kaijutsu: &KaijutsuExtension{
			Personas: []PersonaCase{
				{
					ID:         "honest-vs-default",
					Baseline:   "default-claude",
					Challenger: "honest-deepseek",
					Prompt:     "pre-implementation review",
					Assertions: []string{"surfaces real concerns"},
				},
			},
		},
	}
	resolver := &fakeResolver{
		agents: map[string]TargetAgent{
			"default-claude":   &fakeTarget{def: "baseline output", name: swarm.AgentClaude},
			"honest-deepseek":  &fakeTarget{def: "challenger output", name: swarm.AgentName("deepseek")},
		},
	}
	res, err := RunPersonaSuite(context.Background(), suite, SwarmShapeOpts{
		Resolver:      resolver,
		Judge:         &fakeJudgeAlways{pass: true},
		JudgeName:     swarm.AgentClaude,
		JudgeTemplate: DefaultJudgeTemplate,
		Concurrency:   2,
	})
	if err != nil {
		t.Fatalf("RunPersonaSuite: %v", err)
	}
	er := res.Bench.Results["honest-vs-default"]
	if len(er.Sides) != 2 {
		t.Fatalf("want 2 sides; got %d", len(er.Sides))
	}
	if _, ok := er.Sides["default-claude"]; !ok {
		t.Error("baseline persona side missing")
	}
	if _, ok := er.Sides["honest-deepseek"]; !ok {
		t.Error("challenger persona side missing")
	}
}

// TestRunPresetEval_QuickVsFull covers the Stage 2 plan path: same
// prompt against two preset modes (e.g. dream quick vs full).
func TestRunPresetEval_QuickVsFull(t *testing.T) {
	suite := &Suite{
		SkillName: "demo",
		Evals:     []Eval{{ID: "shim", Name: "shim", Prompt: "p"}},
		Kaijutsu: &KaijutsuExtension{
			Presets: []PresetCase{
				{
					ID:         "dream-quick-vs-full",
					Preset:     "dream",
					Baseline:   "quick",
					Challenger: "full",
					Prompt:     "should we ship X?",
					Assertions: []string{"surfaces concern"},
				},
			},
		},
	}
	resolver := &fakeResolver{
		agents: map[string]TargetAgent{
			"dream:quick": &fakeTarget{def: "quick output", name: swarm.AgentClaude},
			"dream:full":  &fakeTarget{def: "full output", name: swarm.AgentClaude},
		},
	}
	res, err := RunPresetSuite(context.Background(), suite, SwarmShapeOpts{
		Resolver:      resolver,
		Judge:         &fakeJudgeAlways{pass: true},
		JudgeName:     swarm.AgentClaude,
		JudgeTemplate: DefaultJudgeTemplate,
	})
	if err != nil {
		t.Fatalf("RunPresetSuite: %v", err)
	}
	er := res.Bench.Results["dream-quick-vs-full"]
	if _, ok := er.Sides["quick"]; !ok {
		t.Error("baseline mode side missing")
	}
	if _, ok := er.Sides["full"]; !ok {
		t.Error("challenger mode side missing")
	}
}

// TestRunSwarmSkillEval_WithSkillInSwarm covers the Stage 2 plan
// path: skill loaded into a swarm preset's pipeline vs the same
// preset without the skill.
func TestRunSwarmSkillEval_WithSkillInSwarm(t *testing.T) {
	suite := &Suite{
		SkillName: "demo",
		Evals:     []Eval{{ID: "shim", Name: "shim", Prompt: "p"}},
		Kaijutsu: &KaijutsuExtension{
			Swarm: []SwarmCase{
				{
					ID:         "pr-review-with-dream",
					Preset:     "pr-review",
					Diff:       "fixtures/sample.diff",
					Baseline:   "without_skill_in_swarm",
					Challenger: "with_skill_in_swarm",
					Assertions: []string{"catches auth bug"},
				},
			},
		},
	}
	resolver := &fakeResolver{
		agents: map[string]TargetAgent{
			"pr-review:without_skill_in_swarm": &fakeTarget{def: "no auth bug", name: swarm.AgentClaude},
			"pr-review:with_skill_in_swarm":    &fakeTarget{def: "found auth bug at L42", name: swarm.AgentClaude},
		},
	}
	res, err := RunSwarmSkillSuite(context.Background(), suite, SwarmShapeOpts{
		Resolver:      resolver,
		Judge:         &fakeJudgeAlways{pass: true},
		JudgeName:     swarm.AgentClaude,
		JudgeTemplate: DefaultJudgeTemplate,
	})
	if err != nil {
		t.Fatalf("RunSwarmSkillSuite: %v", err)
	}
	er := res.Bench.Results["pr-review-with-dream"]
	if _, ok := er.Sides["with_skill_in_swarm"]; !ok {
		t.Error("with_skill_in_swarm side missing")
	}
	if _, ok := er.Sides["without_skill_in_swarm"]; !ok {
		t.Error("without_skill_in_swarm side missing")
	}
}

// TestRunPersonaEval_RegressionDetected covers the Stage 2
// baseline-vs-challenger regression check: baseline passes,
// challenger fails → HasRegression=true.
func TestRunPersonaEval_RegressionDetected(t *testing.T) {
	suite := &Suite{
		SkillName: "demo",
		Evals:     []Eval{{ID: "shim", Name: "shim", Prompt: "p"}},
		Kaijutsu: &KaijutsuExtension{
			Personas: []PersonaCase{{
				ID: "regress", Baseline: "good", Challenger: "bad",
				Prompt: "p", Assertions: []string{"matches"},
			}},
		},
	}
	resolver := &fakeResolver{
		agents: map[string]TargetAgent{
			"good": &fakeTarget{def: "PASS", name: swarm.AgentClaude},
			"bad":  &fakeTarget{def: "FAIL", name: swarm.AgentClaude},
		},
	}
	// substringJudge passes only when the OUTPUT contains "PASS"
	// (the baseline output). Challenger output "FAIL" doesn't match
	// → fail. Result: baseline passes, challenger fails →
	// HasRegression=true.
	res, err := RunPersonaSuite(context.Background(), suite, SwarmShapeOpts{
		Resolver:      resolver,
		Judge:         &substringJudge{passSubstr: "PASS"},
		JudgeName:     swarm.AgentClaude,
		JudgeTemplate: DefaultJudgeTemplate,
	})
	if err != nil {
		t.Fatalf("RunPersonaSuite: %v", err)
	}
	if !res.HasRegression {
		t.Errorf("expected regression: baseline=PASS, challenger=FAIL; got benchmark=%+v", res.Bench.Results["regress"])
	}
}

// substringJudge: passes if the OUTPUT (interpolated into the
// prompt) contains a specific substring. Tests use it to drive
// per-side verdicts deterministically.
type substringJudge struct {
	passSubstr string
}

func (s *substringJudge) Run(ctx context.Context, prompt string, budget float64) (string, error) {
	if strings.Contains(prompt, "OUTPUT:\n"+s.passSubstr) {
		return `{"pass": true, "reason": "ok"}`, nil
	}
	return `{"pass": false, "reason": "no match"}`, nil
}

// TestKaijutsuExtensionParse_RoundTripsAllShapes verifies the v0.10
// schema accepts the full kaijutsu.* extension in one fixture
// without losing any case.
func TestKaijutsuExtensionParse_RoundTripsAllShapes(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata/agent-skills-eval-fixtures/v1", "with-kaijutsu-extension.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	s, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if s.Kaijutsu == nil {
		t.Fatal("kaijutsu block should be parsed")
	}
	if len(s.Kaijutsu.Swarm) != 1 {
		t.Errorf("swarm count=%d, want 1", len(s.Kaijutsu.Swarm))
	}
	if len(s.Kaijutsu.Personas) != 1 {
		t.Errorf("personas count=%d, want 1", len(s.Kaijutsu.Personas))
	}
	if len(s.Kaijutsu.Presets) != 1 {
		t.Errorf("presets count=%d, want 1", len(s.Kaijutsu.Presets))
	}
	// Spot-check a few field carries.
	if s.Kaijutsu.Personas[0].Baseline != "default-claude" {
		t.Errorf("personas[0].baseline=%q", s.Kaijutsu.Personas[0].Baseline)
	}
	if s.Kaijutsu.Presets[0].Challenger != "full" {
		t.Errorf("presets[0].challenger=%q", s.Kaijutsu.Presets[0].Challenger)
	}
	if s.Kaijutsu.Swarm[0].Preset != "pr-review" {
		t.Errorf("swarm[0].preset=%q", s.Kaijutsu.Swarm[0].Preset)
	}
}

// TestJudgeOverride_PlaceholderValidationRequired covers the
// Stage 2 contract: an evals/judge.md missing `{assertion}` or
// `{output}` hard-fails at LoadJudgeTemplate time.
func TestJudgeOverride_PlaceholderValidationRequired(t *testing.T) {
	tmp := t.TempDir()
	skillDir := filepath.Join(tmp, "skill")
	evalsDir := filepath.Join(skillDir, "evals")
	if err := os.MkdirAll(evalsDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Override missing {output}.
	if err := os.WriteFile(filepath.Join(evalsDir, "judge.md"), []byte("Grade {assertion}"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := LoadJudgeTemplate(skillDir)
	if err == nil {
		t.Fatal("expected validation error for missing {output}")
	}
	if !strings.Contains(err.Error(), "{output}") {
		t.Errorf("error should name the missing placeholder; got: %v", err)
	}
}

// TestJudgeOverride_LoadsCustomTemplate covers the happy-path
// override: valid template loads + replaces the default.
func TestJudgeOverride_LoadsCustomTemplate(t *testing.T) {
	tmp := t.TempDir()
	skillDir := filepath.Join(tmp, "skill")
	evalsDir := filepath.Join(skillDir, "evals")
	if err := os.MkdirAll(evalsDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	custom := "CUSTOM: grade {assertion} against {output}. Return JSON."
	if err := os.WriteFile(filepath.Join(evalsDir, "judge.md"), []byte(custom), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := LoadJudgeTemplate(skillDir)
	if err != nil {
		t.Fatalf("LoadJudgeTemplate: %v", err)
	}
	if got != custom {
		t.Errorf("expected custom template; got %q", got)
	}
}

// TestJudgeOverride_MissingFileFallsBackToDefault: no judge.md →
// returns DefaultJudgeTemplate without erroring.
func TestJudgeOverride_MissingFileFallsBackToDefault(t *testing.T) {
	tmp := t.TempDir()
	got, err := LoadJudgeTemplate(filepath.Join(tmp, "skill-with-no-evals"))
	if err != nil {
		t.Fatalf("missing override should not error: %v", err)
	}
	if got != DefaultJudgeTemplate {
		t.Errorf("expected default template fallback")
	}
}
