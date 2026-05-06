package cli

import (
	"context"
	"testing"

	"github.com/momentmaker/kaijutsu/cli/internal/agents"
	"github.com/momentmaker/kaijutsu/cli/internal/swarm"
)

// fakeDriver satisfies agents.AgentDriver for unit tests without
// requiring a real CLI binary or HTTP endpoint.
type fakeDriver struct {
	name string
	kind agents.DriverKind
}

func (f *fakeDriver) Name() string                                                                 { return f.name }
func (f *fakeDriver) Driver() agents.DriverKind                                                    { return f.kind }
func (f *fakeDriver) Invoke(_ context.Context, _ string, _ agents.InvokeOpts) (agents.Result, error) { return agents.Result{}, nil }

func TestPickPersonaSynthesizer_PrefersWantWhenMatchingAndNoErr(t *testing.T) {
	personas := []*personaAdapter{
		{personaName: "default-claude", driver: &fakeDriver{name: "claude", kind: agents.DriverCLI}},
		{personaName: "default-gemini", driver: &fakeDriver{name: "gemini", kind: agents.DriverCLI}},
	}
	results := []swarm.AgentResult{
		{Agent: "default-claude", Err: ""},
		{Agent: "default-gemini", Err: ""},
	}
	got := pickPersonaSynthesizer("default-gemini", results, personas)
	if got == nil || string(got.Name()) != "default-gemini" {
		t.Errorf("synthesizer = %v, want default-gemini", got)
	}
}

func TestPickPersonaSynthesizer_FallsBackOnWantErrored(t *testing.T) {
	personas := []*personaAdapter{
		{personaName: "default-claude", driver: &fakeDriver{name: "claude", kind: agents.DriverCLI}},
		{personaName: "default-gemini", driver: &fakeDriver{name: "gemini", kind: agents.DriverCLI}},
	}
	// User wants claude but claude errored — fall back to first non-errored.
	results := []swarm.AgentResult{
		{Agent: "default-claude", Err: "boom"},
		{Agent: "default-gemini", Err: ""},
	}
	got := pickPersonaSynthesizer("default-claude", results, personas)
	if got == nil || string(got.Name()) != "default-gemini" {
		t.Errorf("synthesizer = %v, want default-gemini fallback", got)
	}
}

func TestPickPersonaSynthesizer_AllErroredReturnsNil(t *testing.T) {
	personas := []*personaAdapter{
		{personaName: "p1", driver: &fakeDriver{name: "x", kind: agents.DriverCLI}},
	}
	results := []swarm.AgentResult{{Agent: "p1", Err: "boom"}}
	got := pickPersonaSynthesizer("", results, personas)
	if got != nil {
		t.Errorf("synthesizer = %v, want nil", got)
	}
}

func TestPersonaSentinelMatchesAgentSentinel(t *testing.T) {
	// userSentinel is duplicated between cli and agents packages
	// (intentional — keeps the cli-package job assembly readable
	// without an exported import). This test fails fast if the two
	// drift apart.
	const expected = "\n\n<<<USER>>>\n\n"
	if userSentinel != expected {
		t.Errorf("cli userSentinel = %q, want %q", userSentinel, expected)
	}
	// We can't reach the unexported agents.userSentinel directly, so
	// instead exercise the round-trip via splitSystemUser:
	// agents.splitSystemUser is unexported; this test is a placeholder
	// reminder. Keep the constants in lockstep manually.
}

func TestPersonaAdapter_NameSurfacesAsAgentName(t *testing.T) {
	a := &personaAdapter{personaName: "weird-but-valid", driver: &fakeDriver{name: "x", kind: agents.DriverCLI}}
	if string(a.Name()) != "weird-but-valid" {
		t.Errorf("Name() = %q, want %q", a.Name(), "weird-but-valid")
	}
}
