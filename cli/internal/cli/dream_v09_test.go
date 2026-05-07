package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestRotateLensOrder_CanonicalCycle pins the rotation rule across
// all 8 LEAD positions when current set == full canonical set.
// Each prev-LEAD advances to the next cycle entry as the new LEAD.
func TestRotateLensOrder_CanonicalCycle(t *testing.T) {
	full := []string{"honest", "fit", "gaps", "wild", "adversary", "inverse", "status-quo", "time"}
	cases := []struct {
		prevLead string
		wantLead string
	}{
		{"honest", "fit"},
		{"fit", "gaps"},
		{"gaps", "wild"},
		{"wild", "adversary"},
		{"adversary", "inverse"},
		{"inverse", "status-quo"},
		{"status-quo", "time"},
		{"time", "honest"}, // wrap
	}
	for _, tc := range cases {
		got := RotateLensOrder([]string{tc.prevLead}, full)
		if len(got) == 0 || got[0] != tc.wantLead {
			t.Errorf("prev=[%s], current=full → got LEAD %v, want %s", tc.prevLead, got, tc.wantLead)
		}
	}
}

// TestRotateLensOrder_HandlesPartialPriorList covers the rotation
// path when current is a SUBSET of canonical (--lenses base = 4).
// The rotation walks the cycle until it finds a candidate that's
// in current.
func TestRotateLensOrder_HandlesPartialPriorList(t *testing.T) {
	base := []string{"honest", "fit", "gaps", "wild"}
	// prev LEAD = honest → next cycle entry that's in base = fit
	got := RotateLensOrder([]string{"honest"}, base)
	if len(got) == 0 || got[0] != "fit" {
		t.Errorf("base set, prev=honest → got %v, want LEAD=fit", got)
	}
	// prev LEAD = wild → next cycle entry = adversary, NOT in base.
	// Walks: adversary (no), inverse (no), status-quo (no), time
	// (no), honest (yes) → new LEAD = honest.
	got = RotateLensOrder([]string{"wild"}, base)
	if len(got) == 0 || got[0] != "honest" {
		t.Errorf("base set, prev=wild → got %v, want LEAD=honest (wraparound)", got)
	}
	// prev LEAD = adversary (not in current) → walks cycle to next-in-current
	// from adversary's position: inverse (no), status-quo (no), time
	// (no), honest (yes).
	got = RotateLensOrder([]string{"adversary"}, base)
	if len(got) == 0 || got[0] != "honest" {
		t.Errorf("base set, prev=adversary → got %v, want LEAD=honest", got)
	}
}

// TestRotateLensOrder_EmptyAndCorruptionFallback covers degenerate
// inputs: empty prev, empty current, prev[0] not in canonical.
func TestRotateLensOrder_EmptyAndCorruptionFallback(t *testing.T) {
	current := []string{"honest", "fit"}

	if got := RotateLensOrder(nil, current); !reflect.DeepEqual(got, current) {
		t.Errorf("nil prev → want unchanged %v, got %v", current, got)
	}
	if got := RotateLensOrder([]string{}, current); !reflect.DeepEqual(got, current) {
		t.Errorf("empty prev → want unchanged %v, got %v", current, got)
	}
	if got := RotateLensOrder([]string{"unknown-lens"}, current); !reflect.DeepEqual(got, current) {
		t.Errorf("non-canonical prev[0] → want unchanged %v, got %v", current, got)
	}
	if got := RotateLensOrder([]string{"honest"}, nil); got != nil {
		t.Errorf("nil current → want nil, got %v", got)
	}
	if got := RotateLensOrder([]string{"honest"}, []string{}); !reflect.DeepEqual(got, []string{}) {
		t.Errorf("empty current → want []string{}, got %v", got)
	}
}

