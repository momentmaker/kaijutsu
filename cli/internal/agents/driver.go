// Package agents defines the AgentDriver abstraction used by jutsu
// swarm to dispatch to LLM CLIs, HTTP API providers, MCP servers,
// and CLI-as-harness wrappers.
//
// v0.6 Stage 1 introduces this package as a pure refactor — the only
// driver kind populated is `cli` (claude / codex / gemini), preserving
// v0.5 byte-identical behavior. Stages 3-6 add http, cli-compat, and
// mcp drivers.
//
// See docs/specs/2026-05-05-v0.6.0-multi-provider-agents.md for the
// full surface, and docs/decisions/2026-05-05-driver-abstraction.md
// for the why.
package agents

import (
	"context"
	"time"
)

// DriverKind names the dispatch mechanism a provider uses.
type DriverKind string

const (
	DriverCLI       DriverKind = "cli"
	DriverHTTP      DriverKind = "http"
	DriverCLICompat DriverKind = "cli-compat"
	DriverMCP       DriverKind = "mcp"
)

// CacheStatus reports prompt-cache outcome for HTTP-driver invocations.
// Non-HTTP drivers report CacheUnsupported.
type CacheStatus string

const (
	CacheUnsupported  CacheStatus = "unsupported"
	CacheHit          CacheStatus = "hit"
	CacheMiss         CacheStatus = "miss"
	CachePartial      CacheStatus = "partial"
	CacheSkippedShort CacheStatus = "skipped-too-short"
)

// InvokeOpts controls a single AgentDriver.Invoke call.
//
// MaxBudgetUSD is a Stage 1 transition field preserved from v0.5; the
// claude cli driver passes it as --max-budget-usd. Stage 4 will drop
// this in favor of aggregate cost enforcement in the swarm pipeline
// (see Non-goals in the v0.6.0 spec).
type InvokeOpts struct {
	Timeout      time.Duration
	MaxBudgetUSD float64
}

// Result is the structured output of one Invoke call.
type Result struct {
	Raw         string
	CostUSD     float64
	Duration    time.Duration
	Driver      DriverKind
	Err         string
	CacheStatus CacheStatus
}

// AgentDriver is the v0.6 dispatch interface. All swarm participants
// (CLI binaries, HTTP API providers, MCP servers, cli-compat wrappers)
// implement it.
type AgentDriver interface {
	Name() string
	Driver() DriverKind
	Invoke(ctx context.Context, prompt string, opts InvokeOpts) (Result, error)
}
