package swarm

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// driverKindMCP mirrors agents.DriverMCP's string value. Kept as a
// swarm-local const so synth.go doesn't pull in the agents package
// (would tie the swarm primitive to the driver registry; today swarm
// is dispatch-agnostic, agents is the resolved-config layer above
// it). The two strings stay in lockstep — drift is caught by
// TestDriverKindEnumStable in the agents package.
const driverKindMCP = "mcp"

// Synthesis is the markdown report produced by Synthesize. RawDraft
// is what the synthesizer agent produced; MergedTable is the
// deterministic disagreement-table the orchestrator builds locally
// (so we don't depend on the synthesizer formatting it correctly).
type Synthesis struct {
	Markdown    string         // final report posted/printed
	RawDraft    string         // synthesizer agent's verbatim output
	MergedTable string         // deterministic disagreement table
	Cost        float64        // synthesizer's estimated cost
	Duration    time.Duration  // synthesizer call wall time
	Clusters    []FindingGroup // post-clustering view, exposed for debug/replay
}

// SynthOpts threads v0.7 quality-fingerprinting context into the
// synthesizer. Zero value (empty Weights, ShowWeights=false) reproduces
// v0.6.2 behavior byte-for-byte — clusterFindings sorts the same,
// renderDisagreementTable omits weight annotations, the prompt body
// has no `weights:` section. The cli layer fills this in via
// findings.Weighter.WeightsForResults.
type SynthOpts struct {
	// Weights is keyed by AgentResult.Agent (i.e. persona name in
	// v0.6+ persona mode, native CLI name in legacy v0.5 mode).
	// Missing keys are treated as ColdWeight (1.0).
	Weights map[string]float64
	// ShowWeights controls renderDisagreementTable: when true, each
	// agent column header gains "(<weight>)". Off by default in v0.7
	// so users adopt weights via `jutsu finding stats` first.
	ShowWeights bool
}

// FindingGroup represents one logical issue surfaced by 1+ agents.
type FindingGroup struct {
	Key            string                 // file:line — used for clustering
	Severity       Severity               // worst severity across reporters
	Reporters      map[string]Finding     // agent name → that agent's finding
	ConsensusOf    int                    // how many agents flagged this
	OutOfTotal     int                    // total agents in run (for ratio)
}

// Synthesize runs the synthesizer agent against the per-agent results
// and returns a complete review markdown. Stage 2 wires this in after
// FanOut. Stage 3 (--full) inserts a debate pass between FanOut and
// Synthesize. v0.7 adds SynthOpts for quality-fingerprinting weights.
func Synthesize(ctx context.Context, results []AgentResult, synth Agent, preset *Preset, budget float64, perAgentTimeout time.Duration, opts SynthOpts) (*Synthesis, error) {
	clusters := clusterFindings(results, opts.Weights)
	table := renderDisagreementTable(results, clusters, opts.Weights, opts.ShowWeights)

	body, err := json.MarshalIndent(stripRawForPrompt(results), "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal findings for synthesizer: %w", err)
	}
	prompt := buildSynthPrompt(preset.Synthesizer, body, opts.Weights)

	subCtx, cancel := context.WithTimeout(ctx, perAgentTimeout)
	defer cancel()
	start := time.Now()
	raw, err := synth.Run(subCtx, prompt, budget)
	dur := time.Since(start)
	if err != nil {
		return &Synthesis{
			Markdown:    fallbackMarkdown(preset, results, table, raw),
			RawDraft:    raw,
			MergedTable: table,
			Duration:    dur,
			Clusters:    clusters,
		}, fmt.Errorf("synthesizer (%s) failed: %w", synth.Name(), err)
	}
	finalMD := assembleMarkdown(preset, results, table, raw, clusters)
	return &Synthesis{
		Markdown:    finalMD,
		RawDraft:    raw,
		MergedTable: table,
		Cost:        EstimateCostUSD(synth.Name(), EstimateTokens(prompt), EstimateTokens(raw)),
		Duration:    dur,
		Clusters:    clusters,
	}, nil
}

