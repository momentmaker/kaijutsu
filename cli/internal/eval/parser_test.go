package eval

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestParseEvalsJSON_AgentSkillsEvalCompat parses each captured
// upstream fixture verbatim. Pinned shape: skill_name + evals[]
// (with optional files / expected_output / assertions). Drift in
// upstream schema surfaces here.
func TestParseEvalsJSON_AgentSkillsEvalCompat(t *testing.T) {
	cases := []struct {
		fixture          string
		wantSkillName    string
		wantEvalCount    int
		wantFirstEvalID  string
	}{
		{"upstream-readme-example.json", "pdf-processing", 2, "extract-table"},
		{"minimal.json", "minimal-skill", 1, "smoke"},
		{"multi-assertion.json", "multi-assertion-skill", 1, "complex-task"},
	}
	for _, tc := range cases {
		t.Run(tc.fixture, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("testdata/agent-skills-eval-fixtures/v1", tc.fixture))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			s, err := Parse(data)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if s.SkillName != tc.wantSkillName {
				t.Errorf("skill_name=%q, want %q", s.SkillName, tc.wantSkillName)
			}
			if len(s.Evals) != tc.wantEvalCount {
				t.Fatalf("evals len=%d, want %d", len(s.Evals), tc.wantEvalCount)
			}
			if s.Evals[0].ID != tc.wantFirstEvalID {
				t.Errorf("first eval id=%q, want %q", s.Evals[0].ID, tc.wantFirstEvalID)
			}
		})
	}
}

// TestParser_RejectsMismatchedSkillName: ParseAtPath enforces that
// the skill_name in evals.json matches the parent skill directory
// name. Wrong name → error mentioning both.
func TestParser_RejectsMismatchedSkillName(t *testing.T) {
	tmp := t.TempDir()
	skillDir := filepath.Join(tmp, "actual-skill-name")
	evalsDir := filepath.Join(skillDir, "evals")
	if err := os.MkdirAll(evalsDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	body := []byte(`{"skill_name":"different-name","evals":[{"id":"x","name":"X","prompt":"p"}]}`)
	path := filepath.Join(evalsDir, "evals.json")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := ParseAtPath(body, path)
	if err == nil {
		t.Fatal("expected mismatch error")
	}
	if !strings.Contains(err.Error(), "different-name") || !strings.Contains(err.Error(), "actual-skill-name") {
		t.Errorf("error should name both skill names; got: %v", err)
	}
}

// TestParser_RejectsDuplicateEvalIds covers the artifact-collision
// guard. Two evals with id=foo would write to the same eval-foo/
// directory.
func TestParser_RejectsDuplicateEvalIds(t *testing.T) {
	body := []byte(`{
		"skill_name": "skill",
		"evals": [
			{"id": "dup", "name": "A", "prompt": "p1"},
			{"id": "dup", "name": "B", "prompt": "p2"}
		]
	}`)
	_, err := Parse(body)
	if err == nil {
		t.Fatal("expected duplicate-id error")
	}
	if !strings.Contains(err.Error(), "duplicate id") {
		t.Errorf("error should mention 'duplicate id'; got: %v", err)
	}
}

// TestParser_RejectsMissingRequiredFields covers required-field
// validation: skill_name, evals (non-empty), per-eval id/name/prompt.
func TestParser_RejectsMissingRequiredFields(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{"missing skill_name", `{"evals":[{"id":"a","name":"A","prompt":"p"}]}`, "skill_name"},
		{"empty evals", `{"skill_name":"x","evals":[]}`, "non-empty"},
		{"missing eval id", `{"skill_name":"x","evals":[{"name":"A","prompt":"p"}]}`, "id is required"},
		{"missing eval name", `{"skill_name":"x","evals":[{"id":"a","prompt":"p"}]}`, "name is required"},
		{"missing eval prompt", `{"skill_name":"x","evals":[{"id":"a","name":"A"}]}`, "prompt is required"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse([]byte(tc.body))
			if err == nil {
				t.Fatalf("expected error containing %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q should contain %q", err.Error(), tc.want)
			}
		})
	}
}

// TestParser_AcceptsKaijutsuExtensionBlock confirms the v0.10.0
// schema parses kaijutsu.* blocks without rejecting them. Stage 1
// doesn't USE these fields; Stage 2 does. The shape contract holds
// from the start.
func TestParser_AcceptsKaijutsuExtensionBlock(t *testing.T) {
	body := []byte(`{
		"skill_name": "skill",
		"evals": [{"id": "a", "name": "A", "prompt": "p"}],
		"kaijutsu": {
			"swarm": [
				{"id": "s1", "preset": "pr-review",
				 "baseline": "without_skill", "challenger": "with_skill",
				 "diff": "f.diff", "assertions": ["a"]}
			]
		}
	}`)
	s, err := Parse(body)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if s.Kaijutsu == nil {
		t.Fatal("kaijutsu block should be populated")
	}
	if len(s.Kaijutsu.Swarm) != 1 {
		t.Errorf("swarm count=%d, want 1", len(s.Kaijutsu.Swarm))
	}
	if s.Kaijutsu.Swarm[0].Preset != "pr-review" {
		t.Errorf("swarm[0].preset=%q", s.Kaijutsu.Swarm[0].Preset)
	}
}

// TestExpectedOutputAutoPromotion_VerbatimWrap pins the literal
// byte-for-byte wrap. Plan-spec mandate (revised in plan
// doc-review): the wrap must be exactly
// `"Output should satisfy expected behavior: " + ExpectedOutput`.
// Any drift in the prefix breaks judge reproducibility across
// runs/versions.
func TestExpectedOutputAutoPromotion_VerbatimWrap(t *testing.T) {
	e := Eval{
		ID:             "x",
		Name:           "X",
		Prompt:         "p",
		ExpectedOutput: "the answer is 42",
	}
	got := ResolvedAssertions(e)
	want := []string{"Output should satisfy expected behavior: the answer is 42"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("auto-promoted assertion = %#v, want %#v", got, want)
	}
}

// TestResolvedAssertions_ExplicitWinsOverExpectedOutput covers the
// co-presence rule: when both populated, Assertions is authoritative
// and ExpectedOutput is ignored. Skill authors who wrote assertions
// meant them as the contract.
func TestResolvedAssertions_ExplicitWinsOverExpectedOutput(t *testing.T) {
	e := Eval{
		ID:             "x",
		Name:           "X",
		Prompt:         "p",
		ExpectedOutput: "this should be ignored",
		Assertions:     []string{"explicit assertion"},
	}
	got := ResolvedAssertions(e)
	want := []string{"explicit assertion"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("explicit assertion should win; got %#v, want %#v", got, want)
	}
}

// TestResolvedAssertions_NoneReturnsNil covers the empty case (no
// assertions, no expected_output) — caller treats nil as "no judge
// calls; this eval cannot fail".
func TestResolvedAssertions_NoneReturnsNil(t *testing.T) {
	e := Eval{ID: "x", Name: "X", Prompt: "p"}
	if got := ResolvedAssertions(e); got != nil {
		t.Errorf("no-assertion eval returned %#v, want nil", got)
	}
}
