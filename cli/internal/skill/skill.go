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
	// Hooks declares that the skill ships agent hooks (entries in
	// skill.yaml `hooks:` that get registered into the agent's
	// settings file at install time). The CLI prompts before
	// installing skills with hooks: true.
	Hooks bool `yaml:"hooks,omitempty"`
}

type Trust struct {
	ExpectedSigner string `yaml:"expected-signer,omitempty"`
}

type Deps struct {
	Skills []string `yaml:"skills,omitempty"`
}

// Hook is one entry in a skill's hooks block. The CLI translates these
// into per-agent native hook configs (Claude settings.json, Codex
// config.toml, Gemini settings.json) at install time.
type Hook struct {
	// ID must be unique within the skill. Used as part of the
	// idempotency marker so re-installs update prior entries instead
	// of duplicating.
	ID string `yaml:"id"`
	// Event is the kaijutsu canonical event name. Translated per
	// agent: pre-tool-use, post-tool-use, session-start, session-end,
	// notification, user-prompt-submit, pre-compact,
	// permission-request, before-agent, after-agent, before-model,
	// after-model, before-tool-selection.
	Event string `yaml:"event"`
	// Matcher is the per-agent filter expression — e.g. "Bash" for
	// Claude/Gemini tool-name match, "tool_name == 'shell_command'"
	// for Codex. The CLI passes it through unmodified; authors should
	// use the canonical agent-neutral form documented in SCHEMA.md.
	Matcher string `yaml:"matcher"`
	// Script is the path inside the skill directory to the hook
	// script. Resolved against the install location at hook-register
	// time.
	Script         string `yaml:"script"`
	TimeoutSeconds int    `yaml:"timeout_seconds,omitempty"`
	CanBlock       *bool  `yaml:"can_block,omitempty"`
	Description    string `yaml:"description,omitempty"`
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
	Hooks       []Hook      `yaml:"hooks,omitempty"`
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
	seen := map[string]bool{}
	for i, h := range s.Hooks {
		if h.ID == "" {
			return fmt.Errorf("skill.yaml: hooks[%d]: id is required", i)
		}
		if !hookIDPattern.MatchString(h.ID) {
			return fmt.Errorf("skill.yaml: hooks[%d]: invalid id %q (must match %s)", i, h.ID, hookIDPattern.String())
		}
		if seen[h.ID] {
			return fmt.Errorf("skill.yaml: hooks[%d]: duplicate id %q", i, h.ID)
		}
		seen[h.ID] = true
		if h.Event == "" {
			return fmt.Errorf("skill.yaml: hooks[%d]: event is required", i)
		}
		if !validHookEvents[h.Event] {
			return fmt.Errorf("skill.yaml: hooks[%d]: invalid event %q", i, h.Event)
		}
		if h.Matcher == "" {
			return fmt.Errorf("skill.yaml: hooks[%d]: matcher is required", i)
		}
		if h.Script == "" {
			return fmt.Errorf("skill.yaml: hooks[%d]: script is required", i)
		}
	}
	if len(s.Hooks) > 0 && !s.Permissions.Hooks {
		return errors.New("skill.yaml: skills declaring hooks must set permissions.hooks: true")
	}
	return nil
}

var hookIDPattern = regexp.MustCompile(`^[a-z][a-z0-9-]*[a-z0-9]$`)

// validHookEvents is the canonical kaijutsu event vocabulary. Mirrors
// the list documented in SCHEMA.md and translated per agent in
// internal/hooks/.
var validHookEvents = map[string]bool{
	"pre-tool-use":          true,
	"post-tool-use":         true,
	"session-start":         true,
	"session-end":           true,
	"notification":          true,
	"user-prompt-submit":    true,
	"pre-compact":           true,
	"permission-request":    true,
	"before-agent":          true,
	"after-agent":           true,
	"before-model":          true,
	"after-model":           true,
	"before-tool-selection": true,
}