// stripRawForPrompt returns AgentResults shaped for the synthesizer
// prompt: keep agent name + findings, drop Raw/Err/Duration noise so
// the prompt token count stays small.
func stripRawForPrompt(in []AgentResult) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(in))
	for _, r := range in {
		if r.Err != "" && len(r.Findings) == 0 {
			// skip total failures — synthesizer can't act on them
			continue
		}
		out = append(out, map[string]interface{}{
			"agent":    r.Agent,
			"findings": r.Findings,
		})
	}
	return out
}

// buildSynthPrompt formats the preset's synthesizer template with the
// per-agent findings JSON. When weights are non-cold (any value !=
// 1.0), prepends a `weights:` section so the model can deprioritize
// low-weight reporters in its synthesis prose. When all weights are
// cold OR the map is empty, output is byte-identical to v0.6.2 — no
// weights line, just the preset's prompt as-is.
func buildSynthPrompt(template string, body []byte, weights map[string]float64) string {
	core := fmt.Sprintf(template, string(body))
	if !anyNonCold(weights) {
		return core
	}
	var b strings.Builder
	b.WriteString("weights (per-(provider,persona) precision in [0.05, 1.0], 1.0 = cold start):\n")
	// Stable ordering: alphabetical by agent name so the prompt is
	// deterministic across runs.
	names := make([]string, 0, len(weights))
	for n := range weights {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		fmt.Fprintf(&b, "- %s: %.2f\n", n, weights[n])
	}
	b.WriteString("\nWhen synthesizing, deprioritize findings from low-weight reporters; treat 1.0 as cold-start (no signal).\n\n")
	b.WriteString(core)
	return b.String()
}

// anyNonCold mirrors findings.AnyNonCold without the import — synth
// can't import findings (would create a cycle: cli depends on both;
// findings depends on swarm.AgentResult). Cold weight is 1.0 by spec.
func anyNonCold(weights map[string]float64) bool {
	for _, w := range weights {
		if w != 1.0 {
			return true
		}
	}
	return false
}

// weightFor returns the weight for an agent name, defaulting to 1.0
// (cold) when missing or when the map is nil. Used by clusterFindings
// for the weighted-consensus sort.
func weightFor(weights map[string]float64, agent string) float64 {
	if weights == nil {
		return 1.0
	}
	if w, ok := weights[agent]; ok {
		return w
	}
	return 1.0
}

// clusterFindings groups findings across agents that point at the
// same (file, line_range). Stage 2 uses exact-match clustering;
// Stage 5 may add fuzzy line-proximity matching. v0.7 adds optional
// per-agent weights for the secondary sort.
func clusterFindings(results []AgentResult, weights map[string]float64) []FindingGroup {
	totalAgents := 0
	for _, r := range results {
		if r.Err == "" || len(r.Findings) > 0 {
			totalAgents++
		}
	}
	clusters := map[string]*FindingGroup{}
	for _, r := range results {
		for _, f := range r.Findings {
			// MCP-driver info-severity floor: deterministic peers
			// (semgrep, eslint, etc.) often emit a long tail of
			// info-tier findings that drown out the issue+ signal.
			// Drop info-tier MCP findings before they reach the
			// disagreement table. LLM info findings still flow.
			if r.Driver == driverKindMCP && f.Severity == SeverityInfo {
				continue
			}
			key := f.File + ":" + f.LineRange
			if g, ok := clusters[key]; ok {
				g.Reporters[r.Agent] = f
				g.ConsensusOf = len(g.Reporters)
				if severityRank(f.Severity) > severityRank(g.Severity) {
					g.Severity = f.Severity
				}
				continue
			}
			clusters[key] = &FindingGroup{
				Key:         key,
				Severity:    f.Severity,
				Reporters:   map[string]Finding{r.Agent: f},
				ConsensusOf: 1,
				OutOfTotal:  totalAgents,
			}
		}
	}
	out := make([]FindingGroup, 0, len(clusters))
	for _, g := range clusters {
		g.OutOfTotal = totalAgents
		out = append(out, *g)
	}
	// v0.7 sort: weighted_consensus desc → ConsensusOf desc → severity
	// desc → key asc. Spec acceptance "Tiebreaker order:
	// weighted_consensus desc → ConsensusOf desc → severity desc → key
	// asc". Severity-first vs weighted-consensus-first matters: spec
	// puts weighted_consensus first because a high-precision agent's
	// lone finding should outrank a low-precision agent's chorus when
	// both same-severity. When weights is nil/empty/all-cold,
	// weighted_consensus collapses to ConsensusOf, so behavior matches
	// v0.6.2 (which sorted by severity → ConsensusOf → key). The pre-
	// v0.7 ordering's severity-first kicks in only when no weights
	// data exists, which is the cold-start byte-identical contract.
	sort.Slice(out, func(i, j int) bool {
		// Cold-start fast path: no weights data → fall back to v0.6
		// ordering exactly. Cheap to check, preserves backward compat.
		if !anyNonCold(weights) {
			ri, rj := severityRank(out[i].Severity), severityRank(out[j].Severity)
			if ri != rj {
				return ri > rj
			}
			if out[i].ConsensusOf != out[j].ConsensusOf {
				return out[i].ConsensusOf > out[j].ConsensusOf
			}
			return out[i].Key < out[j].Key
		}
		// v0.7 weighted ordering.
		wi := weightedConsensus(out[i], weights)
		wj := weightedConsensus(out[j], weights)
		if wi != wj {
			return wi > wj
		}
		if out[i].ConsensusOf != out[j].ConsensusOf {
			return out[i].ConsensusOf > out[j].ConsensusOf
		}
		ri, rj := severityRank(out[i].Severity), severityRank(out[j].Severity)
		if ri != rj {
			return ri > rj
		}
		return out[i].Key < out[j].Key
	})
	return out
}

