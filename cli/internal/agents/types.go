package agents

// Provider is one entry in the agents.yaml provider catalog. Identifies
// a single dispatchable target (CLI binary, HTTP endpoint, MCP server,
// or cli-compat wrapper). Secrets are never stored here — `APIKeyEnv`
// holds an environment-variable name that the loader reads at invoke
// time.
type Provider struct {
	// Name is the lookup key (e.g. "claude", "deepseek", "semgrep-mcp").
	Name string `yaml:"-" json:"name"`

	// Driver kind: "cli" | "http" | "cli-compat" | "mcp".
	Driver DriverKind `yaml:"driver,omitempty" json:"driver,omitempty"`

	// CLI driver fields.
	Cmd  string   `yaml:"cmd,omitempty" json:"cmd,omitempty"`
	Args []string `yaml:"args,omitempty" json:"args,omitempty"`

	// HTTP driver fields.
	Protocol  string `yaml:"protocol,omitempty" json:"protocol,omitempty"`   // "openai-compat" | "anthropic-compat"
	BaseURL   string `yaml:"base_url,omitempty" json:"base_url,omitempty"`
	Model     string `yaml:"model,omitempty" json:"model,omitempty"`
	APIKeyEnv string `yaml:"api_key_env,omitempty" json:"api_key_env,omitempty"`

	// cli-compat driver fields.
	BaseCLI string            `yaml:"base_cli,omitempty" json:"base_cli,omitempty"`
	Env     map[string]string `yaml:"env,omitempty" json:"env,omitempty"`     // literal env vars
	EnvKey  map[string]string `yaml:"env_key,omitempty" json:"env_key,omitempty"` // env-var indirection: VAR -> os.Getenv(ENV_NAME)

	// MCP driver fields.
	Transport      string            `yaml:"transport,omitempty" json:"transport,omitempty"` // "stdio" | "http"
	Command        string            `yaml:"command,omitempty" json:"command,omitempty"`     // mcp stdio: executable name (separate from cli driver's `Cmd` for clarity in YAML)
	Endpoint       string            `yaml:"endpoint,omitempty" json:"endpoint,omitempty"`
	Headers        map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`         // header-name -> ENV_NAME (indirection)
	HeadersLiteral map[string]string `yaml:"headers_literal,omitempty" json:"headers_literal,omitempty"`
	ToolName       string            `yaml:"tool_name,omitempty" json:"tool_name,omitempty"`
	TimeoutSec     int               `yaml:"timeout_seconds,omitempty" json:"timeout_seconds,omitempty"`

	// Cost rate card (for --estimate; ignored when absent).
	Cost *CostRates `yaml:"cost,omitempty" json:"cost,omitempty"`
}

// CostRates carries per-provider pricing for `--estimate`. Rates are
// in USD per million tokens; `RateCardDate` (YYYY-MM-DD) flags
// staleness when `--estimate` runs more than 90 days later.
type CostRates struct {
	InputPerMtok       float64 `yaml:"input_per_mtok" json:"input_per_mtok"`
	OutputPerMtok      float64 `yaml:"output_per_mtok" json:"output_per_mtok"`
	CachedInputPerMtok float64 `yaml:"cached_input_per_mtok,omitempty" json:"cached_input_per_mtok,omitempty"`
	RateCardDate       string  `yaml:"rate_card_date,omitempty" json:"rate_card_date,omitempty"`
}

// Persona = (provider, optional model override, system prompt, tags).
// The unit of swarm participant identity. Skills can require persona
// tags via `requires_persona_tags`.
type Persona struct {
	// Name is the lookup key (e.g. "default-claude", "paranoid-security-claude").
	Name string `yaml:"-" json:"name"`

	// Provider is the provider name this persona dispatches to.
	Provider string `yaml:"provider" json:"provider"`

	// SystemPrompt is prepended to the skill's system prompt at swarm
	// dispatch. Empty for default personas (preserves v0.5 cache-key
	// compat).
	SystemPrompt string `yaml:"system_prompt,omitempty" json:"system_prompt,omitempty"`

	// Model optionally overrides the provider's default model.
	Model string `yaml:"model,omitempty" json:"model,omitempty"`

	// Tags categorize personas for skill `requires_persona_tags` gates.
	Tags []string `yaml:"tags,omitempty" json:"tags,omitempty"`
}

// Defaults is the optional `defaults:` block in agents.yaml.
type Defaults struct {
	HTTP                   *HTTPDefaults     `yaml:"http,omitempty" json:"http,omitempty"`
	CLICompatTelemetryKill map[string]string `yaml:"cli_compat_telemetry_kill,omitempty" json:"cli_compat_telemetry_kill,omitempty"`
}

// HTTPDefaults are HTTP-driver-wide settings.
type HTTPDefaults struct {
	TimeoutSec int `yaml:"timeout_seconds,omitempty" json:"timeout_seconds,omitempty"`
}

// SchemaVersion is the agents.yaml schema version this binary
// understands. Loader rejects unknown versions hard so users get a
// clear migration message instead of silent field-drop.
const SchemaVersion = 1

// GlobalConfig is the parsed shape of ~/.kaijutsu/agents.yaml.
// Holds the user's full provider catalog + persona declarations +
// global defaults.
type GlobalConfig struct {
	Version   int                  `yaml:"version" json:"version"`
	Providers map[string]*Provider `yaml:"providers,omitempty" json:"providers,omitempty"`
	Personas  map[string]*Persona  `yaml:"personas,omitempty" json:"personas,omitempty"`
	Defaults  *Defaults            `yaml:"defaults,omitempty" json:"defaults,omitempty"`
}

// ProjectConfig is the parsed shape of <repo>/.kaijutsu/agents.yaml.
// Project-scoped: which providers this repo's swarm uses + per-repo
// overrides + project-only personas + project-only providers.
//
// Providers vs Overrides: `providers:` declares new (or replaces
// upstream) full provider definitions; `overrides:` applies partial
// mutations onto an upstream-defined provider, leaving non-overridden
// fields intact. Resolver tries `providers` first, then `overrides`
// is applied last.
type ProjectConfig struct {
	Version   int                  `yaml:"version" json:"version"`
	Enabled   []string             `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	Providers map[string]*Provider `yaml:"providers,omitempty" json:"providers,omitempty"`
	Overrides map[string]*Provider `yaml:"overrides,omitempty" json:"overrides,omitempty"`
	Personas  map[string]*Persona  `yaml:"personas,omitempty" json:"personas,omitempty"`
}
