package swarm

import (
	"context"

	"github.com/momentmaker/kaijutsu/cli/internal/agents"
)

// Agent is the legacy v0.5 dispatch interface preserved for swarm
// pipeline call sites (parallel.go, debate.go, cli/swarm.go's
// pickSynthesizer). v0.6 Stage 1 introduces agents.AgentDriver as
// the canonical interface; swarm.Agent is now a thin adapter that
// delegates to it.
//
// Removed in v0.7 once call sites migrate to AgentDriver directly.
type Agent interface {
	Name() AgentName
	// Run sends prompt to the agent and returns its raw stdout. The
	// caller is responsible for parsing — implementations should not
	// post-process beyond capturing the bytes.
	Run(ctx context.Context, prompt string, maxBudgetUSD float64) (raw string, err error)
}

// AgentFor returns the concrete Agent implementation for a name.
// Internally delegates to agents.For — this is the v0.6 driver
// abstraction. Returns nil for unknown names.
func AgentFor(name AgentName) Agent {
	drv := agents.For(string(name))
	if drv == nil {
		return nil
	}
	return &driverAdapter{name: name, driver: drv}
}

// driverAdapter wraps an agents.AgentDriver to satisfy the legacy
// swarm.Agent interface used by parallel.go and debate.go.
type driverAdapter struct {
	name   AgentName
	driver agents.AgentDriver
}

func (a *driverAdapter) Name() AgentName { return a.name }

func (a *driverAdapter) Run(ctx context.Context, prompt string, budget float64) (string, error) {
	res, err := a.driver.Invoke(ctx, prompt, agents.InvokeOpts{
		MaxBudgetUSD: budget,
	})
	return res.Raw, err
}
