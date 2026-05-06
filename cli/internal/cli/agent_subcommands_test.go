package cli

import (
	"testing"

	"github.com/momentmaker/kaijutsu/cli/internal/agents"
)

func TestBuildProviderFromFlags_CatalogPath(t *testing.T) {
	p, err := buildProviderFromFlags("claude", "", "", nil, "", "", "", "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if p.Driver != agents.DriverCLI {
		t.Errorf("Driver = %q, want cli", p.Driver)
	}
	if p.Cmd != "claude" {
		t.Errorf("Cmd = %q", p.Cmd)
	}
}

func TestBuildProviderFromFlags_NotInCatalogRequiresDriver(t *testing.T) {
	_, err := buildProviderFromFlags("opencode", "", "", nil, "", "", "", "", "", nil, nil)
	if err == nil {
		t.Fatal("expected error; got nil")
	}
}

func TestBuildProviderFromFlags_HTTPRequiresFields(t *testing.T) {
	cases := []struct {
		name, protocol, baseURL, model string
		wantErr                        bool
	}{
		{"missing protocol", "", "https://x", "m", true},
		{"missing base", "openai-compat", "", "m", true},
		{"missing model", "openai-compat", "https://x", "", true},
		{"all set", "openai-compat", "https://x", "m", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := buildProviderFromFlags("x", "http", "", nil, tc.protocol, tc.baseURL, tc.model, "", "", nil, nil)
			if tc.wantErr && err == nil {
				t.Error("expected error; got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestBuildProviderFromFlags_CLICompatRequiresBaseCLI(t *testing.T) {
	_, err := buildProviderFromFlags("foo", "cli-compat", "", nil, "", "", "", "", "", nil, nil)
	if err == nil {
		t.Fatal("expected error for missing --base-cli")
	}
}

func TestParseKeyValPairs(t *testing.T) {
	got := parseKeyValPairs([]string{"FOO=bar", "BAZ=qux", "skip-me", "EMPTY="})
	if got["FOO"] != "bar" || got["BAZ"] != "qux" {
		t.Errorf("parsed = %v", got)
	}
	if _, ok := got["skip-me"]; ok {
		t.Error("entries without '=' should be dropped")
	}
}

func TestListsEqual(t *testing.T) {
	cases := []struct {
		a, b []string
		want bool
	}{
		{[]string{"a", "b"}, []string{"b", "a"}, true},
		{[]string{"a"}, []string{"a", "b"}, false},
		{[]string{"a"}, []string{}, false},
		{[]string{}, []string{}, true},
	}
	for _, tc := range cases {
		if got := listsEqual(tc.a, tc.b); got != tc.want {
			t.Errorf("listsEqual(%v, %v) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestUnionStrings_DedupesPreservingOrder(t *testing.T) {
	got := unionStrings([]string{"a", "b"}, []string{"b", "c", "a"})
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
