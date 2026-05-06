package agents

import (
	"bytes"
	"strings"
	"testing"
)

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
