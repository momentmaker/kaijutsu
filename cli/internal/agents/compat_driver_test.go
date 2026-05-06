package agents

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
)

// resetCompatWarningForTest resets the per-process sync.Once so each
// test exercising the warning sink starts clean. Test-only helper —
// placement here (vs. compat_driver.go) keeps the production package
// free of test scaffolding.
func resetCompatWarningForTest() {
	compatWarningOnce = sync.Once{}
}

func TestMergeCompatEnv_PrecedenceOrder(t *testing.T) {
	t.Setenv("DEEPSEEK_API_KEY", "sk-deep")
	p := &Provider{
		Name:    "deepseek-via-claude",
		Driver:  DriverCLICompat,
		BaseCLI: "claude",
		Env: map[string]string{
			"ANTHROPIC_BASE_URL": "https://api.deepseek.com",
			"DISABLE_TELEMETRY":  "force-overridden", // should beat the default
		},
		EnvKey: map[string]string{
			"ANTHROPIC_API_KEY": "DEEPSEEK_API_KEY",
		},
	}
	caller := map[string]string{"FROM_CALLER": "yes"}
	got := mergeCompatEnv(p, caller)

	// Provider.Env literal value beats default telemetry kill.
	if got["DISABLE_TELEMETRY"] != "force-overridden" {
		t.Errorf("DISABLE_TELEMETRY = %q, want provider.Env override", got["DISABLE_TELEMETRY"])
	}
	// Default telemetry kill applied for the other two.
	if got["DISABLE_ERROR_REPORTING"] != "1" {
		t.Errorf("DISABLE_ERROR_REPORTING = %q, want 1", got["DISABLE_ERROR_REPORTING"])
	}
	if got["DISABLE_NON_ESSENTIAL_MODEL_CALLS"] != "1" {
		t.Errorf("DISABLE_NON_ESSENTIAL_MODEL_CALLS = %q, want 1", got["DISABLE_NON_ESSENTIAL_MODEL_CALLS"])
	}
	// EnvKey indirection resolved.
	if got["ANTHROPIC_API_KEY"] != "sk-deep" {
		t.Errorf("ANTHROPIC_API_KEY = %q, want sk-deep (from DEEPSEEK_API_KEY)", got["ANTHROPIC_API_KEY"])
	}
	// Literal env survives.
	if got["ANTHROPIC_BASE_URL"] != "https://api.deepseek.com" {
		t.Errorf("ANTHROPIC_BASE_URL = %q", got["ANTHROPIC_BASE_URL"])
	}
	// Caller env survives (lowest precedence but non-conflicting).
	if got["FROM_CALLER"] != "yes" {
		t.Errorf("FROM_CALLER = %q, want yes", got["FROM_CALLER"])
	}
}

func TestMergeCompatEnv_EmptyEnvKeyValueSkipped(t *testing.T) {
	t.Setenv("DEFINITELY_UNSET_FOR_TEST", "")
	p := &Provider{
		EnvKey: map[string]string{
			"ANTHROPIC_API_KEY": "DEFINITELY_UNSET_FOR_TEST",
		},
	}
	got := mergeCompatEnv(p, nil)
	if _, ok := got["ANTHROPIC_API_KEY"]; ok {
		t.Errorf("ANTHROPIC_API_KEY should not be in merged env when env-key indirection resolves to empty; got %v", got["ANTHROPIC_API_KEY"])
	}
}

func TestEmitCompatWarning_FiresOncePerProcess(t *testing.T) {
	resetCompatWarningForTest()
	var buf1, buf2 bytes.Buffer
	SetCompatWarningSink(&buf1)
	emitCompatWarningOnce("p1", "claude")
	SetCompatWarningSink(&buf2)
	emitCompatWarningOnce("p2", "claude") // should NOT emit a second time
	if !strings.Contains(buf1.String(), "cli-compat") {
		t.Errorf("first warning missing: %q", buf1.String())
	}
	if buf2.Len() != 0 {
		t.Errorf("second warning fired (sync.Once broken?): %q", buf2.String())
	}
}

func TestBuildDriver_CLICompat_RequiresBaseCLI(t *testing.T) {
	cases := []struct {
		name string
		p    *Provider
		want string
	}{
		{"missing base_cli", &Provider{Name: "x", Driver: DriverCLICompat}, "base_cli is required"},
		{"unknown base_cli", &Provider{Name: "x", Driver: DriverCLICompat, BaseCLI: "opencode"}, "is not a known cli driver"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := BuildDriver(tc.p)
			if err == nil {
				t.Fatal("expected error; got nil")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v; want substring %q", err, tc.want)
			}
		})
	}
}

