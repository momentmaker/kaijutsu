package findings

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

// TestWeighter_NilStoreReturnsCold covers the spec's backward-compat
// gate: callers may construct a Weighter wrapping a nil Store; every
// lookup returns ColdWeight (1.0) so synthesis behavior is byte-
// identical to v0.6.2.
func TestWeighter_NilStoreReturnsCold(t *testing.T) {
	w := NewWeighter(nil)
	if got := w.WeightFor("claude", "default-claude", "pr-review", "fp1"); got != ColdWeight {
		t.Errorf("nil-store weight=%v, want %v", got, ColdWeight)
	}
}

// TestWeighter_StateBoundaries exercises the 3-state algorithm at
// each transition point: 0 actioned (cold), 1+ actioned (bootstrap),
// 10+ actioned (mature). 50% precision at 10 actioned exits bootstrap.
func TestWeighter_StateBoundaries(t *testing.T) {
	store := openWeighterTestStore(t)
	defer store.Close()
	w := NewWeighter(store)

	tuple := func() (string, string, string, string) {
		return "claude", "default-claude", "pr-review", "fp1"
	}

	// Boundary 1: 0 actioned → cold (1.0). DB empty, no rows for tuple.
	if got := w.WeightFor(tuple()); got != ColdWeight {
		t.Errorf("0 actioned: weight=%v, want %v", got, ColdWeight)
	}

	// Insert 1 accepted → bootstrap (0.7). Below threshold.
	insertActioned(t, store, "claude", "default-claude", "pr-review", "fp1", 1, 0)
	if got := w.WeightFor(tuple()); got != BootstrapWeight {
		t.Errorf("1 actioned: weight=%v, want %v", got, BootstrapWeight)
	}

	// Insert up to actioned=9 → still bootstrap.
	insertActioned(t, store, "claude", "default-claude", "pr-review", "fp1", 4, 4) // total accepted=5, dismissed=4 → 9
	if got := w.WeightFor(tuple()); got != BootstrapWeight {
		t.Errorf("9 actioned: weight=%v, want %v (still bootstrap)", got, BootstrapWeight)
	}

	// One more dismissed → actioned=10 → mature. Precision = 5/10 = 0.5.
	insertActioned(t, store, "claude", "default-claude", "pr-review", "fp1", 0, 1)
	got := w.WeightFor(tuple())
	if got < 0.49 || got > 0.51 {
		t.Errorf("10 actioned 5/10: weight=%v, want ~0.5 (mature)", got)
	}
}

// TestWeighter_ClampsToFloor verifies the precision floor: 200 dismissals
// in a row do NOT zero out the weight. 0.05 keeps the lone-wolf signal
// alive (one accept can pull the weight back up next run).
func TestWeighter_ClampsToFloor(t *testing.T) {
	store := openWeighterTestStore(t)
	defer store.Close()
	w := NewWeighter(store)

	insertActioned(t, store, "noisy", "noisy-persona", "pr-review", "fp1", 0, 50)
	if got := w.WeightFor("noisy", "noisy-persona", "pr-review", "fp1"); got != PrecisionFloor {
		t.Errorf("all-dismissed: weight=%v, want floor %v", got, PrecisionFloor)
	}
}

// TestWeighter_ClampsToCeiling verifies a perfect-record agent gets
// exactly 1.0 (not >1.0 due to math rounding) — important so the
// `weighted_consensus` sort treats perfect agents the same as cold-
// start agents (no double-bonus).
func TestWeighter_ClampsToCeiling(t *testing.T) {
	store := openWeighterTestStore(t)
	defer store.Close()
	w := NewWeighter(store)

	insertActioned(t, store, "great", "great-persona", "pr-review", "fp1", 50, 0)
	if got := w.WeightFor("great", "great-persona", "pr-review", "fp1"); got != PrecisionCeiling {
		t.Errorf("all-accepted: weight=%v, want ceiling %v", got, PrecisionCeiling)
	}
}

// TestWeighter_SlidingWindow confirms the LIMIT 200 query: only the
// most recent 200 actioned findings count. We insert 250 with skewed
// distribution (first 50 all-dismissed, recent 200 all-accepted) and
// expect ~1.0 because the older 50 fall outside the window.
func TestWeighter_SlidingWindow(t *testing.T) {
	store := openWeighterTestStore(t)
	defer store.Close()
	w := NewWeighter(store)

	// Old: 50 dismissed at action_at = -1h
	insertActionedWithTime(t, store, "drift", "drift-persona", "pr-review", "fp1",
		0, 50, time.Now().Add(-1*time.Hour))
	// Recent: 200 accepted at action_at = now
	insertActionedWithTime(t, store, "drift", "drift-persona", "pr-review", "fp1",
		200, 0, time.Now())

	got := w.WeightFor("drift", "drift-persona", "pr-review", "fp1")
	if got != PrecisionCeiling {
		t.Errorf("sliding window: weight=%v, want %v (older 50 dismissed should fall outside)",
			got, PrecisionCeiling)
	}
}

