package cli

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/momentmaker/kaijutsu/cli/internal/agents"
	"github.com/momentmaker/kaijutsu/cli/internal/swarm"
)

// runEstimate prints a per-persona cost projection table and returns
// without invoking any agent. Reused by every swarm subcommand under
// the --estimate flag. v0.11.0 thin wrapper around
// swarm.BuildEstimateProjections.
//
// Output is deterministic (sorted by persona name) so users can grep
// it in scripts; same input → same output.
func runEstimate(stdout, stderr io.Writer, projectRoot string, preset *swarm.Preset, ictx *swarm.InputContext, f commonSwarmFlags) error {
	projections, err := swarm.BuildEstimateProjections(projectRoot, preset, ictx, swarm.EstimateOpts{
		Personas:   f.personas,
		MaxCostUSD: f.maxCostUSD,
	})
	if err != nil {
		return err
	}
	if len(projections) == 0 {
		return fmt.Errorf("--estimate: no providers resolved (legacy mode requires at least one of claude/codex/gemini installed; personas mode requires --personas with valid names)")
	}

	// Sort for deterministic output.
	sort.Slice(projections, func(i, j int) bool {
		return projections[i].PersonaName < projections[j].PersonaName
	})

	fmt.Fprintf(stdout, "%-30s %-12s %6s %6s  %s\n", "PERSONA", "DRIVER", "IN_TOK", "OUT_TOK", "COST")
	fmt.Fprintln(stdout, strings.Repeat("-", 78))
	var total float64
	missingRateCard := 0
	for _, cp := range projections {
		fmt.Fprintln(stdout, agents.FormatProjection(cp))
		if cp.HasRateCard {
			total += cp.CostUSD
		} else {
			missingRateCard++
		}
	}
	fmt.Fprintln(stdout, strings.Repeat("-", 78))
	fmt.Fprintf(stdout, "%-30s %-12s %6s %6s  $%.4f total\n", "TOTAL", "", "", "", total)

	worstAccuracy := agents.EstimateAccuracyTokenizerPct
	for _, cp := range projections {
		if cp.AccuracyPct > worstAccuracy {
			worstAccuracy = cp.AccuracyPct
		}
	}
	tokenizerLabel := "tiktoken-go"
	if worstAccuracy >= agents.EstimateAccuracyFallbackPct {
		tokenizerLabel = "tiktoken-go for openai-compat / char-count fallback for the rest"
	}
	footer := fmt.Sprintf("\nestimate: %s (±%d%% worst case across rows); output assumed at %d tokens.",
		tokenizerLabel, worstAccuracy, agents.OutputTokensEstimate)
	if missingRateCard > 0 {
		footer += fmt.Sprintf(" %d provider(s) missing rate card → cost shown as 0.", missingRateCard)
	}
	if f.maxCostUSD > 0 && total > f.maxCostUSD {
		fmt.Fprintf(stderr, "\nwarning: projected $%.4f exceeds --max-cost $%.4f%s\n", total, f.maxCostUSD, footer)
	} else {
		fmt.Fprintln(stdout, footer)
	}
	return nil
}
