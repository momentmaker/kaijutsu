// pipeline.go — v0.11.0 — extracted from cli/swarm.go::runSwarmPipeline.
// Provides a non-cobra entry point so non-cli callers (eval, autopilot)
// can drive a full swarm pipeline (legacy + persona dispatch, debate,
// synthesis, lie-to-them filter, marker, cache, dream-archive).
//
// Helpers that depend on packages outside swarm's import graph
// (findings recorder, dream archive) are injected via callback fields
// on PipelineOpts so we don't pull cli-pkg dependencies into swarm.
package swarm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/momentmaker/kaijutsu/cli/internal/agents"
)

// PipelineOpts is the structured-args input for RunPipeline.
// Mirrors the commonSwarmFlags shape from cli pkg, plus io.Writers
// (replacing cobra's OutOrStdout / ErrOrStderr / InOrStdin) and a
// few callback fields for cli-pkg-only helpers (findings recorder,
// dream archive).
type PipelineOpts struct {
	ProjectRoot string
	Preset      *Preset
	Input       *InputContext

	// Dispatch shape
	Personas    []string // when non-empty, agents.yaml-driven dispatch
	Synthesizer string   // preferred synthesizer agent / persona name

	// Mode + flags
	Mode            string  // "quick" | "full"
	Strict          bool    // lie-to-them synthesis filter
	PerAgentBudget  float64 // per-call budget passed to agent driver
	Timeout         time.Duration
	MaxCostUSD      float64
	Format          string  // "markdown" | "json"
	PostComment     bool    // post/update PR comment via gh
	Yes             bool    // non-interactive consent
	AllowSecrets    bool    // bypass secrets pre-flight
	NoTelemWarn     bool    // suppress cli-compat telemetry warning
	ShowWeights     bool    // v0.7 weights in disagreement table
	ConfidenceFloor float64 // v0.9 confidence floor for findings clustering
	DreamLenses     []string

	// Estimate-only path (dry-run cost projection)
	Estimate bool

	// I/O
	Stdout io.Writer
	Stderr io.Writer
	Stdin  io.Reader

	// Callbacks for behaviors that can't live in swarm pkg without
	// import cycles. Caller-provided. Empty/nil = skip the behavior.
	//
	// EstimateFn prints the cost-projection table to opts.Stdout when
	// Estimate is true. The cli wrapper supplies a function that
	// renders the table; eval / autopilot can supply a simpler one
	// that returns a structured cost projection.
	EstimateFn func(opts PipelineOpts) error
	// RecordFindings persists the findings into ~/.kaijutsu/findings.db.
	// Best-effort; nil = skip.
	RecordFindings func(stderr io.Writer, projectRoot, runID, preset string, results []AgentResult, personas []*PersonaAdapter)
	// ResolveSynthWeights returns the per-agent weights map for the
	// synthesizer. nil = no weight overlay (v0.6 behavior).
	ResolveSynthWeights func(stderr io.Writer, projectRoot, preset string, results []AgentResult, personas []*PersonaAdapter) map[string]float64
	// ResolveDreamLensWeights returns the per-lens weight map for the
	// dream synthesizer. nil = no overlay.
	ResolveDreamLensWeights func(stderr io.Writer, projectRoot, preset string, results []AgentResult, personas []*PersonaAdapter) map[string]float64
	// ArchiveDreamSession writes a dream-graveyard archive entry for
	// dream preset runs. nil or non-dream preset = skip. Callback is
	// responsible for computing the codebase fingerprint from cwd
	// (swarm pkg can't import findings due to import cycle: findings
	// → swarm).
	ArchiveDreamSession func(topic, cwd, body string, lensOrder []string, mode string, full bool) (string, error)
}

// Result captures what the pipeline returned. The cli wrapper turns
// this into stdout output; eval / autopilot consume the fields directly.
type Result struct {
	Markdown      string
	Run           SwarmRun
	Synthesis     *Synthesis
	AgentResults  []AgentResult
	RunID         string
	EstimatedCost float64
	ActualCost    float64
}

