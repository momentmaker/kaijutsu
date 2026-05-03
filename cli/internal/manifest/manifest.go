// Package manifest reads and writes the kaijutsu project manifest
// (kaijutsu.json) and lockfile (kaijutsu.lock.json).
package manifest

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

const SchemaVersion = 1

type RegistryConfig struct {
	Default string   `json:"default,omitempty"`
	Extra   []string `json:"extra,omitempty"`
}

// Manifest mirrors kaijutsu.json.
type Manifest struct {
	Version      int               `json:"version"`
	Agents       []string          `json:"agents"`
	Dependencies map[string]string `json:"dependencies"`
	Registry     *RegistryConfig   `json:"registry,omitempty"`
}

func New(agents []string) *Manifest {
	return &Manifest{
		Version:      SchemaVersion,
		Agents:       agents,
		Dependencies: map[string]string{},
		Registry: &RegistryConfig{
			Default: "https://github.com/momentmaker/kaijutsu",
		},
	}
}

func Load(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if m.Version != SchemaVersion {
		return nil, fmt.Errorf("unsupported manifest version %d", m.Version)
	}
	if m.Dependencies == nil {
		m.Dependencies = map[string]string{}
	}
	return &m, nil
}

func (m *Manifest) Save(path string) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0644)
}

// Validate returns an error if the manifest is structurally invalid.
func (m *Manifest) Validate() error {
	if m.Version != SchemaVersion {
		return errors.New("invalid version")
	}
	if len(m.Agents) == 0 {
		return errors.New("at least one agent required")
	}
	return nil
}
