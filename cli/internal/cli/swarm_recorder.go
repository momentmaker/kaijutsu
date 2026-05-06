package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/momentmaker/kaijutsu/cli/internal/findings"
	"github.com/momentmaker/kaijutsu/cli/internal/swarm"
)

// recordFindingsBestEffort persists the swarm's findings into the v0.7
// quality-fingerprinting store (~/.kaijutsu/findings.db) without ever
// blocking the markdown render. Failures emit a single yellow stderr
// warning and the pipeline continues — the spec calls fingerprinting
// "non-critical": its value is on the NEXT run, when weighting kicks
// in, not on this one.
//
// The hook fires after FanOut + cost overlay + driver-kind stamping,
// before synthesis. We deliberately record raw fan-out results, not
// the post-synthesis cluster: weight math operates on per-(provider,
// persona) precision, which is per-row not per-cluster.
//
// Cache replay path (LoadCachedResults) is upstream of this hook in
// the future; today every invocation re-runs FanOut, so re-recording
// the same run_id would be a duplicate. v0.7.x will gate this behind
// a "did we replay from cache" check once cache replay is wired into
// runSwarmPipeline.
//
// --full mode caveat: in --full, swarm.Debate runs Pass-2 AFTER this
// hook fires, then mergePasses overlays Pass-2 revisions onto Pass-1.
// We currently record Pass-1 only, so any Pass-2-marked [new] finding
// won't get a DB row + any [disputes]/[agreed] revision text won't
// match what `jutsu finding list` shows vs. what the synthesis prints.
// For weight math (per-(provider, persona) precision), this doesn't
// matter — the agent identity is preserved across passes — but the
// summary text divergence is real. v0.7.x will move the recorder
// below the debate block once we have signal on whether users hit
// this divergence in practice.
func recordFindingsBestEffort(stderr io.Writer, projectRoot, runID, preset string, results []swarm.AgentResult, personas []*personaAdapter) {
	// Cheap pre-check: nothing to record at all? Skip silently — we
	// don't even want to open the DB for a 0-finding fan-out.
	if !anyFindings(results) {
		return
	}

	// runID == "" happens for uncommitted-diff inputs (CacheKey is the
	// SHA, which is "" with no commit yet) and for some prompt-input
	// edge cases. Recording without a run_id would break the spec
	// contract that links findings back to runs/<key>/. Skip with an
	// informational warning rather than synthesizing a fake id.
	if runID == "" {
		warnFindings(stderr, "no run_id (uncommitted diff?); skipping record")
		return
	}

	dbPath, err := findings.DefaultPath()
	if err != nil {
		warnFindings(stderr, "resolve db path: %v", err)
		return
	}
	store, err := findings.Open(dbPath)
	if err != nil {
		warnFindings(stderr, "open %s: %v", dbPath, err)
		return
	}
	defer store.Close()

	cwd := projectRoot
	if cwd == "" {
		// projectRoot may be empty during a non-rooted command (e.g.
		// `jutsu swarm prompt --prompt "..."` outside any repo). Fall
		// back to os.Getwd so fingerprint resolves against the user's
		// real cwd, not against "".
		if wd, werr := os.Getwd(); werr == nil {
			cwd = wd
		}
	}
	fp := findings.Fingerprint(cwd)

	meta := findings.RunMeta{
		RunID:              runID,
		CodebaseFP:         fp,
		Preset:             preset,
		ProviderForPersona: providerMap(personas),
	}
	n, err := findings.RecordRun(store, meta, results)
	if err != nil {
		warnFindings(stderr, "record %d result(s): %v", len(results), err)
		return
	}
	if n > 0 {
		// Single line so it sits cleanly above the existing "swarm:"
		// stderr summary. ANSI-free — caller may pipe to a log file.
		fmt.Fprintf(stderr, "findings: recorded %d row(s) → %s (codebase fp: %s)\n", n, dbPath, fp)
	}
}

func anyFindings(results []swarm.AgentResult) bool {
	for _, r := range results {
		if r.Err == "" && len(r.Findings) > 0 {
			return true
		}
	}
	return false
}

// providerMap builds the {persona-name: provider-name} map RecordRun
// uses to populate the `provider` column. Legacy v0.5 mode passes nil
// adapters; RecordRun then falls back to using the agent name as the
// provider, which is correct for native CLIs (claude/codex/gemini).
func providerMap(personas []*personaAdapter) map[string]string {
	if len(personas) == 0 {
		return nil
	}
	m := make(map[string]string, len(personas))
	for _, p := range personas {
		m[p.personaName] = p.providerName
	}
	return m
}

func warnFindings(stderr io.Writer, format string, args ...any) {
	fmt.Fprintf(stderr, "findings: warning: "+format+" (continuing — quality fingerprinting is non-critical)\n", args...)
}

// resolveSynthWeights computes the per-agent weights map the
// synthesizer's clusterFindings sort + (optional) --show-weights
// table-column annotation consume. Returns nil when:
//   - the findings DB doesn't exist yet (cold-start byte-identical
//     contract: synthesizer behaves like v0.6.2),
//   - no agents produced findings (nothing to weight),
//   - the DB read fails (best-effort — weights are non-critical).
//
// The map keys match AgentResult.Agent (persona name in v0.6+, native
// CLI name in v0.5 legacy mode). swarm.weightFor falls back to 1.0
// for missing keys, so partial maps are safe.
func resolveSynthWeights(stderr io.Writer, projectRoot, preset string, results []swarm.AgentResult, personas []*personaAdapter) map[string]float64 {
	if !anyFindings(results) {
		return nil
	}
	dbPath, err := findings.DefaultPath()
	if err != nil {
		return nil
	}
	if _, err := os.Stat(dbPath); err != nil {
		// DB absent → cold-start. Don't warn — that's the v0.6
		// byte-identical contract, not a degraded path.
		return nil
	}
	store, err := findings.Open(dbPath)
	if err != nil {
		warnFindings(stderr, "open db for weights: %v", err)
		return nil
	}
	defer store.Close()

	cwd := projectRoot
	if cwd == "" {
		if wd, werr := os.Getwd(); werr == nil {
			cwd = wd
		}
	}
	fp := findings.Fingerprint(cwd)

	personaNames := make([]string, 0, len(results))
	for _, r := range results {
		if r.Err == "" {
			personaNames = append(personaNames, r.Agent)
		}
	}
	w := findings.NewWeighter(store)
	return w.WeightsForResults(personaNames, providerMap(personas), preset, fp)
}
