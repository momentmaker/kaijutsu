package agents

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"
)

// cliCompatDriver invokes a native CLI (claude / codex / gemini) with
// override env vars injected — the canonical use case is "route claude
// CLI through DeepSeek by setting ANTHROPIC_BASE_URL". This is opt-in
// per spec D6 because the harness CLI may still leak metadata to its
// home vendor via telemetry endpoints that don't respect the BASE_URL
// override; the driver emits a one-shot warning per process to make
// that risk explicit.
//
// Constructed via BuildDriver from a Provider with Driver=DriverCLICompat.
// BaseCLI names the underlying CLI (claude/codex/gemini); Env declares
// literal env vars; EnvKey declares env-var indirection (header value
// is the NAME of an env var to look up at invoke time).
type cliCompatDriver struct {
	provider *Provider
	base     AgentDriver // claude/codex/gemini singleton — dispatches the actual CLI
}

func (d *cliCompatDriver) Name() string       { return d.provider.Name }
func (d *cliCompatDriver) Driver() DriverKind { return DriverCLICompat }

// Invoke dispatches the underlying CLI driver with merged ExtraEnv:
//   provider.Env (literal)        ⊕
//   provider.EnvKey (indirect)    ⊕
//   defaultTelemetryKill (always) ⊕
//   InvokeOpts.ExtraEnv (caller-supplied; lowest precedence)
//
// Caller (swarm pipeline) supplies the warning sink via
// SetCompatWarningSink — once-per-process the warning fires.
func (d *cliCompatDriver) Invoke(ctx context.Context, prompt string, opts InvokeOpts) (Result, error) {
	emitCompatWarningOnce(d.provider.Name, d.provider.BaseCLI)

	merged := mergeCompatEnv(d.provider, opts.ExtraEnv)
	subOpts := opts
	subOpts.ExtraEnv = merged

	res, err := d.base.Invoke(ctx, prompt, subOpts)
	// Override the Driver tag so synth/UI sees the cli-compat label,
	// not the underlying cli.
	res.Driver = DriverCLICompat
	return res, err
}

// defaultTelemetryKillEnv is the spec D6 default kill list applied to
// every cli-compat invocation. Users can override per-provider via
// global agents.yaml `defaults.cli_compat_telemetry_kill`.
var defaultTelemetryKillEnv = map[string]string{
	"DISABLE_TELEMETRY":              "1",
	"DISABLE_ERROR_REPORTING":        "1",
	"DISABLE_NON_ESSENTIAL_MODEL_CALLS": "1",
}

// mergeCompatEnv builds the final env map for a cli-compat invocation.
// Precedence (highest → lowest):
//   1. provider.Env (literal user-declared env)
//   2. provider.EnvKey (env-var indirection; resolved here)
//   3. defaultTelemetryKillEnv (always applied)
//   4. opts.ExtraEnv (caller-supplied; e.g. test fixtures)
func mergeCompatEnv(p *Provider, callerEnv map[string]string) map[string]string {
	out := make(map[string]string)
	for k, v := range callerEnv {
		out[k] = v
	}
	for k, v := range defaultTelemetryKillEnv {
		out[k] = v
	}
	for k, envName := range p.EnvKey {
		if v := os.Getenv(envName); v != "" {
			out[k] = v
		}
	}
	for k, v := range p.Env {
		out[k] = v
	}
	return out
}

// --- Per-process warning sink ---------------------------------------

var (
	compatWarningOnce sync.Once
	compatWarningSink io.Writer = os.Stderr
	compatWarningMu   sync.Mutex
)

// SetCompatWarningSink lets the cli swarm pipeline route the warning
// to cmd.ErrOrStderr instead of os.Stderr. Idempotent; latest call
// wins.
func SetCompatWarningSink(w io.Writer) {
	compatWarningMu.Lock()
	if w != nil {
		compatWarningSink = w
	}
	compatWarningMu.Unlock()
}

func emitCompatWarningOnce(personaName, baseCLI string) {
	compatWarningOnce.Do(func() {
		compatWarningMu.Lock()
		w := compatWarningSink
		compatWarningMu.Unlock()
		fmt.Fprintf(w, "warn: cli-compat driver routing through %q CLI; metadata may still leak via undocumented telemetry endpoints (provider %q). Pass --no-telemetry-warning to suppress.\n", baseCLI, personaName)
	})
}

// resetCompatWarningForTest is exported via _test files only.
func resetCompatWarningForTest() {
	compatWarningOnce = sync.Once{}
}
