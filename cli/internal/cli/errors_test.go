package cli

import (
	"errors"
	"fmt"
	"syscall"
	"testing"
)

func TestExitError_WrapsAndExposesCode(t *testing.T) {
	base := errors.New("boom")
	for _, c := range []struct {
		name string
		wrap func(error) error
		code int
	}{
		{"usage", UsageError, ExitUsage},
		{"not-found", NotFoundError, ExitNotFound},
		{"auth", AuthError, ExitAuth},
	} {
		t.Run(c.name, func(t *testing.T) {
			wrapped := c.wrap(base)
			if got := ExitCode(wrapped); got != c.code {
				t.Errorf("ExitCode = %d; want %d", got, c.code)
			}
			if !errors.Is(wrapped, base) {
				t.Errorf("errors.Is should still find the underlying err")
			}
			if wrapped.Error() != "boom" {
				t.Errorf("Error() = %q; want %q", wrapped.Error(), "boom")
			}
		})
	}
}

func TestExitError_NilWrapsToNil(t *testing.T) {
	if got := UsageError(nil); got != nil {
		t.Errorf("UsageError(nil) = %v; want nil", got)
	}
	if got := NotFoundError(nil); got != nil {
		t.Errorf("NotFoundError(nil) = %v; want nil", got)
	}
	if got := AuthError(nil); got != nil {
		t.Errorf("AuthError(nil) = %v; want nil", got)
	}
}

func TestExitCode_NilIsSuccess(t *testing.T) {
	if got := ExitCode(nil); got != ExitSuccess {
		t.Errorf("ExitCode(nil) = %d; want %d", got, ExitSuccess)
	}
}

func TestExitCode_GenericErrorIsOne(t *testing.T) {
	if got := ExitCode(errors.New("generic")); got != ExitGeneric {
		t.Errorf("ExitCode(generic) = %d; want %d", got, ExitGeneric)
	}
}

func TestExitCode_FindsExitErrorThroughWrappedChain(t *testing.T) {
	// Nest an ExitError under fmt.Errorf("...: %w", ...) — errors.As should
	// still surface it. This guarantees that callers who add their own
	// context wrappers don't accidentally bury the typed code.
	inner := UsageError(errors.New("bad flag"))
	outer := fmt.Errorf("running command: %w", inner)
	if got := ExitCode(outer); got != ExitUsage {
		t.Errorf("ExitCode through wrap chain = %d; want %d", got, ExitUsage)
	}
}

// TestExitCode_CobraPatternsMapToUsage pins the v0.15.1 global cobra-error
// pattern matching: caller-misuse errors emitted by cobra (mutex enforce,
// unknown flag, required flag, etc.) get exit 2 even when the RunE didn't
// wrap them. Strings match the exact cobra output format.
func TestExitCode_CobraPatternsMapToUsage(t *testing.T) {
	cases := []string{
		`if any flags in the group [json yaml] are set none of the others can be; [json yaml] were all set`,
		`unknown flag: --foo`,
		`required flag(s) "name" not set`,
		`flag needs an argument: --name`,
		`invalid argument "x" for "--by"`,
		`unknown command "ghost" for "jutsu"`,
	}
	for _, msg := range cases {
		t.Run(msg, func(t *testing.T) {
			if got := ExitCode(errors.New(msg)); got != ExitUsage {
				t.Errorf("ExitCode = %d; want %d for %q", got, ExitUsage, msg)
			}
		})
	}
}

// TestExitCode_NonCobraGenericStaysOne pins that arbitrary non-matching
// errors still hit exit 1 (the long tail). Without this, a regression that
// over-broadens the cobra pattern set would silently re-route real failures.
func TestExitCode_NonCobraGenericStaysOne(t *testing.T) {
	cases := []string{
		"disk full",
		"network timeout",
		"some random failure",
		"context deadline exceeded",
		// Adversarial: strings that contain marker-words but NOT in the
		// anchored-cobra format. These were the false-positive risk flagged
		// by the v0.15.1 final pr-review.
		"got unknown flag from upstream tool",                  // word "unknown flag" without ": "
		"server returned: required flag value is corrupted",    // "required flag" but no parens
		"git output: invalid argument list",                    // "invalid argument" but no quote
		"pkg ran unknown command in shell",                     // "unknown command" but no quote
	}
	for _, msg := range cases {
		t.Run(msg, func(t *testing.T) {
			if got := ExitCode(errors.New(msg)); got != ExitGeneric {
				t.Errorf("ExitCode = %d; want %d for %q", got, ExitGeneric, msg)
			}
		})
	}
}

// TestExitCode_EINVALStaysOne pins the OS-error guard: syscall.EINVAL
// stringifies to "invalid argument", which collides with cobra's parser
// pattern. A file op / net dial / syscall returning EINVAL is a real OS
// failure, NOT caller misuse — must stay at exit 1. Surfaced by the
// v0.15.1 final pr-review (paranoid-security-claude conf 0.8).
func TestExitCode_EINVALStaysOne(t *testing.T) {
	if got := ExitCode(syscall.EINVAL); got != ExitGeneric {
		t.Errorf("ExitCode(EINVAL) = %d; want %d (must not collide with cobra invalid-argument pattern)", got, ExitGeneric)
	}
	wrapped := fmt.Errorf("opening file: %w", syscall.EINVAL)
	if got := ExitCode(wrapped); got != ExitGeneric {
		t.Errorf("ExitCode(wrap-EINVAL) = %d; want %d", got, ExitGeneric)
	}
}
