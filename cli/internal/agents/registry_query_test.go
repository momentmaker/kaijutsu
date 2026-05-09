package agents

import (
	"sort"
	"testing"
)

// makeResolved builds a Resolved + raw configs fixture for tests.
// Built-ins go via BuiltinPersonas; user-home goes in global config;
// user-project goes in project config. mirrors what Resolve produces.
func makeResolved(t *testing.T, homePersonas, projectPersonas map[string]*Persona) (*Resolved, *GlobalConfig, *ProjectConfig) {
	t.Helper()
	resolved := &Resolved{Personas: map[string]*Persona{}}
	for name, p := range BuiltinPersonas() {
		c := *p
		resolved.Personas[name] = &c
	}
	for name, p := range homePersonas {
		c := *p
		c.Name = name
		resolved.Personas[name] = &c
	}
	for name, p := range projectPersonas {
		c := *p
		c.Name = name
		resolved.Personas[name] = &c
	}
	global := &GlobalConfig{Personas: homePersonas}
	project := &ProjectConfig{Personas: projectPersonas}
	return resolved, global, project
}

func TestBrowsePersonas_NoFilterReturnsAll(t *testing.T) {
	r, g, p := makeResolved(t, nil, nil)
	rows := BrowsePersonas(r, g, p, BrowseFilters{})
	if len(rows) != len(BuiltinPersonas()) {
		t.Errorf("got %d rows, want %d (built-ins only)", len(rows), len(BuiltinPersonas()))
	}
	for _, row := range rows {
		if row.Source != "built-in" {
			t.Errorf("persona %q source = %q, want built-in", row.Name, row.Source)
		}
	}
}

func TestBrowsePersonas_TagFilter(t *testing.T) {
	r, g, p := makeResolved(t, nil, nil)
	rows := BrowsePersonas(r, g, p, BrowseFilters{Tag: "security"})
	if len(rows) == 0 {
		t.Fatal("expected at least one security-tagged persona")
	}
	for _, row := range rows {
		hasTag := false
		for _, t := range row.Tags {
			if t == "security" {
				hasTag = true
				break
			}
		}
		if !hasTag {
			t.Errorf("persona %q surfaced under --tag security but tags = %v", row.Name, row.Tags)
		}
	}
}

func TestBrowsePersonas_ProviderFilter(t *testing.T) {
	r, g, p := makeResolved(t, nil, nil)
	rows := BrowsePersonas(r, g, p, BrowseFilters{Provider: "gemini"})
	if len(rows) == 0 {
		t.Fatal("expected at least one gemini persona")
	}
	for _, row := range rows {
		if row.Provider != "gemini" {
			t.Errorf("persona %q surfaced under --provider gemini but provider = %q", row.Name, row.Provider)
		}
	}
}

func TestBrowsePersonas_SourceFilter(t *testing.T) {
	homePersonas := map[string]*Persona{
		"my-home-persona": {Provider: "claude", SystemPrompt: "you are at home", Tags: []string{"home"}},
	}
	r, g, p := makeResolved(t, homePersonas, nil)
	rows := BrowsePersonas(r, g, p, BrowseFilters{Source: "user:home"})
	if len(rows) != 1 {
		t.Fatalf("expected 1 user:home persona; got %d", len(rows))
	}
	if rows[0].Name != "my-home-persona" {
		t.Errorf("got %q; want my-home-persona", rows[0].Name)
	}
	if rows[0].Source != "user:home" {
		t.Errorf("source = %q; want user:home", rows[0].Source)
	}
}

func TestBrowsePersonas_CombinedFilters(t *testing.T) {
	homePersonas := map[string]*Persona{
		"my-secure-claude": {Provider: "claude", Tags: []string{"security"}, SystemPrompt: "audit"},
		"my-arch-claude":   {Provider: "claude", Tags: []string{"architecture"}, SystemPrompt: "review"},
	}
	r, g, p := makeResolved(t, homePersonas, nil)
	// Combine: tag=security AND provider=claude AND source=user:home
	rows := BrowsePersonas(r, g, p, BrowseFilters{Tag: "security", Provider: "claude", Source: "user:home"})
	if len(rows) != 1 {
		t.Fatalf("expected 1 row; got %d (%v)", len(rows), rows)
	}
	if rows[0].Name != "my-secure-claude" {
		t.Errorf("got %q; want my-secure-claude", rows[0].Name)
	}
}

func TestBrowsePersonas_StableSortByName(t *testing.T) {
	r, g, p := makeResolved(t, nil, nil)
	rows := BrowsePersonas(r, g, p, BrowseFilters{})
	names := make([]string, len(rows))
	for i, row := range rows {
		names[i] = row.Name
	}
	if !sort.StringsAreSorted(names) {
		t.Errorf("rows not sorted by name: %v", names)
	}
}

func TestBrowsePersonas_ProjectShadowsHome(t *testing.T) {
	homePersonas := map[string]*Persona{
		"shared-name": {Provider: "claude", SystemPrompt: "from home"},
	}
	projectPersonas := map[string]*Persona{
		"shared-name": {Provider: "gemini", SystemPrompt: "from project"},
	}
	r, g, p := makeResolved(t, homePersonas, projectPersonas)
	rows := BrowsePersonas(r, g, p, BrowseFilters{Source: "user:project"})
	found := false
	for _, row := range rows {
		if row.Name == "shared-name" {
			found = true
			if row.Source != "user:project" {
				t.Errorf("source = %q; want user:project", row.Source)
			}
			if row.Provider != "gemini" {
				t.Errorf("provider = %q; want gemini (project should shadow home)", row.Provider)
			}
			if row.SystemPrompt != "from project" {
				t.Errorf("system_prompt mismatch; got %q", row.SystemPrompt)
			}
		}
	}
	if !found {
		t.Fatal("shared-name should appear under user:project source")
	}
	// Also verify it does NOT appear under user:home source
	homeRows := BrowsePersonas(r, g, p, BrowseFilters{Source: "user:home"})
	for _, row := range homeRows {
		if row.Name == "shared-name" {
			t.Error("shared-name should NOT appear under user:home (project shadows it)")
		}
	}
}

func TestBrowsePersonas_UserShadowsBuiltin(t *testing.T) {
	// Built-in name "paranoid-security-claude" exists in BuiltinPersonas.
	// Redefine it in user home.
	homePersonas := map[string]*Persona{
		"paranoid-security-claude": {Provider: "deepseek", SystemPrompt: "user override"},
	}
	r, g, p := makeResolved(t, homePersonas, nil)
	rows := BrowsePersonas(r, g, p, BrowseFilters{})
	for _, row := range rows {
		if row.Name == "paranoid-security-claude" {
			if row.Source != "user:home" {
				t.Errorf("source = %q; want user:home (user override should win)", row.Source)
			}
			if row.Provider != "deepseek" {
				t.Errorf("provider = %q; want deepseek (user override)", row.Provider)
			}
		}
	}
}

func TestBrowsePersonas_NilResolvedReturnsEmpty(t *testing.T) {
	rows := BrowsePersonas(nil, nil, nil, BrowseFilters{})
	if rows == nil {
		t.Error("BrowsePersonas should return empty slice, not nil")
	}
	if len(rows) != 0 {
		t.Errorf("expected 0 rows for nil resolved; got %d", len(rows))
	}
}