func TestBuildDriver_CLICompat_SuccessPath(t *testing.T) {
	p := &Provider{
		Name:    "deepseek-via-claude",
		Driver:  DriverCLICompat,
		BaseCLI: "claude",
		Env:     map[string]string{"ANTHROPIC_BASE_URL": "https://api.deepseek.com"},
	}
	d, err := BuildDriver(p)
	if err != nil {
		t.Fatal(err)
	}
	if d.Driver() != DriverCLICompat {
		t.Errorf("Driver() = %q, want %q", d.Driver(), DriverCLICompat)
	}
	if d.Name() != "deepseek-via-claude" {
		t.Errorf("Name() = %q", d.Name())
	}
}

// fakeBase records the InvokeOpts it receives so tests can assert
// the cli-compat driver merged env correctly before delegating.
type fakeBase struct {
	gotOpts InvokeOpts
}

func (f *fakeBase) Name() string       { return "fake-base" }
func (f *fakeBase) Driver() DriverKind { return DriverCLI }
func (f *fakeBase) Invoke(_ context.Context, _ string, opts InvokeOpts) (Result, error) {
	f.gotOpts = opts
	return Result{Raw: "from-base", CostUSD: 0, Driver: DriverCLI, CacheStatus: CacheUnsupported}, nil
}

func TestCLICompatDriver_InvokePassesMergedEnvAndOverridesDriverTag(t *testing.T) {
	resetCompatWarningForTest()
	t.Setenv("DEEPSEEK_API_KEY", "sk-deep-test")

	base := &fakeBase{}
	p := &Provider{
		Name:    "deepseek-via-claude",
		Driver:  DriverCLICompat,
		BaseCLI: "claude",
		Env:     map[string]string{"ANTHROPIC_BASE_URL": "https://api.deepseek.com"},
		EnvKey:  map[string]string{"ANTHROPIC_API_KEY": "DEEPSEEK_API_KEY"},
	}
	d := &cliCompatDriver{provider: p, base: base}

	res, err := d.Invoke(context.Background(), "x", InvokeOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Driver != DriverCLICompat {
		t.Errorf("Result.Driver = %q, want %q (cli-compat must override base's DriverCLI tag)", res.Driver, DriverCLICompat)
	}
	if res.Raw != "from-base" {
		t.Errorf("Raw = %q, want pass-through from base", res.Raw)
	}
	// Base saw the merged env.
	if base.gotOpts.ExtraEnv["ANTHROPIC_BASE_URL"] != "https://api.deepseek.com" {
		t.Errorf("base did not see ANTHROPIC_BASE_URL: %v", base.gotOpts.ExtraEnv)
	}
	if base.gotOpts.ExtraEnv["ANTHROPIC_API_KEY"] != "sk-deep-test" {
		t.Errorf("base did not see resolved ANTHROPIC_API_KEY (env-key indirection): %v", base.gotOpts.ExtraEnv)
	}
	if base.gotOpts.ExtraEnv["DISABLE_TELEMETRY"] != "1" {
		t.Errorf("base did not see telemetry-kill DISABLE_TELEMETRY: %v", base.gotOpts.ExtraEnv)
	}
}

func TestCLICompatDriver_PreservesBaseError(t *testing.T) {
	resetCompatWarningForTest()
	base := &fakeErrBase{}
	d := &cliCompatDriver{
		provider: &Provider{Name: "x", Driver: DriverCLICompat, BaseCLI: "claude"},
		base:     base,
	}
	res, err := d.Invoke(context.Background(), "x", InvokeOpts{})
	if err == nil {
		t.Fatal("expected error pass-through; got nil")
	}
	if res.Driver != DriverCLICompat {
		t.Errorf("Driver tag must be cli-compat even on error path; got %q", res.Driver)
	}
}

type fakeErrBase struct{}

func (fakeErrBase) Name() string       { return "boom" }
func (fakeErrBase) Driver() DriverKind { return DriverCLI }
func (fakeErrBase) Invoke(_ context.Context, _ string, _ InvokeOpts) (Result, error) {
	return Result{Driver: DriverCLI, CacheStatus: CacheUnsupported, Err: "boom"}, errors.New("boom")
}