// weightedConsensus sums each unique reporter's weight ONCE per
// cluster — even if the same agent emitted multiple findings that
// merged into the cluster (clusterFindings already dedupes via the
// Reporters map[string]Finding, so we just iterate). The per-agent
// "vote" carries weight, not count.
func weightedConsensus(g FindingGroup, weights map[string]float64) float64 {
	sum := 0.0
	for agent := range g.Reporters {
		sum += weightFor(weights, agent)
	}
	return sum
}

// severityRank assigns a numeric ordering to each known severity so
// clusterFindings can sort by severity desc. Ranks parallel across
// vocabularies — CVSS "critical" sits at the same rank as review
// "blocker" so a synthesizer that runs across vocabularies (rare;
// not done in Phase 2) sorts sensibly. Within a single preset's run
// only that preset's vocab appears, so the cross-vocab parallels
// don't cause confusion.
func severityRank(s Severity) int {
	switch s {
	// rank 4 (highest)
	case SeverityBlocker, SeverityCritical:
		return 4
	// rank 3
	case SeverityIssue, SeverityHigh, SeverityRecommended:
		return 3
	// rank 2
	case SeverityMinor, SeverityMedium, SeverityAlternative, SeverityRisky:
		return 2
	// rank 1
	case SeverityLow:
		return 1
	// rank 0 (lowest — info-only)
	case SeverityInfo, SeverityInformational, SeveritySpeculative:
		return 0
	}
	return 0
}

