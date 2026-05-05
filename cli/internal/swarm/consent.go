package swarm

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config mirrors .kaijutsu/pr-review.yaml. All fields optional.
// AllowMultiModel is the consent gate — must be `true` before any
// remote model call goes out.
type Config struct {
	AllowMultiModel bool     `yaml:"allow-multi-model"`
	Agents          []string `yaml:"agents,omitempty"`
	Mode            string   `yaml:"mode,omitempty"`
	MaxCostUSD      float64  `yaml:"max_cost_usd,omitempty"`
	ExcludePaths    []string `yaml:"exclude_paths,omitempty"`
	SeverityFloor   string   `yaml:"severity_floor,omitempty"`
	Synthesizer     string   `yaml:"synthesizer,omitempty"`
}

// configPathFor returns the .kaijutsu/<preset>.yaml path relative to
// the project root. Generalizes from the Phase-1 hardcoded
// "pr-review.yaml" so each preset gets its own consent + config file.
func configPathFor(preset *Preset) string {
	return filepath.Join(".kaijutsu", preset.configFileName())
}

// LoadConfig returns the parsed config or a zero-value Config if the
// file does not exist. A malformed file is a hard error so the user
// notices the problem before sending diffs to remote models.
func LoadConfig(projectRoot string, preset *Preset) (*Config, error) {
	full := filepath.Join(projectRoot, configPathFor(preset))
	data, err := os.ReadFile(full)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, err
	}
	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse %s: %w", full, err)
	}
	return &c, nil
}

// SaveConsent writes a minimal config affirming consent. Preserves
// existing fields when present.
func SaveConsent(projectRoot string, preset *Preset) error {
	full := filepath.Join(projectRoot, configPathFor(preset))
	c, err := LoadConfig(projectRoot, preset)
	if err != nil {
		return err
	}
	c.AllowMultiModel = true
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		return err
	}
	out, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(full, out, 0644)
}

// EnsureConsent is the runtime check: if the project root's preset
// config already has allow-multi-model: true, returns nil. Otherwise
// prompts the user (interactive only), persists the answer if they
// say yes, and returns an error if they decline.
//
// The non-interactive AND no-tty paths both give the same actionable
// hint: run `jutsu swarm <preset> --grant-consent` once interactively
// to persist consent, then re-run. This solves the "ran swarm from
// inside an agent CLI session and got a hang" failure mode.
func EnsureConsent(projectRoot string, preset *Preset, in io.Reader, out io.Writer, agents []AgentName, nonInteractive bool) error {
	c, err := LoadConfig(projectRoot, preset)
	if err != nil {
		return err
	}
	if c.AllowMultiModel {
		return nil
	}
	cfgPath := configPathFor(preset)
	hint := fmt.Sprintf("Run `jutsu swarm %s --grant-consent` once interactively to persist consent, OR add `allow-multi-model: true` to %s manually.", preset.Name, cfgPath)
	if nonInteractive {
		return fmt.Errorf("multi-model consent not granted. %s", hint)
	}
	if !stdinIsTTY() {
		return fmt.Errorf("multi-model consent not granted and stdin is not a terminal (running headless or piped). %s", hint)
	}
	providers := providerLabels(agents)
	fmt.Fprintf(out, "\nThis run will send input to %d model provider(s):\n", len(providers))
	for _, p := range providers {
		fmt.Fprintf(out, "  - %s\n", p)
	}
	fmt.Fprintf(out, "Per-repo consent is required. Persist `allow-multi-model: true` to %s? [y/N]: ", cfgPath)
	r := bufio.NewReader(in)
	line, _ := r.ReadString('\n')
	line = strings.TrimSpace(strings.ToLower(line))
	if line != "y" && line != "yes" {
		return errors.New("aborted: multi-model consent declined")
	}
	if err := SaveConsent(projectRoot, preset); err != nil {
		return fmt.Errorf("persist consent: %w", err)
	}
	fmt.Fprintf(out, "saved consent to %s\n", cfgPath)
	return nil
}

// stdinIsTTY reports whether os.Stdin is connected to a terminal
// (vs piped, redirected, or being driven by a parent agent CLI).
// Used by EnsureConsent to give a clearer hint instead of blocking
// on a prompt that nothing can answer.
func stdinIsTTY() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// providerLabels maps agent names to human-readable provider names
// for the consent prompt.
func providerLabels(agents []AgentName) []string {
	labels := map[AgentName]string{
		AgentClaude: "Anthropic (claude)",
		AgentCodex:  "OpenAI (codex)",
		AgentGemini: "Google (gemini)",
	}
	var out []string
	seen := map[string]bool{}
	for _, a := range agents {
		l, ok := labels[a]
		if !ok || seen[l] {
			continue
		}
		seen[l] = true
		out = append(out, l)
	}
	return out
}
