package agents

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestFor_KnownDrivers(t *testing.T) {
	cases := []struct {
		name        string
		wantDriver  DriverKind
		wantNameStr string
	}{
		{"claude", DriverCLI, "claude"},
		{"codex", DriverCLI, "codex"},
		{"antigravity", DriverCLI, "antigravity"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := For(tc.name)
			if d == nil {
				t.Fatalf("For(%q) returned nil", tc.name)
			}
			if got := d.Name(); got != tc.wantNameStr {
				t.Errorf("Name() = %q, want %q", got, tc.wantNameStr)
			}
			if got := d.Driver(); got != tc.wantDriver {
				t.Errorf("Driver() = %q, want %q", got, tc.wantDriver)
			}
		})
	}
}

func TestFor_UnknownReturnsNil(t *testing.T) {
	cases := []string{"", "deepseek", "glm", "Claude", "CLAUDE"}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			if d := For(name); d != nil {
				t.Errorf("For(%q) = %#v; want nil", name, d)
			}
		})
	}
}

// driverKindCases is the canonical set of DriverKind values. Any new
// addition needs an entry here AND the count assertion below — that's
// the trip-wire for accidentally adding a kind without updating the
// cache-key salt rules elsewhere.
var driverKindCases = []struct {
	k    DriverKind
	want string
}{
	{DriverCLI, "cli"},
	{DriverHTTP, "http"},
	{DriverCLICompat, "cli-compat"},
	{DriverMCP, "mcp"},
}

func TestDriverKindEnumStable(t *testing.T) {
	// Cache-key compat: driver kind strings are part of the v0.6+
	// cache-key salt. Drift here invalidates user caches silently.
	for _, tc := range driverKindCases {
		if string(tc.k) != tc.want {
			t.Errorf("DriverKind = %q, want %q", string(tc.k), tc.want)
		}
	}
	const wantCount = 4
	if len(driverKindCases) != wantCount {
		t.Errorf("driverKindCases has %d entries; want %d. If you added a new DriverKind, update both this table and any cache-key/salt logic that enumerates kinds.", len(driverKindCases), wantCount)
	}
}

var cacheStatusCases = []struct {
	s    CacheStatus
	want string
}{
	{CacheUnsupported, "unsupported"},
	{CacheHit, "hit"},
	{CacheMiss, "miss"},
	{CachePartial, "partial"},
	{CacheSkippedShort, "skipped-too-short"},
}

func TestCacheStatusEnumStable(t *testing.T) {
	for _, tc := range cacheStatusCases {
		if string(tc.s) != tc.want {
			t.Errorf("CacheStatus = %q, want %q", string(tc.s), tc.want)
		}
	}
	const wantCount = 5
	if len(cacheStatusCases) != wantCount {
		t.Errorf("cacheStatusCases has %d entries; want %d. If you added a new CacheStatus, update this table and any cost-calculation logic that enumerates statuses.", len(cacheStatusCases), wantCount)
	}
}

func TestStripNestedAgentEnv(t *testing.T) {
	in := []string{
		"PATH=/usr/bin",
		"HOME=/home/test",
		"CLAUDECODE=1",
		"CLAUDE_CODE_ENTRYPOINT=cli",
		"CLAUDE_CODE_EXECPATH=/foo",
		"CLAUDE_CODE_FUTURE_VAR=baz",
		"AI_AGENT=claude-code-foo",
		"GEMINI_SESSION=abc",
		"CODEX_SESSION=def",
		// Real user secrets — must survive even when prefix matches
		// a vendor name. Codex/Gemini API keys are exact-match-skipped.
		"DEEPSEEK_API_KEY=sk-keep",
		"CODEX_API_KEY=keep-me-i-am-secret",
		"GEMINI_API_KEY=keep-me-too",
		"USER=keep",
	}
	got := stripNestedAgentEnv(in)
	keep := map[string]bool{}
	for _, e := range got {
		eq := strings.IndexByte(e, '=')
		if eq > 0 {
			keep[e[:eq]] = true
		}
	}
	mustKeep := []string{
		"PATH", "HOME", "USER",
		"DEEPSEEK_API_KEY", "CODEX_API_KEY", "GEMINI_API_KEY",
	}
	mustStrip := []string{
		"CLAUDECODE", "CLAUDE_CODE_ENTRYPOINT", "CLAUDE_CODE_EXECPATH",
		"CLAUDE_CODE_FUTURE_VAR", "AI_AGENT",
		"GEMINI_SESSION", "CODEX_SESSION",
	}
	for _, k := range mustKeep {
		if !keep[k] {
			t.Errorf("expected %q kept, was stripped (API_KEY collision check)", k)
		}
	}
	for _, k := range mustStrip {
		if keep[k] {
			t.Errorf("expected %q stripped, was kept", k)
		}
	}
}

