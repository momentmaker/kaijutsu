package swarm

import (
	"context"
	"testing"

	"github.com/momentmaker/kaijutsu/cli/internal/agents"
)

// fakeDriver satisfies agents.AgentDriver for unit tests without
// requiring a real CLI binary or HTTP endpoint.
type fakeDriver struct {
	name string
	kind agents.DriverKind
}

func (f *fakeDriver) Name() string                                                                   { return f.name }
func (f *fakeDriver) Driver() agents.DriverKind                                                      { return f.kind }
func (f *fakeDriver) Invoke(_ context.Context, _ string, _ agents.InvokeOpts) (agents.Result, error) { return agents.Result{}, nil }

func TestPickPersonaSynthesizer_PrefersWantWhenMatchingAndNoErr(t *testing.T) {
	personas := []*PersonaAdapter{
		{PersonaName: "default-claude", Driver: &fakeDriver{name: "claude", kind: agents.DriverCLI}},
		{PersonaName: "default-gemini", Driver: &fakeDriver{name: "gemini", kind: agents.DriverCLI}},
	}
	results := []AgentResult{
		{Agent: "default-claude", Err: ""},
		{Agent: "default-gemini", Err: ""},
	}
	got := PickPersonaSynthesizer("default-gemini", results, personas)
	if got == nil || string(got.Name()) != "default-gemini" {
		t.Errorf("synthesizer = %v, want default-gemini", got)
	}
}

func TestPickPersonaSynthesizer_FallsBackOnWantErrored(t *testing.T) {
	personas := []*PersonaAdapter{
		{PersonaName: "default-claude", Driver: &fakeDriver{name: "claude", kind: agents.DriverCLI}},
		{PersonaName: "default-gemini", Driver: &fakeDriver{name: "gemini", kind: agents.DriverCLI}},
	}
	results := []AgentResult{
		{Agent: "default-claude", Err: "boom"},
		{Agent: "default-gemini", Err: ""},
	}
	got := PickPersonaSynthesizer("default-claude", results, personas)
	if got == nil || string(got.Name()) != "default-gemini" {
		t.Errorf("synthesizer = %v, want default-gemini fallback", got)
	}
}

func TestPickPersonaSynthesizer_AllErroredReturnsNil(t *testing.T) {
	personas := []*PersonaAdapter{
		{PersonaName: "p1", Driver: &fakeDriver{name: "x", kind: agents.DriverCLI}},
	}
	results := []AgentResult{{Agent: "p1", Err: "boom"}}
	got := PickPersonaSynthesizer("", results, personas)
	if got != nil {
		t.Errorf("synthesizer = %v, want nil", got)
	}
}

func TestPersonaAdapter_NameSurfacesAsAgentName(t *testing.T) {
	a := &PersonaAdapter{PersonaName: "weird-but-valid", Driver: &fakeDriver{name: "x", kind: agents.DriverCLI}}
	if string(a.Name()) != "weird-but-valid" {
		t.Errorf("Name() = %q, want %q", a.Name(), "weird-but-valid")
	}
}

// fakeCostDriver returns a configurable Result.CostUSD on every Invoke.
type fakeCostDriver struct{ cost float64 }

func (f *fakeCostDriver) Name() string              { return "fake" }
func (f *fakeCostDriver) Driver() agents.DriverKind { return agents.DriverHTTP }
func (f *fakeCostDriver) Invoke(_ context.Context, _ string, _ agents.InvokeOpts) (agents.Result, error) {
	return agents.Result{Raw: "ok", CostUSD: f.cost, Driver: agents.DriverHTTP, CacheStatus: agents.CacheMiss}, nil
}

func TestOverlayPersonaCosts_OverridesEstimateForHTTPDrivers(t *testing.T) {
	a := &PersonaAdapter{PersonaName: "p1", Driver: &fakeCostDriver{cost: 0.42}}
	if _, err := a.Run(context.Background(), "x", 0); err != nil {
		t.Fatal(err)
	}
	results := []AgentResult{{Agent: "p1", Cost: 0.01}}
	OverlayPersonaCosts(results, []*PersonaAdapter{a})
	if results[0].Cost != 0.42 {
		t.Errorf("Cost = %v, want 0.42 (real cost should override estimate)", results[0].Cost)
	}
}

func TestOverlayPersonaCosts_PreservesEstimateWhenDriverReportsZero(t *testing.T) {
	a := &PersonaAdapter{PersonaName: "p1", Driver: &fakeCostDriver{cost: 0}}
	if _, err := a.Run(context.Background(), "x", 0); err != nil {
		t.Fatal(err)
	}
	results := []AgentResult{{Agent: "p1", Cost: 0.05}}
	OverlayPersonaCosts(results, []*PersonaAdapter{a})
	if results[0].Cost != 0.05 {
		t.Errorf("Cost = %v, want 0.05 (estimate should survive when driver reports 0)", results[0].Cost)
	}
}

func TestOverlayPersonaCosts_IgnoresUnmappedResults(t *testing.T) {
	a := &PersonaAdapter{PersonaName: "p1", Driver: &fakeCostDriver{cost: 99}}
	if _, err := a.Run(context.Background(), "x", 0); err != nil {
		t.Fatal(err)
	}
	results := []AgentResult{
		{Agent: "p1", Cost: 0.01},
		{Agent: "no-match", Cost: 0.02},
	}
	OverlayPersonaCosts(results, []*PersonaAdapter{a})
	if results[0].Cost != 99 {
		t.Errorf("results[0].Cost = %v, want 99", results[0].Cost)
	}
	if results[1].Cost != 0.02 {
		t.Errorf("results[1].Cost = %v, want 0.02 (unmapped should be untouched)", results[1].Cost)
	}
}

func TestUserSentinel_StableValue(t *testing.T) {
	// The swarm UserSentinel must match the constant the
	// agents/http_driver.go uses for splitSystemUser. Both packages
	// share the literal value "\n\n<<<USER>>>\n\n"; this test guards
	// the swarm-side constant. agents.userSentinel is unexported so
	// a true cross-package assertion isn't possible without exposing
	// the constant — this test at least pins the swarm side.
	const expected = "\n\n<<<USER>>>\n\n"
	if UserSentinel != expected {
		t.Errorf("UserSentinel = %q, want %q", UserSentinel, expected)
	}
}
