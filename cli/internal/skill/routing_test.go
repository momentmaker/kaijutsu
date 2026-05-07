package skill

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestLoadRouting_HappyPath covers the v0.9 routing field: parses
// per-persona + default lists into the Routing struct.
func TestLoadRouting_HappyPath(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "skill.yaml")
	body := `
name: pr-review
version: 1.0.0
license: MIT
layout: flat
description: "x"
agents: [claude, codex, gemini]
permissions:
  bash: false
  network: false
  fs-write: false
routing:
  per-persona:
    honest: [claude]
    adversary: [claude, codex, gemini]
  default: [claude, codex, gemini]
`
	if err := os.WriteFile(p, []byte(body), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	s, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.Routing == nil {
		t.Fatal("Routing block missing after parse")
	}
	if got := s.Routing.PerPersona["honest"]; !reflect.DeepEqual(got, []string{"claude"}) {
		t.Errorf("PerPersona[honest] = %v, want [claude]", got)
	}
	if got := s.Routing.PerPersona["adversary"]; !reflect.DeepEqual(got, []string{"claude", "codex", "gemini"}) {
		t.Errorf("PerPersona[adversary] = %v, want [claude codex gemini]", got)
	}
	if got := s.Routing.Default; !reflect.DeepEqual(got, []string{"claude", "codex", "gemini"}) {
		t.Errorf("Default = %v, want [claude codex gemini]", got)
	}
}

// TestLoadRouting_OmittedFieldNil verifies skills without the routing
// block continue to load (backward-compat) — Routing remains nil.
func TestLoadRouting_OmittedFieldNil(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "skill.yaml")
	body := `
name: pr-review
version: 1.0.0
license: MIT
layout: flat
description: "x"
agents: [claude]
permissions:
  bash: false
  network: false
  fs-write: false
`
	if err := os.WriteFile(p, []byte(body), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	s, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.Routing != nil {
		t.Errorf("Routing should be nil when block omitted; got %+v", s.Routing)
	}
}

// TestLoadRouting_EmptyDefaultEqualsOmittedDefault confirms the spec
// rule: empty list AND missing field are equivalent at the YAML
// parse layer (both produce nil Default — Go's zero value for the
// slice). Downstream Phase A logic treats both identically.
func TestLoadRouting_EmptyDefaultEqualsOmittedDefault(t *testing.T) {
	tmp := t.TempDir()
	bodyEmpty := `
name: pr-review
version: 1.0.0
license: MIT
layout: flat
description: "x"
agents: [claude]
permissions:
  bash: false
  network: false
  fs-write: false
routing:
  default: []
`
	bodyOmitted := `
name: pr-review
version: 1.0.0
license: MIT
layout: flat
description: "x"
agents: [claude]
permissions:
  bash: false
  network: false
  fs-write: false
routing:
  per-persona:
    honest: [claude]
`
	pa := filepath.Join(tmp, "a.yaml")
	pb := filepath.Join(tmp, "b.yaml")
	_ = os.WriteFile(pa, []byte(bodyEmpty), 0644)
	_ = os.WriteFile(pb, []byte(bodyOmitted), 0644)
	a, errA := Load(pa)
	if errA != nil {
		t.Fatalf("Load empty: %v", errA)
	}
	b, errB := Load(pb)
	if errB != nil {
		t.Fatalf("Load omitted: %v", errB)
	}
	if a.Routing == nil {
		t.Fatal("a.Routing should be non-nil (block present)")
	}
	if b.Routing == nil {
		t.Fatal("b.Routing should be non-nil (block present, default omitted)")
	}
	if len(a.Routing.Default) != 0 {
		t.Errorf("empty Default should len=0; got %v", a.Routing.Default)
	}
	if len(b.Routing.Default) != 0 {
		t.Errorf("omitted Default should len=0; got %v", b.Routing.Default)
	}
}