// RunPipeline runs the full swarm pipeline: dispatch (legacy or
// persona), optional debate (full mode), synthesis, optional strict
// lie-to-them filter, marker append, cache write, optional PR comment.
//
// This is the non-cobra extraction of cli/swarm.go::runSwarmPipeline.
// Behavior preserved exactly — same order of operations, same stderr
// messages, same defaults. The cli wrapper feeds a PipelineOpts built
// from cobra flags + I/O writers + cli-pkg callbacks.
func RunPipeline(ctx context.Context, opts PipelineOpts) (*Result, error) {
	out := opts.Stdout
	stderr := opts.Stderr
	if out == nil {
		out = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}
	stdin := opts.Stdin
	if stdin == nil {
		stdin = strings.NewReader("")
	}

	preset := opts.Preset
	ictx := opts.Input

	// Estimate dry-run path: defer to caller's renderer + return early.
	if opts.Estimate {
		if opts.EstimateFn == nil {
			return nil, errors.New("RunPipeline: --estimate requested but EstimateFn callback is nil")
		}
		if err := opts.EstimateFn(opts); err != nil {
			return nil, err
		}
		return &Result{}, nil
	}

	// Telemetry-warning sink: route the cli-compat one-shot warning
	// through caller's stderr so tests can capture it; suppress when
	// NoTelemWarn is set.
	if opts.NoTelemWarn {
		agents.SetCompatWarningSink(io.Discard)
	} else {
		agents.SetCompatWarningSink(stderr)
	}

	// Privacy gate: hard-block on secrets unless explicitly overridden.
	if ictx.InputKind == InputDiff || ictx.InputKind == InputFiles {
		hits := SecretsScan(ictx.Body)
		if len(hits) > 0 && !opts.AllowSecrets {
			fmt.Fprintf(stderr, "secrets pre-flight scan blocked %d match(es):\n", len(hits))
			for _, h := range hits {
				fmt.Fprintf(stderr, "  - %s (%s)\n", h.Match, h.Reason)
			}
			return nil, errors.New("refusing to send input to remote models; remove the secrets or pass --allow-secrets at your own risk")
		}
		if len(hits) > 0 {
			fmt.Fprintf(stderr, "warning: --allow-secrets bypassed %d secrets-scan hit(s); input WILL be sent to model providers\n", len(hits))
		}
	}

	// Persona-driven dispatch (v0.6 Stage 3b): when Personas is set,
	// jobs are assembled from agents.yaml-resolved personas instead of
	// auto-detected native CLIs. Legacy v0.5 path runs unchanged when
	// the slice is empty.
	var (
		jobs            []Job
		personaAdapters []*PersonaAdapter
	)
	if len(opts.Personas) > 0 {
		var err error
		jobs, personaAdapters, err = AssemblePersonaJobs(opts.ProjectRoot, preset, ictx, opts.Personas)
		if err != nil {
			return nil, err
		}
		consentNames := make([]AgentName, 0, len(personaAdapters))
		for _, p := range personaAdapters {
			consentNames = append(consentNames, p.Name())
		}
		if cerr := EnsureConsent(opts.ProjectRoot, preset, stdin, stderr, consentNames, opts.Yes); cerr != nil {
			return nil, cerr
		}
		// Spec D6: --personas mode invalidates the legacy v0.5 cache
		// key. Mix persona names into the key so two different persona
		// mixes against the same input don't collide.
		ictx.CacheKey = MixCacheKeyWithPersonas(ictx.CacheKey, opts.Personas)
		fmt.Fprintf(stderr, "swarm: %d persona(s) dispatching: %v\n", len(personaAdapters), opts.Personas)
	} else {
		available := AvailableAgents()
		if len(available) == 0 {
			return nil, errors.New("no agent CLI available. Install at least one of: claude, codex, agy (Antigravity), then re-run (or pass --personas to dispatch via agents.yaml)")
		}
		if cerr := EnsureConsent(opts.ProjectRoot, preset, stdin, stderr, available, opts.Yes); cerr != nil {
			return nil, cerr
		}
		fmt.Fprintf(stderr, "swarm: %d agent(s) available: %v\n", len(available), available)
		jobs = make([]Job, 0, len(available))
		for _, name := range available {
			tmpl, ok := preset.PerAgent[name]
			if !ok {
				fmt.Fprintf(stderr, "swarm: no preset prompt for %s; skipping\n", name)
				continue
			}
			jobs = append(jobs, Job{
				Agent:  AgentFor(name),
				Prompt: fmt.Sprintf(tmpl, ictx.Body),
			})
		}
		if len(jobs) == 0 {
			return nil, fmt.Errorf("no jobs assembled — preset %q is missing prompts for every available agent", preset.Name)
		}
	}

	start := time.Now()
	results := FanOut(ctx, jobs, opts.PerAgentBudget, opts.Timeout)
	finished := time.Now()

	// Persona mode: replace estimated costs with the real billed cost
	// reported by drivers; stamp driver kind for synthesizer.
	if len(personaAdapters) > 0 {
		OverlayPersonaCosts(results, personaAdapters)
		StampDriverKind(results, personaAdapters)
	}

	run := SwarmRun{
		Preset:     preset.Name,
		PR:         ictx.PR,
		SHA:        ictx.SHA,
		Mode:       opts.Mode,
		Agents:     results,
		StartedAt:  start,
		FinishedAt: finished,
	}
	for _, r := range results {
		run.TotalCost += r.Cost
	}

	// Hard-fail when EVERY agent errored.
	usable := 0
	for _, r := range results {
		if r.Err == "" {
			usable++
		}
	}
	if usable == 0 {
		var b strings.Builder
		fmt.Fprintf(&b, "all %d agent(s) errored; no synthesis performed:", len(results))
		for _, r := range results {
			fmt.Fprintf(&b, "\n  - %s: %s", r.Agent, r.Err)
		}
		ReportSwarmStderr(stderr, results, finished.Sub(start), run.TotalCost)
		return nil, errors.New(b.String())
	}

	// JSON-only path: skip synthesis entirely.
	if opts.Format == "json" {
		if opts.MaxCostUSD > 0 && run.TotalCost > opts.MaxCostUSD {
			fmt.Fprintf(stderr, "warning: estimated total cost $%.2f exceeded --max-cost $%.2f\n", run.TotalCost, opts.MaxCostUSD)
		}
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		if err := enc.Encode(run); err != nil {
			return nil, err
		}
		ReportSwarmStderr(stderr, results, finished.Sub(start), run.TotalCost)
		return &Result{Run: run, AgentResults: results, RunID: ictx.CacheKey, ActualCost: run.TotalCost}, nil
	}

	// Markdown synthesis path. In persona mode the synthesizer is
	// picked from the persona list; legacy mode uses native CLI lookup.
	var synthAgent Agent
	if len(personaAdapters) > 0 {
		synthAgent = PickPersonaSynthesizer(opts.Synthesizer, results, personaAdapters)
	} else {
		synthAgent = pickSynthesizerLegacy(opts.Synthesizer, results)
	}
	if synthAgent == nil {
		fmt.Fprintln(stderr, "warning: no synthesizer agent available; falling back to JSON dump")
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		if err := enc.Encode(run); err != nil {
			return nil, err
		}
		ReportSwarmStderr(stderr, results, finished.Sub(start), run.TotalCost)
		return &Result{Run: run, AgentResults: results, RunID: ictx.CacheKey, ActualCost: run.TotalCost}, nil
	}

	overBudget := opts.MaxCostUSD > 0 && run.TotalCost > opts.MaxCostUSD
	if overBudget {
		fmt.Fprintf(stderr, "warning: Pass-1 cost $%.2f already exceeds --max-cost $%.2f; skipping optional debate/strict passes\n", run.TotalCost, opts.MaxCostUSD)
	}

	// v0.7 quality fingerprinting: compute per-agent weights from the
	// findings DB BEFORE synthesis. Best-effort — when the DB is
	// absent, weights collapses to nil and Synthesize behaves
	// byte-identical to v0.6.2 (cold-start contract).
	var synthWeights, lensWeights map[string]float64
	if opts.ResolveSynthWeights != nil {
		synthWeights = opts.ResolveSynthWeights(stderr, opts.ProjectRoot, preset.Name, results, personaAdapters)
	}
	if opts.ResolveDreamLensWeights != nil {
		lensWeights = opts.ResolveDreamLensWeights(stderr, opts.ProjectRoot, preset.Name, results, personaAdapters)
	}
	synthOpts := SynthOpts{
		Weights:         synthWeights,
		ShowWeights:     opts.ShowWeights,
		LensWeights:     lensWeights,
		ConfidenceFloor: opts.ConfidenceFloor,
	}

	var (
		synth    *Synthesis
		synthErr error
	)
	resultsToRecord := results
	if opts.Mode == "full" && !overBudget {
		fmt.Fprintln(stderr, "swarm: --full mode — Pass 2 round-robin debate starting")
		pass2 := Debate(ctx, results, preset, opts.PerAgentBudget, opts.Timeout)
		for _, r := range pass2 {
			run.TotalCost += r.Cost
		}
		resultsToRecord = MergePasses(results, pass2)
		synth, synthErr = SynthesizeWithDebate(ctx, results, pass2, synthAgent, preset, opts.PerAgentBudget, opts.Timeout, synthOpts)
	} else {
		synth, synthErr = Synthesize(ctx, results, synthAgent, preset, opts.PerAgentBudget, opts.Timeout, synthOpts)
	}

	// v0.7 quality fingerprinting hook (post-debate). Best-effort.
	if opts.RecordFindings != nil {
		opts.RecordFindings(stderr, opts.ProjectRoot, ictx.CacheKey, preset.Name, resultsToRecord, personaAdapters)
	}
	if synth != nil {
		run.TotalCost += synth.Cost
	}
	if synthErr != nil {
		fmt.Fprintf(stderr, "warning: synthesis: %v (using deterministic fallback markdown)\n", synthErr)
	}
	md := ""
	if synth != nil {
		md = synth.Markdown
	}
	if opts.Strict && md != "" && !overBudget {
		filtered, lieCost, lieErr := LieToThem(ctx, md, synthAgent, opts.PerAgentBudget, opts.Timeout)
		if lieErr != nil {
			fmt.Fprintf(stderr, "warning: --strict lie-to-them filter failed: %v (keeping unfiltered draft)\n", lieErr)
		} else {
			md = filtered
			run.TotalCost += lieCost
		}
	}
	if opts.MaxCostUSD > 0 && run.TotalCost > opts.MaxCostUSD {
		fmt.Fprintf(stderr, "warning: total cost $%.2f exceeded --max-cost $%.2f\n", run.TotalCost, opts.MaxCostUSD)
	}
	md = appendMarkerInternal(md, ictx.CacheKey)
	if cerr := CacheRun(opts.ProjectRoot, preset, ictx.CacheKey, results, md); cerr != nil {
		fmt.Fprintf(stderr, "warning: cache write failed: %v\n", cerr)
	}

	// v0.8.3 dream graveyard auto-write. Only fires for the dream
	// preset; other presets skip silently. Best-effort — failure
	// stays in stderr, doesn't block the markdown render.
	if preset.Name == "dream" && opts.ArchiveDreamSession != nil {
		cwd := opts.ProjectRoot
		if cwd == "" {
			if wd, werr := os.Getwd(); werr == nil {
				cwd = wd
			}
		}
		// Use the actual selected lens list when populated. Falls back
		// to base-4 for non-dream-cmd code paths.
		lensOrder := opts.DreamLenses
		if len(lensOrder) == 0 {
			lensOrder = DreamLensesBase()
		}
		// Callback resolves the codebase fingerprint internally.
		// Best-effort — failures don't block.
		path, gerr := opts.ArchiveDreamSession(ictx.Body, cwd, md, lensOrder, "swarm", opts.Mode == "full")
		if gerr != nil {
			fmt.Fprintf(stderr, "warning: dream graveyard write failed: %v\n", gerr)
		} else {
			fmt.Fprintf(stderr, "dream session archived → %s\n", path)
		}
	}

	fmt.Fprint(out, md)
	if opts.PostComment {
		if ictx.PR == 0 {
			fmt.Fprintln(stderr, "warning: --post-comment requested but no PR detected; skipping post")
		} else if perr := PostOrUpdateComment(ctx, ictx.PR, md); perr != nil {
			fmt.Fprintf(stderr, "warning: post comment failed: %v\n", perr)
		} else {
			fmt.Fprintf(stderr, "posted/updated PR comment on #%d\n", ictx.PR)
		}
	}
	ReportSwarmStderr(stderr, results, finished.Sub(start), run.TotalCost)

	return &Result{
		Markdown:     md,
		Run:          run,
		Synthesis:    synth,
		AgentResults: results,
		RunID:        ictx.CacheKey,
		ActualCost:   run.TotalCost,
	}, nil
}