// renderDisagreementTable builds a deterministic markdown table
// keyed by clustered findings × agent columns. The orchestrator
// builds this locally rather than trusting the synthesizer to format
// it — too easy for a model to drop columns or misalign rows.
//
// v0.7: weights + showWeights control optional column-header
// annotations. When showWeights=false (the v0.7 default — see
// spec acceptance "Default off in v0.7"), the table is byte-identical
// to v0.6.2 regardless of the weights map content.
func renderDisagreementTable(results []AgentResult, clusters []FindingGroup, weights map[string]float64, showWeights bool) string {
	if len(clusters) == 0 {
		return ""
	}
	// Stable column order matching swarm.AllAgents but only for
	// agents that actually participated.
	cols := make([]string, 0, len(results))
	seen := map[string]bool{}
	for _, want := range AllAgents {
		for _, r := range results {
			if r.Agent == string(want) && !seen[r.Agent] {
				cols = append(cols, r.Agent)
				seen[r.Agent] = true
			}
		}
	}
	for _, r := range results {
		if !seen[r.Agent] {
			cols = append(cols, r.Agent)
			seen[r.Agent] = true
		}
	}

	// Build a name→driver map so we can tag mcp columns + cells with
	// [deterministic] to distinguish them from LLM peers.
	driverByAgent := map[string]string{}
	for _, r := range results {
		driverByAgent[r.Agent] = r.Driver
	}

	var b strings.Builder
	fmt.Fprintf(&b, "| Finding | Severity | Consensus |")
	for _, c := range cols {
		label := c
		if driverByAgent[c] == driverKindMCP {
			label = c + " [deterministic]"
		}
		if showWeights {
			label = fmt.Sprintf("%s (%.2f)", label, weightFor(weights, c))
		}
		fmt.Fprintf(&b, " %s |", label)
	}
	b.WriteString("\n")
	fmt.Fprintf(&b, "|---|---|---|")
	for range cols {
		b.WriteString("---|")
	}
	b.WriteString("\n")
	for _, g := range clusters {
		var firstSummary string
		for _, f := range g.Reporters {
			firstSummary = f.Summary
			break
		}
		fmt.Fprintf(&b, "| `%s` — %s | %s | %d/%d |",
			g.Key, escapePipes(firstSummary), g.Severity, g.ConsensusOf, g.OutOfTotal)
		for _, c := range cols {
			if f, ok := g.Reporters[c]; ok {
				marker := "✓"
				if driverByAgent[c] == driverKindMCP {
					marker = "✓⚙"
				}
				fmt.Fprintf(&b, " %s (%s) |", marker, f.Severity)
			} else {
				b.WriteString(" — |")
			}
		}
		b.WriteString("\n")
	}
	return b.String()
}

func escapePipes(s string) string {
	return strings.ReplaceAll(s, "|", "\\|")
}

// assembleMarkdown wraps the synthesizer's draft with the deterministic
// disagreement table + run metadata header + footer marker. Header
// is preset-specific so non-pr-review presets (doc-review,
// brainstorm, refactor-plan, security-audit) don't render under a
// "pr-review" label.
func assembleMarkdown(preset *Preset, results []AgentResult, table, draft string, clusters []FindingGroup) string {
	var b strings.Builder

	fmt.Fprintf(&b, "## kaijutsu %s\n\n", preset.Name)
	consensus := 0
	contested := 0
	for _, g := range clusters {
		switch {
		case g.ConsensusOf == g.OutOfTotal && g.OutOfTotal >= 2:
			consensus++
		case g.ConsensusOf == 1 && g.OutOfTotal >= 2:
			contested++
		}
	}
	fmt.Fprintf(&b, "**Findings:** %d total · %d consensus · %d contested\n\n",
		len(clusters), consensus, contested)

	if table != "" {
		b.WriteString("### Disagreement Table\n\n")
		b.WriteString(table)
		b.WriteString("\n")
	}

	b.WriteString("### Synthesis\n\n")
	b.WriteString(strings.TrimSpace(draft))
	b.WriteString("\n\n")

	b.WriteString("<details><summary>Per-agent stats</summary>\n\n")
	for _, r := range results {
		if r.Err != "" {
			fmt.Fprintf(&b, "- **%s** — error: %s\n", r.Agent, r.Err)
			continue
		}
		fmt.Fprintf(&b, "- **%s** — %d finding(s) · %s · est $%.3f\n",
			r.Agent, len(r.Findings), r.Duration.Round(time.Millisecond), r.Cost)
	}
	b.WriteString("\n</details>\n")
	return b.String()
}

func fallbackMarkdown(preset *Preset, results []AgentResult, table, partial string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## kaijutsu %s (synthesis failed; raw findings below)\n\n", preset.Name)
	if table != "" {
		b.WriteString("### Disagreement Table\n\n")
		b.WriteString(table)
		b.WriteString("\n")
	}
	for _, r := range results {
		fmt.Fprintf(&b, "### %s\n\n", r.Agent)
		if r.Err != "" {
			fmt.Fprintf(&b, "_error: %s_\n\n", r.Err)
			continue
		}
		for _, f := range r.Findings {
			fmt.Fprintf(&b, "- **%s** `%s:%s` — %s\n  %s\n",
				f.Severity, f.File, f.LineRange, f.Summary, f.Reasoning)
		}
		b.WriteString("\n")
	}
	if partial != "" {
		b.WriteString("<details><summary>Synthesizer partial output</summary>\n\n```\n")
		b.WriteString(partial)
		b.WriteString("\n```\n</details>\n")
	}
	return b.String()
}
