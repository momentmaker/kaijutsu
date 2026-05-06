package agents

import (
	"fmt"
	"os"
	"sort"
)

// Resolved is the fully-merged provider + persona view consumed by
// the swarm pipeline and `jutsu agent` subcommands. Built by Resolve()
// from the global ⊕ project ⊕ built-ins layers.
type Resolved struct {
	// Providers keyed by name. Includes only providers in `Enabled`.
	Providers map[string]*Provider

	// Personas keyed by name. Includes built-ins + any user-declared
	// personas in either layer; auto-synthesized `default-<provider>`
	// for any enabled provider that lacks one.
	Personas map[string]*Persona

	// Enabled is the project's enabled provider list (resolved from
	// project config, or the legacy mix when both layers are absent).
	Enabled []string

	// MissingKeys names enabled providers whose APIKeyEnv is set but
	// the env var is not. Callers decide whether this is hard-fail
	// (swarm dispatch) or warn-only (`agent list`, `agent doctor`).
	MissingKeys []string

	// Defaults from the global config (HTTP timeouts, telemetry kill
	// envs); zero-value when absent.
	Defaults *Defaults
}

// Resolve produces a unified view from global ⊕ project configs. Order
// of precedence (highest → lowest):
//   1. project.Overrides[name] merged onto base provider
//   2. project.Personas[name] (full replacement)
//   3. global.Personas[name]  (full replacement of built-in)
//   4. global.Providers[name] (base catalog)
//   5. BuiltinProviders[name] (vendored catalog fallback)
//   6. BuiltinPersonas[name]  (default-* + reference flavored)
//
// When project.Enabled is empty AND global.Providers is empty, the
// legacy v0.5 mix is synthesized (`[claude, codex, gemini]`) so users
// who never wrote agents.yaml get byte-identical v0.5 behavior.
func Resolve(global *GlobalConfig, project *ProjectConfig) (*Resolved, error) {
	if global == nil {
		global = &GlobalConfig{}
	}
	if project == nil {
		project = &ProjectConfig{}
	}

	// --- Provider resolution ----------------------------------------

	enabled := project.Enabled
	if len(enabled) == 0 {
		// No project enabled list. Fall back to project providers,
		// then global catalog, else the legacy v0.5 mix.
		switch {
		case len(project.Providers) > 0:
			for name := range project.Providers {
				enabled = append(enabled, name)
			}
			sort.Strings(enabled)
		case len(global.Providers) > 0:
			for name := range global.Providers {
				enabled = append(enabled, name)
			}
			sort.Strings(enabled)
		default:
			enabled = LegacyEnabledMix()
		}
	}

	builtins := BuiltinProviders()
	resolvedProviders := make(map[string]*Provider, len(enabled))
	var missing []string
	// Resolution order per name (highest priority first):
	//   1. project.Providers[name]   (project-scoped full definition)
	//   2. global.Providers[name]    (user global catalog)
	//   3. builtins[name]            (vendored catalog)
	// Then project.Overrides[name] is merged on top.
	for _, name := range enabled {
		var base *Provider
		switch {
		case project.Providers[name] != nil:
			base = cloneProvider(project.Providers[name])
		case global.Providers[name] != nil:
			base = cloneProvider(global.Providers[name])
		default:
			if p, ok := builtins[name]; ok {
				base = cloneProvider(p)
			}
		}
		if base == nil {
			return nil, fmt.Errorf("provider %q is enabled but not declared in any layer (global, project, or built-in catalog). Add it via `jutsu agent add %s` or declare it in agents.yaml", name, name)
		}
		if ov, ok := project.Overrides[name]; ok && ov != nil {
			mergeProvider(base, ov)
		}
		base.Name = name
		resolvedProviders[name] = base

		if base.APIKeyEnv != "" && os.Getenv(base.APIKeyEnv) == "" {
			missing = append(missing, name)
		}
	}

	// --- Persona resolution ------------------------------------------

	resolvedPersonas := make(map[string]*Persona)
	for name, p := range BuiltinPersonas() {
		resolvedPersonas[name] = clonePersona(p)
	}
	for name, p := range global.Personas {
		if p == nil {
			continue
		}
		c := clonePersona(p)
		c.Name = name
		resolvedPersonas[name] = c
	}
	for name, p := range project.Personas {
		if p == nil {
			continue
		}
		c := clonePersona(p)
		c.Name = name
		resolvedPersonas[name] = c
	}
	// Auto-synthesize default-<provider> for any enabled provider
	// lacking one. Empty system_prompt preserves v0.5 cache compat.
	for _, name := range enabled {
		key := "default-" + name
		if _, ok := resolvedPersonas[key]; ok {
			continue
		}
		resolvedPersonas[key] = &Persona{
			Name:         key,
			Provider:     name,
			SystemPrompt: "",
		}
	}

	return &Resolved{
		Providers:   resolvedProviders,
		Personas:    resolvedPersonas,
		Enabled:     enabled,
		MissingKeys: missing,
		Defaults:    global.Defaults,
	}, nil
}