// pickSynthesizerLegacy chooses which agent runs the synthesis pass
// in legacy (non-persona) mode.
func pickSynthesizerLegacy(want string, results []AgentResult) Agent {
	if want != "" && Available(AgentName(want)) {
		return AgentFor(AgentName(want))
	}
	for _, r := range results {
		if r.Err == "" && Available(AgentName(r.Agent)) {
			return AgentFor(AgentName(r.Agent))
		}
	}
	return nil
}

// ReportSwarmStderr writes the per-agent summary to stderr after a
// pipeline run. Separate function so the cli wrapper + future callers
// can invoke it consistently.
func ReportSwarmStderr(stderr io.Writer, results []AgentResult, dur time.Duration, totalCost float64) {
	fmt.Fprintf(stderr, "\nswarm done in %s · est cost $%.2f\n",
		dur.Round(time.Millisecond), totalCost)
	for _, r := range results {
		if r.Err != "" {
			fmt.Fprintf(stderr, "  %s: ERROR %s\n", r.Agent, r.Err)
		} else {
			fmt.Fprintf(stderr, "  %s: %d finding(s) · %s\n", r.Agent, len(r.Findings), r.Duration.Round(time.Millisecond))
		}
	}
}

// appendMarkerInternal tacks the kaijutsu-pr-review HTML comment
// marker onto the synthesis markdown. Internal name avoids collision
// with the cli pkg's appendMarker (which the wrapper still calls
// via this path).
func appendMarkerInternal(md, key string) string {
	return strings.TrimRight(md, "\n") + "\n\n" + Marker(NewRunID(), key) + "\n"
}

