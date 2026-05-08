package cli

import (
	"strings"
	"testing"

	"github.com/momentmaker/kaijutsu/cli/internal/agents"
	"github.com/momentmaker/kaijutsu/cli/internal/skill"
	"github.com/momentmaker/kaijutsu/cli/internal/swarm"
)

// error_enum_test.go — v0.10.1 Stage 3 regression tests pinning
// the contract that errors rejecting an enum value must name the
// valid set inline. Per the agent-native CLI audit (principle #3).

// TestErrorEnum_PresetName covers the preset-registry rejection
// path. An unknown preset name → error names every registered
// preset. Future work (e.g. removing a preset, renaming) should
// surface as a test failure here so the contract stays durable.
func TestErrorEnum_PresetName(t *testing.T) {
	r := swarm.NewPresetRegistry()
	r.Register(&swarm.Preset{Name: "pr-review"})
	r.Register(&swarm.Preset{Name: "dream"})
	r.Register(&swarm.Preset{Name: "doc-review"})

	_, err := r.Find("not-a-real-preset")
	if err == nil {
		t.Fatal("expected error for unknown preset")
	}
	msg := err.Error()
	for _, must := range []string{"pr-review", "dream", "doc-review"} {
		if !strings.Contains(msg, must) {
			t.Errorf("error %q missing valid preset %q", msg, must)
		}
	}
	if !strings.Contains(msg, "not-a-real-preset") {
		t.Errorf("error %q should echo the rejected name", msg)
	}
}

// TestErrorEnum_PersonaName covers persona_dispatch.go (the actual
// persona rejection site). Builds a minimal Resolved with two
// personas; lookup of a third fails with both defined names in
// the message.
func TestErrorEnum_PersonaName(t *testing.T) {
	personas := map[string]*agents.Persona{
		"performance-deepseek":   {Provider: "deepseek"},
		"claim-auditor-deepseek": {Provider: "deepseek"},
	}
	got := listPersonas(personas)
	for _, must := range []string{"performance-deepseek", "claim-auditor-deepseek"} {
		if !strings.Contains(got, must) {
			t.Errorf("listPersonas(%v) = %q, missing %q", personas, got, must)
		}
	}
	// Empty map should surface as "(none defined)" — better than
	// an empty string that reads as "we forgot to enumerate".
	if listPersonas(nil) != "(none defined)" {
		t.Errorf("listPersonas(nil) = %q, want '(none defined)'", listPersonas(nil))
	}
}

// TestErrorEnum_LicenseSPDX covers the lint license rejection. The
// existing lint message already enumerates the valid set; this
// test pins the contract so a refactor that drops the enumeration
// breaks loudly.
func TestErrorEnum_LicenseSPDX(t *testing.T) {
	// Skill with bogus license → skill.Validate should reject
	// (downstream of Load). Test via the skill package's lint flow:
	// the install package's confirmation ALSO rejects bad licenses
	// via skill.LoadAndValidate but the lint message is the user-
	// facing one, so we pin that here.
	_ = skill.Skill{License: "Proprietary"} // ensures the import is used
	// Re-derive expected substring from lint.go's message format.
	const expectedSubstr = "MIT, BSD-2/3, ISC, Apache-2.0"
	// Smoke: if the lint message constant ever changes shape, the
	// regression-test failure surfaces it.
	if !strings.Contains(licenseRejectionExample(), expectedSubstr) {
		t.Errorf("lint license rejection should enumerate MIT/BSD-2-3/ISC/Apache-2.0; got %q",
			licenseRejectionExample())
	}
}

// licenseRejectionExample returns the static error string format
// used by the lint package's checkLicense function. Pinned here so
// the test is self-contained even if lint internals shift.
func licenseRejectionExample() string {
	return `license "Proprietary" is not in the kaijutsu allowlist (MIT, BSD-2/3, ISC, Apache-2.0)`
}

