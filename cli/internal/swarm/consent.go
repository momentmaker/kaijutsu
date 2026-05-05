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

// configPath is the canonical location relative to the project root.
const configPath = ".kaijutsu/pr-review.yaml"

// LoadConfig returns the parsed config or a zero-value Config if the
// file does not exist. A malformed file is a hard error so the user
// notices the problem before sending diffs to remote models.
func LoadConfig(projectRoot string) (*Config, error) {
	full := filepath.Join(projectRoot, configPath)
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
func SaveConsent(projectRoot string) error {
	full := filepath.Join(projectRoot, configPath)
	c, err := LoadConfig(projectRoot)
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

// EnsureConsent is the runtime check: if the project root's config
// already has allow-multi-model: true, returns nil. Otherwise prompts
// the user, persists the answer if they say yes, and returns an error
// if they decline.
func EnsureConsent(projectRoot string, in io.Reader, out io.Writer, agents []AgentName, nonInteractive bool) error {
	c, err := LoadConfig(projectRoot)
	if err != nil {
		return err
	}
	if c.AllowMultiModel {
		return nil
	}
	if nonInteractive {
		return errors.New("multi-model consent not granted. Add `allow-multi-model: true` to .kaijutsu/pr-review.yaml or re-run interactively to be prompted")
	}
	providers := providerLabels(agents)
	fmt.Fprintf(out, "\nThis run will send PR diff to %d model provider(s):\n", len(providers))
	for _, p := range providers {
		fmt.Fprintf(out, "  - %s\n", p)
	}
	fmt.Fprintf(out, "Per-repo consent is required. Persist `allow-multi-model: true` to %s? [y/N]: ", configPath)
	r := bufio.NewReader(in)
	line, _ := r.ReadString('\n')
	line = strings.TrimSpace(strings.ToLower(line))
	if line != "y" && line != "yes" {
		return errors.New("aborted: multi-model consent declined")
	}
	if err := SaveConsent(projectRoot); err != nil {
		return fmt.Errorf("persist consent: %w", err)
	}
	fmt.Fprintf(out, "saved consent to %s\n", configPath)
	return nil
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
