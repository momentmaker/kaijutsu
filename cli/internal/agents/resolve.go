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

// Resolve produces a unified view from global ⊕ project configs.
//
// Provider resolution per name (highest → lowest priority):
//   1. project.Providers[name]   (project-scoped full definition)
//   2. global.Providers[name]    (user global catalog)
//   3. BuiltinProviders[name]    (vendored catalog fallback)
// Then project.Overrides[name] is merged ON TOP of whichever base
// won (overrides modify the resolved base; they do not become the
// base themselves).
//
// Persona resolution per name (highest → lowest priority, full
// replacement at each layer — no field merge):
//   1. project.Personas[name]
//   2. global.Personas[name]
//   3. BuiltinPersonas[name]   (default-* + 4 reference flavored)
// After the user layers apply, default-<provider> personas are
// auto-synthesized for any enabled provider that lacks one.
//
// When project.Enabled is empty AND no provider catalog exists in
// either layer, the legacy v0.5 mix is synthesized (`[claude, codex,
// gemini]`) so users who never wrote agents.yaml get byte-identical
// v0.5 behavior.
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

		if err := validateDriverKind(base); err != nil {
			return nil, err
		}

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
	c.Env = copyStringMap(p.Env)
	c.EnvKey = copyStringMap(p.EnvKey)
	c.Headers = copyStringMap(p.Headers)
	c.HeadersLiteral = copyStringMap(p.HeadersLiteral)
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
// them non-empty; scalar fields replace when non-zero. Map and slice
// fields are deep-copied so mutating dst later cannot leak back to
// src (which may be a long-lived config struct shared across calls).
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
		dst.Env = copyStringMap(src.Env)
	}
	if len(src.EnvKey) > 0 {
		dst.EnvKey = copyStringMap(src.EnvKey)
	}
	if src.Transport != "" {
		dst.Transport = src.Transport
	}
	if src.Command != "" {
		dst.Command = src.Command
	}
	if src.Endpoint != "" {
		dst.Endpoint = src.Endpoint
	}
	if len(src.Headers) > 0 {
		dst.Headers = copyStringMap(src.Headers)
	}
	if len(src.HeadersLiteral) > 0 {
		dst.HeadersLiteral = copyStringMap(src.HeadersLiteral)
	}
	if src.ToolName != "" {
		dst.ToolName = src.ToolName
	}
	if src.TimeoutSec != 0 {
		dst.TimeoutSec = src.TimeoutSec
	}
	if src.Cost != nil {
		costCopy := *src.Cost
		dst.Cost = &costCopy
	}
}

// validateDriverKind rejects providers with an unrecognized driver
// kind. Loader-level enum validation — without this, a YAML typo like
// `driver: clil` parses silently and surfaces as a confusing dispatch
// error downstream.
func validateDriverKind(p *Provider) error {
	switch p.Driver {
	case DriverCLI, DriverHTTP, DriverCLICompat, DriverMCP:
		return nil
	case "":
		return fmt.Errorf("provider %q has no driver kind set; declare driver: cli|http|cli-compat|mcp", p.Name)
	}
	return fmt.Errorf("provider %q has unknown driver kind %q (allowed: cli, http, cli-compat, mcp)", p.Name, p.Driver)
}

func copyStringMap(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
