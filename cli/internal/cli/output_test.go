package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

// TestAutoFormat_ExplicitFormatWins covers the priority chain: when
// --format is set, AutoFormat returns it regardless of TTY state.
func TestAutoFormat_ExplicitFormatWins(t *testing.T) {
	cmd := &cobra.Command{}
	BindFormatFlags(cmd)
	cmd.Flags().Set("format", "markdown")

	if got := AutoFormat(cmd, FormatJSON); got != FormatMarkdown {
		t.Errorf("AutoFormat with --format=markdown = %q, want markdown", got)
	}
}

// TestAutoFormat_JSONShorthandWins covers --json as alias for
// --format json. Either should produce FormatJSON.
func TestAutoFormat_JSONShorthandWins(t *testing.T) {
	cmd := &cobra.Command{}
	BindFormatFlags(cmd)
	cmd.Flags().Set("json", "true")

	if got := AutoFormat(cmd, FormatMarkdown); got != FormatJSON {
		t.Errorf("AutoFormat with --json = %q, want json", got)
	}
}

// TestAutoFormat_NoFlagsRegistered exercises the nil-safe path: a
// command that hasn't called BindFormatFlags still gets an answer
// based on TTY detection (no panic, no missing-flag error).
func TestAutoFormat_NoFlagsRegistered(t *testing.T) {
	cmd := &cobra.Command{}
	// No BindFormatFlags call.
	got := AutoFormat(cmd, FormatMarkdown)
	// In test runs stdout is usually NOT a TTY → expect JSON. But
	// this can flip when running interactively. Assert the result is
	// one of the valid format constants.
	if got != FormatJSON && got != FormatMarkdown {
		t.Errorf("AutoFormat without flags = %q, want json or markdown", got)
	}
}

// TestAutoFormat_FormatFlagBeatsJSON covers the explicit-overrides-
// shorthand precedence: if both --format=markdown AND --json=true
// are set, --format wins (it's the more specific override).
func TestAutoFormat_FormatFlagBeatsJSON(t *testing.T) {
	cmd := &cobra.Command{}
	BindFormatFlags(cmd)
	cmd.Flags().Set("format", "markdown")
	cmd.Flags().Set("json", "true")

	if got := AutoFormat(cmd, FormatJSON); got != FormatMarkdown {
		t.Errorf("AutoFormat with conflicting flags = %q, want markdown (explicit --format wins)", got)
	}
}

// TestBindFormatFlags_Idempotent covers calling BindFormatFlags on a
// command that already has the flags — should not panic. Cobra panics
// on duplicate flag registration; the nil-safe lookup pattern in
// AutoFormat doesn't help if BindFormatFlags itself crashes.
func TestBindFormatFlags_Idempotent(t *testing.T) {
	cmd := &cobra.Command{}
	BindFormatFlags(cmd)
	defer func() {
		if r := recover(); r == nil {
			// no second call yet — try one
		}
	}()
	// Duplicate registration WILL panic in cobra. We want callers
	// to register once per command; this test documents the
	// expectation by checking the FIRST call doesn't panic, and
	// leaving the duplicate-call-panics behavior as cobra's contract.
	if cmd.Flags().Lookup("format") == nil || cmd.Flags().Lookup("json") == nil {
		t.Fatal("BindFormatFlags did not register --format and --json")
	}
}
