package cli

import (
	"bytes"
	"os"
	"strings"
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

func TestLookupTestProvider_LayerFallthrough(t *testing.T) {
	global := &agents.GlobalConfig{
		Providers: map[string]*agents.Provider{
			"global-only": {Name: "global-only", Driver: agents.DriverHTTP, BaseURL: "https://g", Model: "g"},
		},
	}
	project := &agents.ProjectConfig{
		Providers: map[string]*agents.Provider{
			"project-only": {Name: "project-only", Driver: agents.DriverHTTP, BaseURL: "https://p", Model: "p"},
			"shared":       {Name: "shared", Driver: agents.DriverHTTP, BaseURL: "https://shared-project", Model: "p"},
		},
	}
	cases := []struct {
		name   string
		want   string
		errSub string
	}{
		{"project-only", "https://p", ""},
		{"global-only", "https://g", ""},
		{"shared", "https://shared-project", ""}, // project beats global
		{"claude", "", ""},                       // built-in cli, no BaseURL
		{"nonexistent", "", "not found"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := lookupTestProvider(tc.name, global, project)
			if tc.errSub != "" {
				if err == nil {
					t.Fatalf("expected error for %q; got nil", tc.name)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.BaseURL != tc.want {
				t.Errorf("BaseURL = %q, want %q", got.BaseURL, tc.want)
			}
		})
	}
}

func TestLookupTestProvider_FindsBuiltinWithoutEnable(t *testing.T) {
	// User just installed jutsu — no agents.yaml at all. `agent test claude`
	// should still work because BuiltinProviders has it.
	got, err := lookupTestProvider("claude", &agents.GlobalConfig{}, &agents.ProjectConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "claude" || got.Driver != agents.DriverCLI {
		t.Errorf("got %+v", got)
	}
}

func TestRemoveFromProject_DropsProviderAndEnabled(t *testing.T) {
	tmp := t.TempDir()
	dir := tmp + "/.kaijutsu"
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	yamlIn := `version: 1
enabled: [claude, deepseek, gemini]
providers:
  deepseek:
    driver: http
    protocol: openai-compat
    base_url: https://x
    model: m
`
	if err := os.WriteFile(dir+"/agents.yaml", []byte(yamlIn), 0644); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := removeFromProject(&buf, tmp, "deepseek"); err != nil {
		t.Fatal(err)
	}
	c, err := agents.LoadProjectConfig(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := c.Providers["deepseek"]; ok {
		t.Error("deepseek not removed from providers")
	}
	for _, n := range c.Enabled {
		if n == "deepseek" {
			t.Error("deepseek still in enabled list")
		}
	}
}

func TestRemoveFromProject_NoOpWhenAbsent(t *testing.T) {
	tmp := t.TempDir()
	var buf bytes.Buffer
	if err := removeFromProject(&buf, tmp, "ghost"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "no project entry for") {
		t.Errorf("expected no-op message; got %q", buf.String())
	}
}

func TestScanCrossRepoReferences_FindsHits(t *testing.T) {
	root := t.TempDir()
	for _, repo := range []string{"a", "b"} {
		dir := root + "/" + repo + "/.kaijutsu"
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		var enabled string
		if repo == "a" {
			enabled = "[claude, deepseek]"
		} else {
			enabled = "[claude]"
		}
		body := "version: 1\nenabled: " + enabled + "\n"
		if err := os.WriteFile(dir+"/agents.yaml", []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	hits := scanCrossRepoReferences(root, "deepseek")
	if len(hits) != 1 {
		t.Fatalf("hits = %v, want 1", hits)
	}
	if !strings.HasSuffix(hits[0], "/a") {
		t.Errorf("hit = %q, want suffix /a", hits[0])
	}
}

func TestScanCrossRepoReferences_NoneFound(t *testing.T) {
	root := t.TempDir()
	dir := root + "/empty-repo/.kaijutsu"
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir+"/agents.yaml", []byte("version: 1\nenabled: [claude]\n"), 0644); err != nil {
		t.Fatal(err)
	}
	hits := scanCrossRepoReferences(root, "deepseek")
	if len(hits) != 0 {
		t.Errorf("hits = %v, want empty", hits)
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
