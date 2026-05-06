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
