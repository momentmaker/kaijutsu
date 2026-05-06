// Package swarm orchestrates parallel runs of multiple agent CLIs
// (claude, codex, gemini) against a shared task, then synthesizes
// the results into a single output.
//
// Phase 1 ships one preset — pr-review. Phase 2 generalizes presets
// (brainstorm, refactor-plan, security-audit) on the same primitive.
package swarm

import "time"

// Severity classifies a finding. Different presets use different
// vocabularies (declared via Preset.SeverityVocab); the orchestrator
// preserves whichever string the preset's agents emit, only
// normalizing case and stripping unknowns to "info".
type Severity string

// Review-flavored vocab (pr-review, doc-review).
const (
	SeverityBlocker Severity = "blocker"
	SeverityIssue   Severity = "issue"
	SeverityMinor   Severity = "minor"
	SeverityInfo    Severity = "info"
)

// CVSS-aligned vocab (security-audit). Distinct from "blocker/issue/
// minor/info" — security-audit's threat model uses the CVSS terms
// the InfoSec community already speaks.
const (
	SeverityCritical      Severity = "critical"
	SeverityHigh          Severity = "high"
	SeverityMedium        Severity = "medium"
	SeverityLow           Severity = "low"
	SeverityInformational Severity = "informational"
)

// Brainstorm-flavored vocab (brainstorm, refactor-plan). Different
// shape because the output is "options to pick" not "issues to fix".
const (
	SeverityRecommended Severity = "recommended"
	SeverityAlternative Severity = "alternative"
	SeverityRisky       Severity = "risky"
	SeveritySpeculative Severity = "speculative"
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
	// Duration of the agent invocation. Encoded in JSON as int64
	// nanoseconds (Go's default for time.Duration); divide by 1e6
	// to get milliseconds.
	Duration time.Duration `json:"duration_ns"`
	Err      string    `json:"error,omitempty"`
	Raw      string    `json:"-"` // not serialized; used for cache/replay
	// Driver kind that produced this result ("cli" | "http" |
	// "cli-compat" | "mcp"). Populated by the persona-mode pipeline
	// in cli/swarm.go; empty for legacy v0.5 cli-only path. Used by
	// the synthesizer to tag deterministic-peer findings (mcp) and
	// apply the info-severity floor for mcp-only entries.
	Driver string `json:"driver,omitempty"`
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
