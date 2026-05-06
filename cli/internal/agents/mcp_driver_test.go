package agents

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// stubMcpBinaryPath builds the stub MCP server binary into a temp
// dir at the start of the package's tests and returns its path. The
// binary is rebuilt for each call (cheap; small binary) so individual
// tests can use t.TempDir for isolation.
func stubMcpBinaryPath(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "stub_mcp_server")
	cmd := exec.Command("go", "build", "-o", bin, "./testdata/stub_mcp_server")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("build stub server: %v", err)
	}
	return bin
}

func TestMCPDriver_StdioHappyPath(t *testing.T) {
	bin := stubMcpBinaryPath(t)
	d := &mcpDriver{provider: &Provider{
		Name:      "semgrep-stub",
		Driver:    DriverMCP,
		Transport: "stdio",
		Command:   bin,
		ToolName:  "analyze",
	}}
	res, err := d.Invoke(context.Background(), "review the diff", InvokeOpts{})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if res.Driver != DriverMCP {
		t.Errorf("Result.Driver = %q, want %q", res.Driver, DriverMCP)
	}
	if res.CostUSD != 0 {
		t.Errorf("CostUSD = %v, want 0 (mcp drivers are free)", res.CostUSD)
	}
	if res.CacheStatus != CacheUnsupported {
		t.Errorf("CacheStatus = %q, want %q", res.CacheStatus, CacheUnsupported)
	}
	if !strings.Contains(res.Raw, "missing input validation") {
		t.Errorf("Raw = %q; want canned finding text", res.Raw)
	}
}

func TestMCPDriver_StdioServerErrorSurfaces(t *testing.T) {
	bin := stubMcpBinaryPath(t)
	t.Setenv("STUB_BEHAVIOR", "error")
	d := &mcpDriver{provider: &Provider{
		Name: "broken", Driver: DriverMCP, Transport: "stdio", Command: bin,
	}}
	_, err := d.Invoke(context.Background(), "x", InvokeOpts{})
	if err == nil {
		t.Fatal("expected error from stub server; got nil")
	}
	if !strings.Contains(err.Error(), "stub-error") {
		t.Errorf("error = %v; want substring 'stub-error'", err)
	}
}

func TestMCPDriver_RejectsHTTPTransport(t *testing.T) {
	d := &mcpDriver{provider: &Provider{
		Name: "x", Driver: DriverMCP, Transport: "http", Endpoint: "https://x",
	}}
	_, err := d.Invoke(context.Background(), "x", InvokeOpts{})
	if err == nil {
		t.Fatal("expected error for http transport (not implemented); got nil")
	}
	if !strings.Contains(err.Error(), "http transport not implemented") {
		t.Errorf("error = %v", err)
	}
}

func TestMCPDriver_RejectsUnknownTransport(t *testing.T) {
	d := &mcpDriver{provider: &Provider{
		Name: "x", Driver: DriverMCP, Transport: "carrier-pigeon",
	}}
	_, err := d.Invoke(context.Background(), "x", InvokeOpts{})
	if err == nil {
		t.Fatal("expected error for unknown transport; got nil")
	}
	if !strings.Contains(err.Error(), "unsupported transport") {
		t.Errorf("error = %v", err)
	}
}

func TestMCPDriver_StdioMissingCommandRejects(t *testing.T) {
	d := &mcpDriver{provider: &Provider{
		Name: "x", Driver: DriverMCP, Transport: "stdio", // no Command
	}}
	_, err := d.Invoke(context.Background(), "x", InvokeOpts{})
	if err == nil {
		t.Fatal("expected error for missing command; got nil")
	}
	if !strings.Contains(err.Error(), "command field") {
		t.Errorf("error = %v", err)
	}
}

func TestBuildDriver_MCP_RejectsMissingFields(t *testing.T) {
	cases := []struct {
		name string
		p    *Provider
		want string
	}{
		{"missing transport", &Provider{Name: "x", Driver: DriverMCP}, "transport is required"},
		{"stdio missing command", &Provider{Name: "x", Driver: DriverMCP, Transport: "stdio"}, "command is required"},
		{"http missing endpoint", &Provider{Name: "x", Driver: DriverMCP, Transport: "http"}, "endpoint is required"},
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

func TestBuildDriver_MCP_StdioSuccess(t *testing.T) {
	p := &Provider{
		Name:      "semgrep-mcp",
		Driver:    DriverMCP,
		Transport: "stdio",
		Command:   "echo",
	}
	d, err := BuildDriver(p)
	if err != nil {
		t.Fatal(err)
	}
	if d.Driver() != DriverMCP {
		t.Errorf("Driver() = %q", d.Driver())
	}
}

func TestDefaultSeverityVocab_PerPreset(t *testing.T) {
	cases := []struct {
		prompt string
		want   []string
	}{
		{"vocabulary is critical | high | medium | low", []string{"critical", "high", "medium", "low", "informational"}},
		{"options are recommended | alternative | risky | speculative", []string{"recommended", "alternative", "risky", "speculative"}},
		{"any other prompt", []string{"blocker", "issue", "minor", "info"}},
	}
	for i, tc := range cases {
		got := defaultSeverityVocab(tc.prompt)
		if len(got) != len(tc.want) {
			t.Errorf("case %d: got %v want %v", i, got, tc.want)
			continue
		}
		for j := range got {
			if got[j] != tc.want[j] {
				t.Errorf("case %d: got[%d]=%q want %q", i, j, got[j], tc.want[j])
			}
		}
	}
}
