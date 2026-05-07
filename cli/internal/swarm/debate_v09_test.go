package swarm

import (
	"fmt"
	"strings"
	"testing"
)

// TestDreamDebateTemplate_PreservesLensPrefix pins the v0.9 contract:
// the template instructs agents to keep the [lens:<name>] prefix on
// every Pass-2 output finding so MergePasses + the recorder
// validator continue to accept the rows.
func TestDreamDebateTemplate_PreservesLensPrefix(t *testing.T) {
	got := dreamPreset.Debate
	for _, mustContain := range []string{
		"[lens:<name>] [new]",
		"[lens:<name>] [disputes]",
		"[lens:<name>] [revised]",
		"[lens:<name>] [agreed]",
		"preserve",
	} {
		if !strings.Contains(got, mustContain) {
			t.Errorf("dreamPreset.Debate missing required token %q", mustContain)
		}
	}
}

// TestDreamDebateTemplate_RevisionTagsRecognized lists every revision
// tag the synthesizer's MergePasses + dreamSynthesizer downstream
// expect to see in Pass-2 output. If the template ever drops one,
// this test fires.
func TestDreamDebateTemplate_RevisionTagsRecognized(t *testing.T) {
	want := []string{"[new]", "[disputes]", "[revised]", "[agreed]"}
	for _, tag := range want {
		if !strings.Contains(dreamPreset.Debate, tag) {
			t.Errorf("dreamPreset.Debate missing revision tag %q", tag)
		}
	}
}

// TestDreamDebateTemplate_AgreedRequiresTwoPeers covers the spec's
// single-peer caveat: the prompt MUST tell the model that [agreed]
// is unreachable with peer count = 1 (per-skill routing collapse to
// one provider). Without this, the model could emit [agreed]
// against itself in single-peer fixtures.
func TestDreamDebateTemplate_AgreedRequiresTwoPeers(t *testing.T) {
	if !strings.Contains(dreamPreset.Debate, "peer count >= 2") {
		t.Errorf("dreamPreset.Debate should mention 'peer count >= 2' for the [agreed] tag")
	}
}

// TestDreamDebateTemplate_FmtFmtSafe ensures the template uses
// EXACTLY two %s placeholders (matching swarm.Debate's fmt.Sprintf
// signature) and contains no other format directives that would
// surface as %!s(MISSING) at runtime. Regression guard for the v0.8
// dreamLensWild incident.
func TestDreamDebateTemplate_FmtFmtSafe(t *testing.T) {
	got := fmt.Sprintf(dreamPreset.Debate, "ORIGINAL_PLACEHOLDER", "PEERS_PLACEHOLDER")
	for _, banned := range []string{"%!s(", "%!d(", "%!v("} {
		if strings.Contains(got, banned) {
			t.Errorf("rendered debate template contains %q (escape stray %% chars); rendered:\n%s", banned, got)
		}
	}
	if !strings.Contains(got, "ORIGINAL_PLACEHOLDER") {
		t.Errorf("first format-verb slot should accept original-findings JSON")
	}
	if !strings.Contains(got, "PEERS_PLACEHOLDER") {
		t.Errorf("second format-verb slot should accept peers'-findings JSON")
	}
}

// TestDreamDebateTemplate_LoadBearingPrefixRequired locks in the
// reasoning-prefix invariant: Pass-2 findings must start their
// reasoning with "load_bearing: true|false" same as Pass-1, so the
// recorder validator continues to accept them post-merge.
func TestDreamDebateTemplate_LoadBearingPrefixRequired(t *testing.T) {
	if !strings.Contains(dreamPreset.Debate, "load_bearing:") {
		t.Errorf("dreamPreset.Debate must instruct agents to keep the load_bearing: reasoning prefix")
	}
}