func TestIsTransientCLIError(t *testing.T) {
	cases := []struct {
		msg  string
		want bool
	}{
		// Transient — retry-worthy
		{"foo bar broken pipe baz", true},
		{"transport: stream error: code 0", true},
		{"rpc error: code = Unavailable desc = backend dead", true},
		{"connection reset by peer", true},
		// Permanent — should NOT retry (avoid 2x cost on user errors)
		{"claude exited exit status 1 after 30s: bad config", false},
		{"context deadline exceeded", false},
		{"executable file not found in $PATH", false},
		{"permission denied opening config", false},
		{"some other error", false},
	}
	for _, tc := range cases {
		if got := isTransientCLIError(tc.msg); got != tc.want {
			t.Errorf("isTransientCLIError(%q) = %v, want %v", tc.msg, got, tc.want)
		}
	}
}

// TestRunCLI_ErrorPath validates the runCLI invariant: when the child
// process exits non-zero, Result.Err is populated with the same string
// the returned error wraps (caller can use either source).
func TestRunCLI_ErrorPath(t *testing.T) {
	res, err := runCLI(context.Background(), InvokeOpts{}, "false", nil, "")
	if err == nil {
		t.Fatal("runCLI(false) returned nil error; want non-nil")
	}
	if res.Err == "" {
		t.Error("runCLI(false): Result.Err is empty; want populated")
	}
	if res.Err != err.Error() {
		t.Errorf("runCLI(false): Result.Err = %q, error.Error() = %q; want equal", res.Err, err.Error())
	}
	if !strings.Contains(res.Err, "false exited") {
		t.Errorf("runCLI(false): Result.Err = %q; want substring %q", res.Err, "false exited")
	}
	if res.Driver != DriverCLI {
		t.Errorf("runCLI: Result.Driver = %q, want %q", res.Driver, DriverCLI)
	}
	if res.CacheStatus != CacheUnsupported {
		t.Errorf("runCLI: Result.CacheStatus = %q, want %q", res.CacheStatus, CacheUnsupported)
	}
	if res.Duration <= 0 {
		t.Error("runCLI: Result.Duration is 0; want positive")
	}
}

// TestRunCLI_PermanentErrorDoesNotRetry confirms the retry classifier
// keeps "exit status 1" out of the transient set — earlier draft had
// generic exit-1 marked transient and would have wasted a 2nd call
// on every legitimate user error.
func TestRunCLI_PermanentErrorDoesNotRetry(t *testing.T) {
	start := time.Now()
	_, err := runCLI(context.Background(), InvokeOpts{}, "false", nil, "")
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected error from /usr/bin/false")
	}
	// Single attempt = sub-100ms; retry would push past 500ms sleep.
	if elapsed > 400*time.Millisecond {
		t.Errorf("permanent error retried (elapsed %v); should fail fast", elapsed)
	}
}

// TestRunCLI_TimeoutFromOpts validates that opts.Timeout actually
// derives a child context — without this, the InvokeOpts.Timeout field
// is documented but ignored, breaking the contract.
func TestRunCLI_TimeoutFromOpts(t *testing.T) {
	// `sleep 10` would normally run for 10 seconds; opts.Timeout=50ms
	// must terminate it well under that.
	start := time.Now()
	_, err := runCLI(context.Background(), InvokeOpts{Timeout: 50 * time.Millisecond}, "sleep", []string{"10"}, "")
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("runCLI(sleep 10, Timeout=50ms) returned nil error; want non-nil (deadline exceeded)")
	}
	if elapsed > 5*time.Second {
		t.Errorf("runCLI: elapsed %v; opts.Timeout was apparently ignored (want <5s)", elapsed)
	}
}