// TestWeighter_ScopedToTuple verifies cross-tuple queries don't bleed
// into each other. A claude/persona-A row must not influence a
// claude/persona-B weight.
func TestWeighter_ScopedToTuple(t *testing.T) {
	store := openWeighterTestStore(t)
	defer store.Close()
	w := NewWeighter(store)

	insertActioned(t, store, "claude", "persona-A", "pr-review", "fp1", 100, 0)
	if got := w.WeightFor("claude", "persona-B", "pr-review", "fp1"); got != ColdWeight {
		t.Errorf("persona-B leaked from persona-A: weight=%v, want %v", got, ColdWeight)
	}
	if got := w.WeightFor("claude", "persona-A", "doc-review", "fp1"); got != ColdWeight {
		t.Errorf("doc-review leaked from pr-review: weight=%v, want %v", got, ColdWeight)
	}
	if got := w.WeightFor("claude", "persona-A", "pr-review", "fp2"); got != ColdWeight {
		t.Errorf("fp2 leaked from fp1: weight=%v, want %v", got, ColdWeight)
	}
}

// TestWeightsForResults_PersonaProviderFallback covers the legacy
// v0.5 path: empty providerForPersona map → persona name used as
// provider. Mirrors RecordRun's fallback so weights line up with
// what the recorder writes.
func TestWeightsForResults_PersonaProviderFallback(t *testing.T) {
	store := openWeighterTestStore(t)
	defer store.Close()
	w := NewWeighter(store)

	insertActioned(t, store, "claude", "claude", "pr-review", "fp1", 100, 0)
	weights := w.WeightsForResults([]string{"claude"}, nil, "pr-review", "fp1")
	if got, ok := weights["claude"]; !ok || got != PrecisionCeiling {
		t.Errorf("legacy fallback: weights[claude]=%v ok=%v, want %v",
			got, ok, PrecisionCeiling)
	}
}

// TestAnyNonCold short-circuit for the synthesizer prompt: when every
// weight is 1.0, the prompt skips the `weights:` section so behavior
// remains byte-identical to v0.6.2.
func TestAnyNonCold(t *testing.T) {
	if AnyNonCold(map[string]float64{"a": 1.0, "b": 1.0}) {
		t.Error("all cold: AnyNonCold true, want false")
	}
	if !AnyNonCold(map[string]float64{"a": 1.0, "b": 0.7}) {
		t.Error("mixed: AnyNonCold false, want true")
	}
	if AnyNonCold(nil) {
		t.Error("nil: AnyNonCold true, want false")
	}
}

// BenchmarkWeightFor measures lookup latency against a 50K-row DB.
// Spec acceptance: < 5ms per lookup. The composite index
// (codebase_fp, preset, provider, persona, action_at DESC) should let
// the planner walk the index instead of scanning.
func BenchmarkWeightFor(b *testing.B) {
	tmp := b.TempDir()
	store, err := Open(filepath.Join(tmp, "bench.db"))
	if err != nil {
		b.Fatalf("Open: %v", err)
	}
	defer store.Close()

	// Seed 50K rows: 100 personas × 500 findings each, 50% actioned.
	tx, _ := store.db.Begin()
	stmt, _ := tx.Prepare(`INSERT INTO findings(
		run_id, codebase_fp, preset, provider, persona,
		severity, file, line_range, summary,
		user_action, action_at
	) VALUES (?,?,?,?,?,?,?,?,?,?,?)`)
	for p := 0; p < 100; p++ {
		persona := fmt.Sprintf("persona-%d", p)
		for i := 0; i < 500; i++ {
			action := "accepted"
			if i%2 == 0 {
				action = "dismissed"
			}
			_, _ = stmt.Exec(
				"run", "fp1", "pr-review", "claude", persona,
				"issue", "x.go", "1", "summary",
				action, time.Now().Add(-time.Duration(i)*time.Minute),
			)
		}
	}
	_ = stmt.Close()
	_ = tx.Commit()

	w := NewWeighter(store)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = w.WeightFor("claude", "persona-50", "pr-review", "fp1")
	}
}

// --- helpers ---

func openWeighterTestStore(t *testing.T) *Store {
	t.Helper()
	tmp := t.TempDir()
	s, err := Open(filepath.Join(tmp, "weighter.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return s
}

// insertActioned adds the given counts of accepted/dismissed rows for
// the tuple, with action_at = now (used by all the boundary tests
// where ordering doesn't matter).
func insertActioned(t *testing.T, s *Store, provider, persona, preset, fp string, accepted, dismissed int) {
	t.Helper()
	insertActionedWithTime(t, s, provider, persona, preset, fp, accepted, dismissed, time.Now())
}

func insertActionedWithTime(t *testing.T, s *Store, provider, persona, preset, fp string, accepted, dismissed int, at time.Time) {
	t.Helper()
	tx, err := s.db.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	stmt, err := tx.Prepare(`INSERT INTO findings(
		run_id, codebase_fp, preset, provider, persona,
		severity, file, line_range, summary,
		user_action, action_at
	) VALUES (?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	defer stmt.Close()
	for i := 0; i < accepted; i++ {
		if _, err := stmt.Exec("r", fp, preset, provider, persona, "issue", "x.go", "1", "s", "accepted", at); err != nil {
			t.Fatalf("insert accepted: %v", err)
		}
	}
	for i := 0; i < dismissed; i++ {
		if _, err := stmt.Exec("r", fp, preset, provider, persona, "issue", "x.go", "1", "s", "dismissed", at); err != nil {
			t.Fatalf("insert dismissed: %v", err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
}
