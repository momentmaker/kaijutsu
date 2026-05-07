package findings


// Weight algorithm constants (v0.7 hardcoded; v0.7.x may make
// configurable). See spec docs/specs/2026-05-06-v0.7.0-quality-fingerprinting.md
// "Confidence weighting" section for the rationale on each value.
const (
	// WindowSize: how many recent actioned findings per tuple count
	// toward precision. Window is by action_at — drift is detected by
	// recent USER decisions, not recent dispatch volume.
	WindowSize = 200

	// BootstrapThreshold: actioned-count above which we exit bootstrap
	// state and start computing real precision.
	BootstrapThreshold = 10

	// ColdWeight: the cold-start / DB-absent default. 1.0 = byte-
	// identical v0.6 behavior so users with no findings DB see no
	// change in synthesizer ordering.
	ColdWeight = 1.0

	// BootstrapWeight: applied when 1 ≤ actioned < BootstrapThreshold.
	// "Slightly skeptical without dismissing" — agents with too little
	// data to compute precision get a small downweight so the user can
	// see the synthesizer is starting to learn but isn't yet committed.
	BootstrapWeight = 0.7

	// PrecisionFloor: clamp for mature precision. Never zero, even
	// when an agent has been wrong 200 times in a row, so the
	// "lone wolf" signal stays visible.
	PrecisionFloor = 0.05

	// PrecisionCeiling: clamp top — exactly 1.0. Functions as a
	// degenerate clamp; kept named for symmetry with PrecisionFloor.
	PrecisionCeiling = 1.0
)

// Weighter computes per-tuple confidence weights from the findings
// store. Safe for concurrent use — every method is a single read
// query against the WAL-mode SQLite handle.
type Weighter struct {
	store *Store
}

// NewWeighter wraps a Store. Callers may pass a nil Store to get a
// "cold" weighter that always returns ColdWeight — matches the spec
// requirement that synthesizer behavior is byte-identical to v0.6.2
// when no DB exists.
func NewWeighter(s *Store) *Weighter {
	return &Weighter{store: s}
}

// WeightFor returns the confidence weight for a (provider, persona,
// preset, codebase_fp) tuple. The three-state algorithm:
//
//  1. Cold-start (DB absent OR 0 actioned for tuple): ColdWeight (1.0)
//  2. Bootstrap (1 ≤ actioned < BootstrapThreshold): BootstrapWeight (0.7)
//  3. Mature (actioned ≥ BootstrapThreshold): clamped precision
//
// Errors during the lookup (e.g. closed DB) degrade to ColdWeight
// silently — the synthesizer must never fail because the weighter
// hiccuped. The error path is unobservable to callers, intentionally:
// quality fingerprinting is non-critical, and a partial-data warning
// at synth time would be more confusing than missing weight data.
func (w *Weighter) WeightFor(provider, persona, preset, codebaseFp string) float64 {
	if w == nil || w.store == nil || w.store.db == nil {
		return ColdWeight
	}

	rows, err := w.store.db.Query(`
		SELECT user_action FROM findings
		WHERE codebase_fp = ?
		  AND preset = ?
		  AND provider = ?
		  AND persona = ?
		  AND user_action IS NOT NULL
		ORDER BY action_at DESC
		LIMIT ?
	`, codebaseFp, preset, provider, persona, WindowSize)
	if err != nil {
		return ColdWeight
	}
	defer rows.Close()

	accepted, dismissed := 0, 0
	for rows.Next() {
		var action string
		if err := rows.Scan(&action); err != nil {
			return ColdWeight
		}
		switch action {
		case "accepted":
			accepted++
		case "dismissed":
			dismissed++
		}
	}
	if err := rows.Err(); err != nil {
		return ColdWeight
	}

	actioned := accepted + dismissed
	switch {
	case actioned == 0:
		return ColdWeight
	case actioned < BootstrapThreshold:
		return BootstrapWeight
	}

	precision := float64(accepted) / float64(actioned)
	return clampWeight(precision)
}

// WeightsForResults batches WeightFor calls into a single map keyed by
// persona name. Callers (synthesizer integration) need a {persona →
// weight} map for clusterFindings; this is the convenience wrapper
// that also handles the providerForPersona lookup chain.
//
// providerForPersona may be empty; in that case the persona name is
// used as the provider (mirrors RecordRun's legacy v0.5 fallback).
func (w *Weighter) WeightsForResults(personas []string, providerForPersona map[string]string, preset, codebaseFp string) map[string]float64 {
	out := make(map[string]float64, len(personas))
	for _, p := range personas {
		provider, ok := providerForPersona[p]
		if !ok || provider == "" {
			provider = p
		}
		out[p] = w.WeightFor(provider, p, preset, codebaseFp)
	}
	return out
}

func clampWeight(p float64) float64 {
	if p < PrecisionFloor {
		return PrecisionFloor
	}
	if p > PrecisionCeiling {
		return PrecisionCeiling
	}
	return p
}

// AnyNonCold reports whether the weights map contains at least one
// value that differs from ColdWeight. Used by the synthesizer's
// `weights:` prompt section: when every tuple is in cold-start
// (weight = 1.0), we emit no weights section so behavior stays byte-
// identical to v0.6.2.
func AnyNonCold(weights map[string]float64) bool {
	for _, w := range weights {
		if w != ColdWeight {
			return true
		}
	}
	return false
}