// cloneProvider returns a deep-enough copy that callers can safely
// mutate. Maps are recreated; slices are copied.
func cloneProvider(p *Provider) *Provider {
	if p == nil {
		return nil
	}
	c := *p
	if p.Args != nil {
		c.Args = append([]string(nil), p.Args...)
	}
	if p.Env != nil {
		c.Env = make(map[string]string, len(p.Env))
		for k, v := range p.Env {
			c.Env[k] = v
		}
	}
	if p.EnvKey != nil {
		c.EnvKey = make(map[string]string, len(p.EnvKey))
		for k, v := range p.EnvKey {
			c.EnvKey[k] = v
		}
	}
	if p.Headers != nil {
		c.Headers = make(map[string]string, len(p.Headers))
		for k, v := range p.Headers {
			c.Headers[k] = v
		}
	}
	if p.HeadersLiteral != nil {
		c.HeadersLiteral = make(map[string]string, len(p.HeadersLiteral))
		for k, v := range p.HeadersLiteral {
			c.HeadersLiteral[k] = v
		}
	}
	if p.Cost != nil {
		costCopy := *p.Cost
		c.Cost = &costCopy
	}
	return &c
}

// clonePersona returns a deep copy of a persona.
func clonePersona(p *Persona) *Persona {
	if p == nil {
		return nil
	}
	c := *p
	if p.Tags != nil {
		c.Tags = append([]string(nil), p.Tags...)
	}
	return &c
}

// mergeProvider applies non-zero fields from src onto dst (project
// overrides global). Slice/map fields fully replace when src declares
// them non-empty; scalar fields replace when non-zero.
func mergeProvider(dst, src *Provider) {
	if src == nil {
		return
	}
	if src.Driver != "" {
		dst.Driver = src.Driver
	}
	if src.Cmd != "" {
		dst.Cmd = src.Cmd
	}
	if len(src.Args) > 0 {
		dst.Args = append([]string(nil), src.Args...)
	}
	if src.Protocol != "" {
		dst.Protocol = src.Protocol
	}
	if src.BaseURL != "" {
		dst.BaseURL = src.BaseURL
	}
	if src.Model != "" {
		dst.Model = src.Model
	}
	if src.APIKeyEnv != "" {
		dst.APIKeyEnv = src.APIKeyEnv
	}
	if src.BaseCLI != "" {
		dst.BaseCLI = src.BaseCLI
	}
	if len(src.Env) > 0 {
		dst.Env = src.Env
	}
	if len(src.EnvKey) > 0 {
		dst.EnvKey = src.EnvKey
	}
	if src.Transport != "" {
		dst.Transport = src.Transport
	}
	if src.Endpoint != "" {
		dst.Endpoint = src.Endpoint
	}
	if len(src.Headers) > 0 {
		dst.Headers = src.Headers
	}
	if len(src.HeadersLiteral) > 0 {
		dst.HeadersLiteral = src.HeadersLiteral
	}
	if src.ToolName != "" {
		dst.ToolName = src.ToolName
	}
	if src.TimeoutSec != 0 {
		dst.TimeoutSec = src.TimeoutSec
	}
	if src.Cost != nil {
		dst.Cost = src.Cost
	}
}
