// Package skill parses skill.yaml.
package skill

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

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

// Routing declares per-skill provider preferences that the swarm
// dispatcher consumes via ResolvePersonaProvider. v0.9 introduces
// this as a hint mechanism — skill authors who know "honest lens
// works best on claude" can encode the preference. The end-user
// override path (`routing.disabled: true` in ~/.kaijutsu/agents.yaml)
// is documented in the spec but deferred to v0.9.x; until then
// users override via KAIJUTSU_DISABLE_AGENTS=<comma-list> at the
// agent-availability layer (drops a provider entirely from
// detection so Phase B fails over to whatever else is available).
//
// Resolution is two-phase (see swarm.ResolvePersonaProvider):
//   - Phase A: build preferred-provider list (per-persona → default
//     → registry default).
//   - Phase B: pick first available from list. With --strict-routing
//     ON, hard-fail when none available. Without, drop persona +
//     warn.
//   - Phase C: minimum-dispatch invariant — Phase B drops every
//     persona → hard-fail regardless of --strict-routing.
type Routing struct {
	// PerPersona maps persona name to an ordered list of preferred
	// providers (e.g. honest-persona → ["claude", "codex"]). Walked
	// in declaration order; first available wins.
	PerPersona map[string][]string `yaml:"per-persona,omitempty"`
	// Default is the global fallback when persona isn't named in
	// PerPersona. Empty list (or omitted) falls through to the
	// persona registry's default mapping (existing v0.6 behavior).
	// Per spec: empty AND omitted are equivalent.
	Default []string `yaml:"default,omitempty"`
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
	Upstream    string      `yaml:"upstream,omitempty"` // attribution URL for adapted skills
	Tags        []string    `yaml:"tags,omitempty"`
	Agents      []string    `yaml:"agents"`
	Shared      []string    `yaml:"shared,omitempty"`
	Permissions Permissions `yaml:"permissions"`
	Deps        *Deps       `yaml:"deps,omitempty"`
	Trust       *Trust      `yaml:"trust,omitempty"`
	Hooks       []Hook      `yaml:"hooks,omitempty"`
	Routing     *Routing    `yaml:"routing,omitempty"`
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

// SynthesizeFromSKILLMD reads a vanilla SKILL.md (Anthropic Agent Skills
// standard — frontmatter + body, no kaijutsu skill.yaml alongside) and
// synthesizes a minimal Skill with safe defaults: bash=false,
// network=false, fs-write=false, hooks=false. license is read from the
// repo's LICENSE file (passed via repoLicensePath; "" disables the
// license check). Used by the installer's vanilla-SKILL compat mode for
// installing skills from collections like addyosmani/agent-skills that
// ship only SKILL.md files.
//
// The returned Skill carries `synthesized: true` semantics — callers
// must surface this to the user (the safe-defaults disclaimer) before
// installing.
func SynthesizeFromSKILLMD(skillMDPath, repoLicensePath string) (*Skill, error) {
	data, err := os.ReadFile(skillMDPath)
	if err != nil {
		return nil, err
	}
	name, description, err := parseSkillMDFrontmatter(data)
	if err != nil {
		return nil, fmt.Errorf("parse %s frontmatter: %w", skillMDPath, err)
	}
	if err := ValidateName(name); err != nil {
		return nil, fmt.Errorf("synthesize: invalid name in SKILL.md frontmatter: %w", err)
	}
	license := "MIT"
	if repoLicensePath != "" {
		licStr, err := detectLicense(repoLicensePath)
		if err != nil {
			return nil, fmt.Errorf("synthesize: license detection failed: %w", err)
		}
		license = licStr
	}
	return &Skill{
		Name:        name,
		Version:     "0.0.0",
		License:     license,
		Layout:      "flat",
		Description: description,
		Agents:      []string{"claude", "codex", "gemini"},
		Permissions: Permissions{
			Bash:    false,
			Network: false,
			FsWrite: false,
			Hooks:   false,
		},
	}, nil
}

// parseSkillMDFrontmatter extracts name + description from a YAML
// frontmatter block at the top of a markdown file. Returns an error if
// the file lacks frontmatter or required fields.
func parseSkillMDFrontmatter(data []byte) (name, description string, err error) {
	const delim = "---"
	body := string(data)
	if !strings.HasPrefix(strings.TrimSpace(body), delim) {
		return "", "", errors.New("no YAML frontmatter")
	}
	// Skip leading whitespace then the opening ---
	idx := strings.Index(body, delim)
	if idx < 0 {
		return "", "", errors.New("no opening frontmatter delimiter")
	}
	rest := body[idx+len(delim):]
	close := strings.Index(rest, "\n"+delim)
	if close < 0 {
		return "", "", errors.New("no closing frontmatter delimiter")
	}
	front := rest[:close]
	var meta struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
	}
	if err := yaml.Unmarshal([]byte(front), &meta); err != nil {
		return "", "", err
	}
	if meta.Name == "" {
		return "", "", errors.New("frontmatter missing name")
	}
	if meta.Description == "" {
		return "", "", errors.New("frontmatter missing description")
	}
	return meta.Name, meta.Description, nil
}

// detectLicense reads a LICENSE file and returns the SPDX-ish identifier
// for the kaijutsu compatibility allowlist (MIT, BSD-2-Clause,
// BSD-3-Clause, ISC, Apache-2.0). Falls back to "MIT" when only marker
// text is present without an explicit SPDX header.
func detectLicense(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	body := strings.ToLower(string(data))
	switch {
	case strings.Contains(body, "mit license"):
		return "MIT", nil
	case strings.Contains(body, "apache license"):
		return "Apache-2.0", nil
	case strings.Contains(body, "bsd 2-clause"):
		return "BSD-2-Clause", nil
	case strings.Contains(body, "bsd 3-clause"):
		return "BSD-3-Clause", nil
	case strings.Contains(body, "isc license"):
		return "ISC", nil
	}
	return "", fmt.Errorf("license at %s is not in the kaijutsu compatibility allowlist (MIT / BSD-2/3 / ISC / Apache-2.0)", path)
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
