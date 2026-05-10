// errors.go — v0.15.0 typed exit-code surface.
//
// jutsu ships a documented 4-code public contract:
//
//   0  success
//   2  usage error          (bad flag, missing arg, mutually-exclusive combo)
//   3  not-found            (preset/persona/skill/agent missing from registry)
//   4  auth                 (required env var unset, provider rejected creds)
//   1  anything else        (I/O, network, internal panic recovery, generic)
//
// Codes 5/6/7/8/9 are intentionally NOT shipped — the v0.14 + v0.15 dream
// passes flagged full-set lock-in as contract risk. We add codes only when
// concrete user friction surfaces, never preemptively.
//
// Callers wrap errors with UsageError / NotFoundError / AuthError; the root
// cobra runner unwraps + calls os.Exit with the wrapped code. Plain errors
// keep the cobra default of exit 1.
//
// Spec: docs/specs/2026-05-09-v0.15.0-exit-codes-and-export.md
// Reference: docs/exit-codes.md
package cli

import "errors"

// Stable exit codes — public contract from v0.15.0 onwards. Future minor
// versions may ADD codes, must NEVER renumber or repurpose existing ones.
const (
	ExitSuccess  = 0
	ExitGeneric  = 1
	ExitUsage    = 2
	ExitNotFound = 3
	ExitAuth     = 4
)

// ExitError carries an explicit exit code alongside an underlying error.
// The root cobra runner detects this type via errors.As, prints the
// underlying error like cobra normally would, then exits with the carried
// code. Generic (non-ExitError) errors continue to exit 1.
type ExitError struct {
	Code int
	Err  error
}

func (e *ExitError) Error() string {
	if e == nil || e.Err == nil {
		return ""
	}
	return e.Err.Error()
}

func (e *ExitError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// UsageError wraps err with exit code 2. Use when the caller passed bad
// flag values, mutually-exclusive flag combos, or a malformed arg.
func UsageError(err error) error {
	if err == nil {
		return nil
	}
	return &ExitError{Code: ExitUsage, Err: err}
}

// NotFoundError wraps err with exit code 3. Use when a registry/catalog
// lookup miss is the cause (preset/persona/skill/agent name not registered).
func NotFoundError(err error) error {
	if err == nil {
		return nil
	}
	return &ExitError{Code: ExitNotFound, Err: err}
}

// AuthError wraps err with exit code 4. Use when a required provider
// credential is missing or rejected at preflight.
func AuthError(err error) error {
	if err == nil {
		return nil
	}
	return &ExitError{Code: ExitAuth, Err: err}
}

// ExitCode extracts the exit code carried by err, or 1 for any non-nil
// generic error (cobra default), or 0 for nil. Used by the cmd/jutsu/main
// runner to bridge cobra's error return to os.Exit.
func ExitCode(err error) int {
	if err == nil {
		return ExitSuccess
	}
	var exitErr *ExitError
	if errors.As(err, &exitErr) {
		return exitErr.Code
	}
	return ExitGeneric
}
