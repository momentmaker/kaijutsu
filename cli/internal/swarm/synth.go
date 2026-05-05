package swarm

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

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
// Synthesize.
func Synthesize(ctx context.Context, results []AgentResult, synth Agent, preset *Preset, budget float64, perAgentTimeout time.Duration) (*Synthesis, error) {
	clusters := clusterFindings(results)
	table := renderDisagreementTable(results, clusters)

	body, err := json.MarshalIndent(stripRawForPrompt(results), "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal findings for synthesizer: %w", err)
	}
	prompt := fmt.Sprintf(preset.Synthesizer, string(body))

	subCtx, cancel := context.WithTimeout(ctx, perAgentTimeout)
	defer cancel()
	start := time.Now()
	raw, err := synth.Run(subCtx, prompt, budget)
	dur := time.Since(start)
	if err != nil {
		return &Synthesis{
			Markdown:    fallbackMarkdown(results, table, raw),
			RawDraft:    raw,
			MergedTable: table,
			Duration:    dur,
			Clusters:    clusters,
		}, fmt.Errorf("synthesizer (%s) failed: %w", synth.Name(), err)
	}
	finalMD := assembleMarkdown(results, table, raw, clusters)
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

// clusterFindings groups findings across agents that point at the
// same (file, line_range). Stage 2 uses exact-match clustering;
// Stage 5 may add fuzzy line-proximity matching.
func clusterFindings(results []AgentResult) []FindingGroup {
	totalAgents := 0
	for _, r := range results {
		if r.Err == "" || len(r.Findings) > 0 {
			totalAgents++
		}
	}
	clusters := map[string]*FindingGroup{}
	for _, r := range results {
		for _, f := range r.Findings {
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
	sort.Slice(out, func(i, j int) bool {
		ri, rj := severityRank(out[i].Severity), severityRank(out[j].Severity)
		if ri != rj {
			return ri > rj // higher severity first
		}
		if out[i].ConsensusOf != out[j].ConsensusOf {
			return out[i].ConsensusOf > out[j].ConsensusOf
		}
		return out[i].Key < out[j].Key
	})
	return out
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
func renderDisagreementTable(results []AgentResult, clusters []FindingGroup) string {
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

	var b strings.Builder
	fmt.Fprintf(&b, "| Finding | Severity | Consensus |")
	for _, c := range cols {
		fmt.Fprintf(&b, " %s |", c)
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
				fmt.Fprintf(&b, " ✓ (%s) |", f.Severity)
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
// disagreement table + run metadata header + footer marker. Stage 4
// will swap the marker into the format `comment.go` expects for
// edit-in-place re-runs.
func assembleMarkdown(results []AgentResult, table, draft string, clusters []FindingGroup) string {
	var b strings.Builder

	b.WriteString("## kaijutsu pr-review\n\n")
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

func fallbackMarkdown(results []AgentResult, table, partial string) string {
	var b strings.Builder
	b.WriteString("## kaijutsu pr-review (synthesis failed; raw findings below)\n\n")
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
