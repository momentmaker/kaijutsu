package eval

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// Parse validates evals.json bytes and returns a *Suite. Cross-field
// rules:
//   - skill_name: required, non-empty.
//   - evals: required, non-empty, all (id, name, prompt) populated;
//     duplicate ids in the upstream evals[] array are rejected at
//     parse time so artifact directories don't collide.
//
// Parser does NOT verify skill_name matches a parent directory —
// callers (cli) handle that via ParseAtPath which knows the path.
// Stage 1 contract: this function is path-agnostic so library
// callers (tests, future programmatic embedders) can parse from
// arbitrary byte sources.
func Parse(data []byte) (*Suite, error) {
	var s Suite
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("decode evals.json: %w", err)
	}
	if err := validateSuite(&s); err != nil {
		return nil, err
	}
	return &s, nil
}

// ParseAtPath wraps Parse with a directory-context check: the suite's
// skill_name must match the parent directory of the evals.json file.
// Match is case-sensitive (skill names are lowercase per spec). The
// evalsJSONPath argument is the absolute path to evals.json itself,
// not its parent.
func ParseAtPath(data []byte, evalsJSONPath string) (*Suite, error) {
	s, err := Parse(data)
	if err != nil {
		return nil, err
	}
	// Parent of evals.json is `evals/`; parent of that is the skill
	// directory whose basename is the canonical skill name.
	skillDir := filepath.Dir(filepath.Dir(evalsJSONPath))
	if skillDir != "" && skillDir != "." && skillDir != "/" {
		base := filepath.Base(skillDir)
		if base != s.SkillName {
			return nil, fmt.Errorf("skill_name mismatch: evals.json declares %q but parent skill directory is %q", s.SkillName, base)
		}
	}
	return s, nil
}

// validateSuite enforces top-level schema rules. Returns the FIRST
// error encountered (no aggregation) so authors fix one issue at a
// time rather than playing whack-a-mole with cascading messages.
func validateSuite(s *Suite) error {
	if s == nil {
		return errors.New("nil suite")
	}
	if strings.TrimSpace(s.SkillName) == "" {
		return errors.New("skill_name is required")
	}
	if len(s.Evals) == 0 {
		return errors.New("evals array must be non-empty")
	}
	seenID := make(map[string]bool, len(s.Evals))
	for i, e := range s.Evals {
		if strings.TrimSpace(e.ID) == "" {
			return fmt.Errorf("evals[%d]: id is required", i)
		}
		if strings.TrimSpace(e.Name) == "" {
			return fmt.Errorf("evals[%d] (id=%s): name is required", i, e.ID)
		}
		if strings.TrimSpace(e.Prompt) == "" {
			return fmt.Errorf("evals[%d] (id=%s): prompt is required", i, e.ID)
		}
		if seenID[e.ID] {
			return fmt.Errorf("evals[%d]: duplicate id %q (artifact dirs would collide)", i, e.ID)
		}
		seenID[e.ID] = true
	}
	return nil
}

// ResolvedAssertions returns the effective assertions for an eval —
// either the explicit Assertions array OR a single synthesized
// assertion from ExpectedOutput. Auto-promotion rule (spec §2):
//
//	if len(Assertions) == 0 && ExpectedOutput != "":
//	    return ["Output should satisfy expected behavior: <ExpectedOutput>"]
//
// Co-presence: when BOTH Assertions and ExpectedOutput are populated,
// Assertions wins (no merge, no auto-append). Spec §"expected_output
// auto-promotion" is silent on co-presence; we pick "explicit
// assertions are authoritative" because skill authors who wrote
// assertions clearly meant them as the contract.
func ResolvedAssertions(e Eval) []string {
	if len(e.Assertions) > 0 {
		return e.Assertions
	}
	if strings.TrimSpace(e.ExpectedOutput) != "" {
		return []string{
			"Output should satisfy expected behavior: " + e.ExpectedOutput,
		}
	}
	return nil
}
