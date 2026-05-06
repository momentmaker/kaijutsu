package agents

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadGlobal_AbsentFileReturnsZeroValue(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "agents.yaml")
	c, err := loadGlobal(path)
	if err != nil {
		t.Fatalf("loadGlobal absent file: %v", err)
	}
	if c.Version != SchemaVersion {
		t.Errorf("Version = %d, want %d (zero-value should backfill)", c.Version, SchemaVersion)
	}
}

func TestLoadGlobal_RoundTrip(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "agents.yaml")
	yaml := `version: 1
providers:
  deepseek:
    driver: http
    protocol: openai-compat
    base_url: https://api.deepseek.com/v1
    model: deepseek-coder
    api_key_env: DEEPSEEK_API_KEY
personas:
  paranoid-security-claude:
    provider: claude
    system_prompt: "be paranoid"
    tags: [security]
`
	if err := os.WriteFile(path, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}
	c, err := loadGlobal(path)
	if err != nil {
		t.Fatalf("loadGlobal: %v", err)
	}
	if c.Providers["deepseek"].Driver != DriverHTTP {
		t.Errorf("deepseek.Driver = %q, want http", c.Providers["deepseek"].Driver)
	}
	if c.Providers["deepseek"].Name != "deepseek" {
		t.Errorf("Name backfill failed: got %q, want deepseek", c.Providers["deepseek"].Name)
	}
	if c.Personas["paranoid-security-claude"].SystemPrompt != "be paranoid" {
		t.Errorf("system_prompt parse failed")
	}
	if c.Personas["paranoid-security-claude"].Name != "paranoid-security-claude" {
		t.Errorf("persona Name backfill failed")
	}
}

func TestLoadGlobal_UnsupportedVersion(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "agents.yaml")
	if err := os.WriteFile(path, []byte("version: 99\n"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := loadGlobal(path)
	if err == nil {
		t.Fatal("expected error for unsupported schema version; got nil")
	}
	if !strings.Contains(err.Error(), "unsupported schema version 99") {
		t.Errorf("error = %v; want substring 'unsupported schema version 99'", err)
	}
}

func TestLoadGlobal_MalformedYAMLIsHardError(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "agents.yaml")
	if err := os.WriteFile(path, []byte("this: is: not: valid:\n  yaml"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := loadGlobal(path)
	if err == nil {
		t.Fatal("expected parse error for malformed YAML; got nil")
	}
}

func TestLoadProject_AbsentFileReturnsZeroValue(t *testing.T) {
	tmp := t.TempDir()
	c, err := LoadProjectConfig(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if c.Version != SchemaVersion {
		t.Errorf("Version = %d, want zero-value backfill to %d", c.Version, SchemaVersion)
	}
	if len(c.Enabled) != 0 {
		t.Errorf("Enabled non-empty: %v", c.Enabled)
	}
}

func TestBuiltinProviders_AllEntriesValid(t *testing.T) {
	for name, p := range BuiltinProviders() {
		if p == nil {
			t.Errorf("provider %q: nil entry", name)
			continue
		}
		if p.Name != name {
			t.Errorf("provider %q: Name field = %q, want %q", name, p.Name, name)
		}
		switch p.Driver {
		case DriverCLI:
			if p.Cmd == "" {
				t.Errorf("provider %q (cli): Cmd is empty", name)
			}
		case DriverHTTP:
			if p.Protocol == "" {
				t.Errorf("provider %q (http): Protocol empty", name)
			}
			if p.BaseURL == "" {
				t.Errorf("provider %q (http): BaseURL empty", name)
			}
			if p.Model == "" {
				t.Errorf("provider %q (http): Model empty", name)
			}
			if p.Cost != nil {
				if p.Cost.InputPerMtok < 0 {
					t.Errorf("provider %q: InputPerMtok negative (%v)", name, p.Cost.InputPerMtok)
				}
				if p.Cost.OutputPerMtok < 0 {
					t.Errorf("provider %q: OutputPerMtok negative (%v)", name, p.Cost.OutputPerMtok)
				}
				// Output normally costs more than input — sanity check
				// that we didn't transpose them when filling the rate card.
				if p.Cost.OutputPerMtok > 0 && p.Cost.InputPerMtok > 0 && p.Cost.OutputPerMtok < p.Cost.InputPerMtok {
					t.Errorf("provider %q: OutputPerMtok (%v) < InputPerMtok (%v); fields likely transposed", name, p.Cost.OutputPerMtok, p.Cost.InputPerMtok)
				}
				if p.Cost.RateCardDate == "" {
					t.Errorf("provider %q: rate card has no RateCardDate; --estimate stale-warning logic will misbehave", name)
				}
			}
		default:
			t.Errorf("provider %q: unexpected Driver kind %q in catalog", name, p.Driver)
		}
	}
}

func TestBuiltinProviders_HasLegacyMix(t *testing.T) {
	cat := BuiltinProviders()
	for _, name := range LegacyEnabledMix() {
		if _, ok := cat[name]; !ok {
			t.Errorf("legacy mix references %q but BuiltinProviders has no entry", name)
		}
	}
}

func TestLoadProject_EnabledAndOverrides(t *testing.T) {
	tmp := t.TempDir()
	dir := filepath.Join(tmp, ".kaijutsu")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	yaml := `version: 1
enabled: [claude, deepseek]
overrides:
  deepseek:
    model: deepseek-reasoner
`
	if err := os.WriteFile(filepath.Join(dir, "agents.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}
	c, err := LoadProjectConfig(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Enabled) != 2 || c.Enabled[0] != "claude" || c.Enabled[1] != "deepseek" {
		t.Errorf("Enabled = %v", c.Enabled)
	}
	if c.Overrides["deepseek"].Model != "deepseek-reasoner" {
		t.Errorf("override.Model not parsed")
	}
}