// TestRotateLensOrder_PreservesRestInCanonicalOrder verifies the new
// order is [newLead, ...rest of current in canonical order]. Insertion
// order in `current` doesn't leak; canonical ordering wins.
//
// Algorithm: result = [newLead] + (canonical order of current minus
// newLead). With prev=honest, current={wild, gaps, fit, honest}:
//   new LEAD = fit (canonical pos 1, in current)
//   rest in canonical order minus fit = honest, gaps, wild
//   result = [fit, honest, gaps, wild]
func TestRotateLensOrder_PreservesRestInCanonicalOrder(t *testing.T) {
	current := []string{"wild", "gaps", "fit", "honest"}
	got := RotateLensOrder([]string{"honest"}, current)
	want := []string{"fit", "honest", "gaps", "wild"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// TestReadDreamLensOrder_HappyPath round-trips: WriteDreamSession
// emits the lens_order line; ReadDreamLensOrder parses it back.
func TestReadDreamLensOrder_HappyPath(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("KAIJUTSU_DREAM_DIR", tmp)

	lensOrder := []string{"honest", "fit", "gaps", "wild"}
	path, err := WriteDreamSession("topic X", "abc123def4567890", "body", lensOrder, "swarm", false)
	if err != nil {
		t.Fatalf("WriteDreamSession: %v", err)
	}
	got, err := ReadDreamLensOrder(path)
	if err != nil {
		t.Fatalf("ReadDreamLensOrder: %v", err)
	}
	if !reflect.DeepEqual(got, lensOrder) {
		t.Errorf("got %v, want %v", got, lensOrder)
	}
}

// TestReadDreamLensOrder_MissingHeader returns nil when the file
// has no lens_order: line (pre-v0.8.3 file or external write).
func TestReadDreamLensOrder_MissingHeader(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "no-header.md")
	if err := os.WriteFile(path, []byte("# no frontmatter here\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := ReadDreamLensOrder(path)
	if err != nil {
		t.Fatalf("ReadDreamLensOrder: %v", err)
	}
	if got != nil {
		t.Errorf("got %v, want nil", got)
	}
}

// TestReadDreamLensOrder_MalformedLine returns nil when the line
// exists but isn't a YAML flow sequence — defensive against future
// header format edits.
func TestReadDreamLensOrder_MalformedLine(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "bad.md")
	body := "lens_order: honest, fit, gaps\n# rest of file\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := ReadDreamLensOrder(path)
	if err != nil {
		t.Fatalf("ReadDreamLensOrder: %v", err)
	}
	if got != nil {
		t.Errorf("malformed line should yield nil; got %v", got)
	}
}

// TestReadDreamLensOrder_QuotedTopicScalarsDontBreakParse verifies the
// parser doesn't get confused by the quoted topic line that lives
// in the same frontmatter block.
func TestReadDreamLensOrder_QuotedTopicScalarsDontBreakParse(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("KAIJUTSU_DREAM_DIR", tmp)
	// Topic with colons exercises the YAML-quoting path in
	// WriteDreamSession.
	path, err := WriteDreamSession("Should we ship: v0.9?", "fp", "body", []string{"honest", "fit"}, "swarm", false)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := ReadDreamLensOrder(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	want := []string{"honest", "fit"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v (quoted topic shouldn't disrupt parser)", got, want)
	}
}

// TestRotation_FiresWithKillswitchOn locks in the v0.9 risk-table
// behavior: rotation is independent of the adaptive-lens-weighting
// killswitch. KAIJUTSU_DREAM_ADAPTIVE_LENS=off disables tier gating
// and reordering, but rotation still applies (the two systems are
// distinct: rotation reads the graveyard, weighting reads the DB).
func TestRotation_FiresWithKillswitchOn(t *testing.T) {
	t.Setenv("KAIJUTSU_DREAM_ADAPTIVE_LENS", "off")
	got := RotateLensOrder([]string{"honest"}, []string{"honest", "fit", "gaps", "wild"})
	if len(got) == 0 || got[0] != "fit" {
		t.Errorf("rotation should fire even with killswitch on; got %v", got)
	}
	// Cleanup hint — t.Setenv restores at test end.
	if !strings.HasPrefix(got[0], "fit") {
		t.Fatalf("guard tripped")
	}
}
