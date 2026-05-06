package agents

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// GlobalConfigPath returns ~/.kaijutsu/agents.yaml (or empty + nil if
// the home directory is unavailable, which is a hard fail at the call
// site).
func GlobalConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, ".kaijutsu", "agents.yaml"), nil
}

// ProjectConfigPath returns <root>/.kaijutsu/agents.yaml.
func ProjectConfigPath(projectRoot string) string {
	return filepath.Join(projectRoot, ".kaijutsu", "agents.yaml")
}

// LoadGlobalConfig parses ~/.kaijutsu/agents.yaml. Returns a zero-value
// GlobalConfig with `Version: SchemaVersion` populated when the file
// is absent — the resolver synthesizes built-in providers + personas
// in that case.
//
// Malformed YAML is a hard error (don't silently drop user config).
// Unknown schema version is a hard error with a clear migration hint.
func LoadGlobalConfig() (*GlobalConfig, error) {
	path, err := GlobalConfigPath()
	if err != nil {
		return nil, err
	}
	return loadGlobal(path)
}

func loadGlobal(path string) (*GlobalConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &GlobalConfig{Version: SchemaVersion}, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var c GlobalConfig
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if err := validateVersion(c.Version, path); err != nil {
		return nil, err
	}
	// Backfill Name field on map entries — yaml.v3 doesn't populate it
	// because it's `yaml:"-"`. Downstream code expects p.Name to match
	// the map key.
	for name, p := range c.Providers {
		if p == nil {
			continue
		}
		p.Name = name
	}
	for name, p := range c.Personas {
		if p == nil {
			continue
		}
		p.Name = name
	}
	return &c, nil
}

// LoadProjectConfig parses <root>/.kaijutsu/agents.yaml. Returns a
// zero-value ProjectConfig on absent file (loader synthesizes legacy
// `enabled: [claude, codex, gemini]` downstream when both project and
// global are empty, preserving v0.5 behavior).
func LoadProjectConfig(projectRoot string) (*ProjectConfig, error) {
	return loadProject(ProjectConfigPath(projectRoot))
}

func loadProject(path string) (*ProjectConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &ProjectConfig{Version: SchemaVersion}, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var c ProjectConfig
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if err := validateVersion(c.Version, path); err != nil {
		return nil, err
	}
	for name, p := range c.Providers {
		if p == nil {
			continue
		}
		p.Name = name
	}
	for name, p := range c.Overrides {
		if p == nil {
			continue
		}
		p.Name = name
	}
	for name, p := range c.Personas {
		if p == nil {
			continue
		}
		p.Name = name
	}
	return &c, nil
}

// validateVersion enforces SchemaVersion compat. Version=0 is treated
// as "absent" (default zero-value when YAML omits the field) and
// permitted with a soft warning printed by the caller. Other values
// are hard errors.
func validateVersion(v int, path string) error {
	if v == 0 || v == SchemaVersion {
		return nil
	}
	return fmt.Errorf("%s: unsupported schema version %d (this build supports %d). Run `jutsu agent migrate` or upgrade jutsu", path, v, SchemaVersion)
}
