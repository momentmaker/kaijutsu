package cli

import (
	"strings"
	"testing"

	"github.com/momentmaker/kaijutsu/cli/internal/swarm"
)

// TestResolveDreamLenses_Defaults covers the empty + "base" cases —
// both should resolve to the canonical 4 base lenses in order.
func TestResolveDreamLenses_Defaults(t *testing.T) {
	cases := []string{"", "base"}
	for _, in := range cases {
		got, err := resolveDreamLenses(in)
		if err != nil {
			t.Fatalf("resolveDreamLenses(%q): %v", in, err)
		}
		want := swarm.DreamLensesBase()
		if len(got) != len(want) {
			t.Fatalf("len(got)=%d, want %d for input %q", len(got), len(want), in)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("resolveDreamLenses(%q)[%d] = %q, want %q", in, i, got[i], want[i])
			}
		}
	}
}

// TestResolveDreamLenses_All verifies "all" expands to the 8-lens set.
func TestResolveDreamLenses_All(t *testing.T) {
	got, err := resolveDreamLenses("all")
	if err != nil {
		t.Fatalf("resolveDreamLenses(all): %v", err)
	}
	if len(got) != 8 {
		t.Errorf("'all' expanded to %d lenses, want 8", len(got))
	}
}

// TestResolveDreamLenses_CommaList verifies an explicit comma-list
// produces an ordered, deduped subset.
func TestResolveDreamLenses_CommaList(t *testing.T) {
	got, err := resolveDreamLenses("honest,gaps,inverse")
	if err != nil {
		t.Fatalf("resolveDreamLenses: %v", err)
	}
	want := []string{"honest", "gaps", "inverse"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestResolveDreamLenses_DedupesAndStrips covers whitespace tolerance
// + dedup. " honest , gaps , honest " → ["honest", "gaps"].
func TestResolveDreamLenses_DedupesAndStrips(t *testing.T) {
	got, err := resolveDreamLenses(" honest , gaps , honest ")
	if err != nil {
		t.Fatalf("resolveDreamLenses: %v", err)
	}
	want := []string{"honest", "gaps"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d (got %v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestResolveDreamLenses_RejectsUnknownLens verifies a bad name in
// the comma-list errors with a helpful message listing valid names.
func TestResolveDreamLenses_RejectsUnknownLens(t *testing.T) {
	_, err := resolveDreamLenses("honest,premortem,gaps")
	if err == nil {
		t.Fatal("expected error for unknown lens 'premortem'")
	}
	if !strings.Contains(err.Error(), "premortem") {
		t.Errorf("error should name the bad lens; got: %v", err)
	}
	if !strings.Contains(err.Error(), "Valid:") {
		t.Errorf("error should list valid lenses; got: %v", err)
	}
}

// TestResolveDreamLenses_EmptyAfterParseErrors covers the edge case
// where the input is non-empty but parses to an empty list (e.g.
// just commas or whitespace).
func TestResolveDreamLenses_EmptyAfterParseErrors(t *testing.T) {
	for _, bad := range []string{",,,", " , , "} {
		_, err := resolveDreamLenses(bad)
		if err == nil {
			t.Errorf("resolveDreamLenses(%q) expected error, got nil", bad)
		}
	}
}
