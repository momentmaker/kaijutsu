// Package swarm orchestrates parallel runs of multiple agent CLIs
// (claude, codex, gemini) against a shared task, then synthesizes
// the results into a single output.
//
// Phase 1 ships one preset — pr-review. Phase 2 generalizes presets
// (brainstorm, refactor-plan, security-audit) on the same primitive.
package swarm

import "time"

// Severity classifies a finding. Ordered from most to least severe.
type Severity string

const (
	SeverityBlocker Severity = "blocker"
	SeverityIssue   Severity = "issue"
	SeverityMinor   Severity = "minor"
	SeverityInfo    Severity = "info"
)

// Finding is one structured review item from one agent.
type Finding struct {
	Severity   Severity `json:"severity"`
	File       string   `json:"file"`
	LineRange  string   `json:"line_range"` // "42" or "42-58"
	Summary    string   `json:"summary"`
	Reasoning  string   `json:"reasoning"`
	Confidence float64  `json:"confidence"` // 0.0–1.0
}

// AgentResult bundles one agent's output from one swarm pass.
type AgentResult struct {
	Agent    string    `json:"agent"`
	Findings []Finding `json:"findings"`
	Cost     float64   `json:"cost_usd_est"`
	Duration time.Duration `json:"duration_ms"`
	Err      string    `json:"error,omitempty"`
	Raw      string    `json:"-"` // not serialized; used for cache/replay
}

// SwarmRun is the top-level result of a single jutsu-swarm invocation.
// Stage 1 emits this as JSON; Stage 2 layers a synthesized markdown
// view on top.
type SwarmRun struct {
	Preset     string        `json:"preset"`
	PR         int           `json:"pr,omitempty"`
	SHA        string        `json:"sha,omitempty"`
	Mode       string        `json:"mode"`     // quick | full
	Agents     []AgentResult `json:"agents"`
	TotalCost  float64       `json:"total_cost_usd_est"`
	StartedAt  time.Time     `json:"started_at"`
	FinishedAt time.Time     `json:"finished_at"`
}
