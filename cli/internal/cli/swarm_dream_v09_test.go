package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestDreamModeFullCostPrompt_NonTTYRejects covers the spec's
// non-interactive guard: --mode full without --yes against a
// non-TTY stdin (test buffer) hard-fails with the spec's exact
// "requires interactive confirmation or --yes" wording.
func TestDreamModeFullCostPrompt_NonTTYRejects(t *testing.T) {
	root := NewRootCmd()
	// SetIn with a *bytes.Buffer makes InOrStdin non-TTY (no Fd).
	// SetArgs intentionally omits --yes so confirmDreamFullModeCost
	// fires.
	root.SetIn(&bytes.Buffer{})
	out := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(stderr)
	root.SetArgs([]string{"swarm", "dream", "topic X", "--mode", "full"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected non-TTY without --yes to hard-fail")
	}
	if !strings.Contains(err.Error(), "requires interactive confirmation or --yes") {
		t.Errorf("error should match spec wording; got: %v", err)
	}
	if !strings.Contains(err.Error(), "refusing to dispatch a non-trivial cost run silently") {
		t.Errorf("error should explain the rejection rationale; got: %v", err)
	}
}

// TestDreamModeFullCostPrompt_YesBypasses confirms --yes skips the
// confirmation entirely (no estimate prompt printed; no stdin read).
// Downstream may still error (no agents in test env), but the cost
// prompt itself MUST not block.
func TestDreamModeFullCostPrompt_YesBypasses(t *testing.T) {
	root := NewRootCmd()
	root.SetIn(&bytes.Buffer{}) // non-TTY, but --yes overrides
	stderr := &bytes.Buffer{}
	root.SetOut(&bytes.Buffer{})
	root.SetErr(stderr)
	root.SetArgs([]string{"swarm", "dream", "topic X", "--mode", "full", "--yes"})

	_ = root.Execute() // downstream may fail; we only check the prompt
	if strings.Contains(stderr.String(), "Continue? [y/N]") {
		t.Error("--yes should suppress the cost prompt; got prompt in stderr")
	}
	if strings.Contains(stderr.String(), "requires interactive confirmation") {
		t.Error("--yes should bypass non-TTY rejection; got rejection in stderr")
	}
}

// TestEstimateDreamFullCost_OrderOfMagnitude verifies the cost
// estimate is non-zero and roughly the right ballpark. Exact value
// is not contracted; the prompt's purpose is to give the user an
// order-of-magnitude number for the y/N decision.
func TestEstimateDreamFullCost_OrderOfMagnitude(t *testing.T) {
	cases := []struct {
		name   string
		lenses []string
		minUSD float64
		maxUSD float64
	}{
		{"4 lenses (base)", make([]string, 4), 0.10, 1.00},
		{"8 lenses (all)", make([]string, 8), 0.20, 2.00},
		{"empty lens list defaults to 4", nil, 0.10, 1.00},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := estimateDreamFullCost(tc.lenses)
			if got < tc.minUSD || got > tc.maxUSD {
				t.Errorf("estimate=%v outside ballpark [%v, %v] for %s",
					got, tc.minUSD, tc.maxUSD, tc.name)
			}
		})
	}
}
