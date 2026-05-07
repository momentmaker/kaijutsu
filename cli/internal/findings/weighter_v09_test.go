package findings

import (
	"testing"
	"time"
)

// TestWeighter_LensSpecificMaturePath: when the (provider, persona,
// preset, codebase, lens) tuple has >= BootstrapThreshold actioned
// rows, return that window's clamped precision. Tier-1 hit.
func TestWeighter_LensSpecificMaturePath(t *testing.T) {
	store := openWeighterTestStore(t)
	defer store.Close()
	w := NewWeighter(store)

	// 9 accepted + 1 dismissed = 10 actioned in the lens-specific
	// window, all on lens="honest". Precision = 0.9.
	insertActionedWithLens(t, store, "claude", "honest-persona", "dream", "fp1", "honest", 9, 1)

	got := w.WeightForLens("claude", "honest-persona", "dream", "fp1", "honest")
	if got < 0.89 || got > 0.91 {
		t.Errorf("lens-specific mature: weight=%v, want ~0.9", got)
	}
}

// TestWeighter_LensFallsBackToTupleWhenSparse: the lens-specific
// window has only 2 actioned rows (below bootstrap), but the tuple-
// without-lens window has 12 actioned. Return the tuple precision,
// not BootstrapWeight. This is the load-bearing tier-2 fallback.
func TestWeighter_LensFallsBackToTupleWhenSparse(t *testing.T) {
	store := openWeighterTestStore(t)
	defer store.Close()
	w := NewWeighter(store)

	// 2 rows on lens=honest (below threshold).
	insertActionedWithLens(t, store, "claude", "honest-persona", "dream", "fp1", "honest", 2, 0)
	// 10 more rows on lens=fit (different lens, same tuple). Tuple-
	// without-lens window sees 12 total — mature. Precision: all
	// accepted across the 12 → ~1.0 → clamped to PrecisionCeiling.
	insertActionedWithLens(t, store, "claude", "honest-persona", "dream", "fp1", "fit", 10, 0)

	got := w.WeightForLens("claude", "honest-persona", "dream", "fp1", "honest")
	// Tuple precision = 12 accepted / 12 actioned = 1.0.
	if got != PrecisionCeiling {
		t.Errorf("lens sparse, tuple mature: weight=%v, want %v (tuple ceiling)", got, PrecisionCeiling)
	}
}

// TestWeighter_LensColdDefaultWhenBothEmpty: lens-specific window
// empty, tuple-without-lens window also empty → ColdWeight. The
// tier-3 path.
func TestWeighter_LensColdDefaultWhenBothEmpty(t *testing.T) {
	store := openWeighterTestStore(t)
	defer store.Close()
	w := NewWeighter(store)

	got := w.WeightForLens("claude", "honest-persona", "dream", "fp1", "honest")
	if got != ColdWeight {
		t.Errorf("both empty: weight=%v, want %v (cold)", got, ColdWeight)
	}
}

// TestWeighter_LensSparseTupleSparse: lens has 0 actioned, tuple has
// 1-9 actioned (below threshold). Return BootstrapWeight per the
// tier-2/3 boundary in v0.9 spec — tuple precision is unstable below
// threshold so weighter degrades to bootstrap.
func TestWeighter_LensSparseTupleSparse(t *testing.T) {
	store := openWeighterTestStore(t)
	defer store.Close()
	w := NewWeighter(store)

	// Only 3 actioned rows on a different lens; tuple total = 3.
	insertActionedWithLens(t, store, "claude", "p", "dream", "fp1", "fit", 3, 0)

	got := w.WeightForLens("claude", "p", "dream", "fp1", "honest")
	if got != BootstrapWeight {
		t.Errorf("lens empty, tuple bootstrap: weight=%v, want %v", got, BootstrapWeight)
	}
}

// TestWeighter_EmptyLensIdenticalToWeightFor: passing lens="" via
// WeightForLens must produce byte-identical results to WeightFor.
// Regression guard for the v0.7 behavior.
func TestWeighter_EmptyLensIdenticalToWeightFor(t *testing.T) {
	store := openWeighterTestStore(t)
	defer store.Close()
	w := NewWeighter(store)

	insertActioned(t, store, "claude", "p", "pr-review", "fp1", 7, 3)

	classic := w.WeightFor("claude", "p", "pr-review", "fp1")
	v09 := w.WeightForLens("claude", "p", "pr-review", "fp1", "")
	if classic != v09 {
		t.Errorf("WeightFor=%v != WeightForLens(lens=\"\")=%v", classic, v09)
	}
}

// TestWeighter_NilStoreReturnsColdForLens: nil-store path must work
// for the new WeightForLens method too.
func TestWeighter_NilStoreReturnsColdForLens(t *testing.T) {
	w := NewWeighter(nil)
	if got := w.WeightForLens("claude", "p", "dream", "fp1", "honest"); got != ColdWeight {
		t.Errorf("nil-store lens weight=%v, want %v", got, ColdWeight)
	}
}

// --- helpers ---

// insertActionedWithLens is the v0.9 variant of insertActioned that
// populates the lens column. Used by 3-tier fallback tests.
func insertActionedWithLens(t *testing.T, s *Store, provider, persona, preset, fp, lens string, accepted, dismissed int) {
	t.Helper()
	tx, err := s.db.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	stmt, err := tx.Prepare(`INSERT INTO findings(
		run_id, codebase_fp, preset, provider, persona,
		severity, file, line_range, summary, lens,
		user_action, action_at
	) VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	defer stmt.Close()
	now := time.Now()
	for i := 0; i < accepted; i++ {
		if _, err := stmt.Exec("r", fp, preset, provider, persona, "issue", "x.go", "1", "[lens:"+lens+"] s", lens, "accepted", now); err != nil {
			t.Fatalf("insert accepted: %v", err)
		}
	}
	for i := 0; i < dismissed; i++ {
		if _, err := stmt.Exec("r", fp, preset, provider, persona, "issue", "x.go", "1", "[lens:"+lens+"] s", lens, "dismissed", now); err != nil {
			t.Fatalf("insert dismissed: %v", err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
}
