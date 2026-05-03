// Package skill parses skill.yaml.
package skill

import (
	"errors"
	"fmt"
	"os"
	"regexp"

	"gopkg.in/yaml.v3"
)

// NamePattern is the regex skill names must match. Mirrors the JSON Schema
// constraint in schemas/skill.schema.json. Constraining names this tightly
// is also a security boundary: skill.Name is used as a filesystem path
// component during install and remove, so disallowing slashes and dots
// prevents path traversal via a malicious skill.yaml.
var NamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]*[a-z0-9]$`)

// ValidateName returns an error if name fails NamePattern.
func ValidateName(name string) error {
	if !NamePattern.MatchString(name) {
		return fmt.Errorf("invalid skill name %q: must match %s", name, NamePattern.String())
	}
	return nil
}

type Permissions struct {
	Bash    bool        `yaml:"bash"`
	Network bool        `yaml:"network"`
	FsWrite interface{} `yaml:"fs-write"`
}

type Trust struct {
	ExpectedSigner string `yaml:"expected-signer,omitempty"`
}

type Deps struct {
	Skills []string `yaml:"skills,omitempty"`
}

// Skill mirrors skill.yaml.
type Skill struct {
	Name        string      `yaml:"name"`
	Version     string      `yaml:"version"`
	License     string      `yaml:"license"`
	Layout      string      `yaml:"layout"`
	Description string      `yaml:"description"`
	Author      string      `yaml:"author,omitempty"`
	Homepage    string      `yaml:"homepage,omitempty"`
	Repository  string      `yaml:"repository,omitempty"`
	Tags        []string    `yaml:"tags,omitempty"`
	Agents      []string    `yaml:"agents"`
	Shared      []string    `yaml:"shared,omitempty"`
	Permissions Permissions `yaml:"permissions"`
	Deps        *Deps       `yaml:"deps,omitempty"`
	Trust       *Trust      `yaml:"trust,omitempty"`
}

func Load(path string) (*Skill, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s Skill
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if err := s.Validate(); err != nil {
		return nil, err
	}
	return &s, nil
}

func (s *Skill) Validate() error {
	if s.Name == "" {
		return errors.New("skill.yaml: name is required")
	}
	if err := ValidateName(s.Name); err != nil {
		return fmt.Errorf("skill.yaml: %w", err)
	}
	if s.Version == "" {
		return errors.New("skill.yaml: version is required")
	}
	if s.License == "" {
		return errors.New("skill.yaml: license is required")
	}
	if s.Layout != "flat" && s.Layout != "rich" {
		return fmt.Errorf("skill.yaml: layout must be flat or rich, got %q", s.Layout)
	}
	if len(s.Agents) == 0 {
		return errors.New("skill.yaml: at least one agent is required")
	}
	return nil
}
