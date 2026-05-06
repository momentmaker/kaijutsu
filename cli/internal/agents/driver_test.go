package agents

import (
	"testing"
)

func TestFor_KnownDrivers(t *testing.T) {
	cases := []struct {
		name        string
		wantDriver  DriverKind
		wantNameStr string
	}{
		{"claude", DriverCLI, "claude"},
		{"codex", DriverCLI, "codex"},
		{"gemini", DriverCLI, "gemini"},
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

func TestDriverKindEnumStable(t *testing.T) {
	// Cache-key compat: driver kind strings are part of the v0.6+
	// cache-key salt. Drift here invalidates user caches silently.
	cases := []struct {
		k    DriverKind
		want string
	}{
		{DriverCLI, "cli"},
		{DriverHTTP, "http"},
		{DriverCLICompat, "cli-compat"},
		{DriverMCP, "mcp"},
	}
	for _, tc := range cases {
		if string(tc.k) != tc.want {
			t.Errorf("DriverKind = %q, want %q", string(tc.k), tc.want)
		}
	}
}

func TestCacheStatusEnumStable(t *testing.T) {
	cases := []struct {
		s    CacheStatus
		want string
	}{
		{CacheUnsupported, "unsupported"},
		{CacheHit, "hit"},
		{CacheMiss, "miss"},
		{CachePartial, "partial"},
		{CacheSkippedShort, "skipped-too-short"},
	}
	for _, tc := range cases {
		if string(tc.s) != tc.want {
			t.Errorf("CacheStatus = %q, want %q", string(tc.s), tc.want)
		}
	}
}
