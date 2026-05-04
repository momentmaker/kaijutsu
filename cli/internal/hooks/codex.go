package hooks

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

// codexConfigPath returns the absolute path to the Codex config file.
// Codex hooks live in ~/.codex/config.toml (or .codex/config.toml at
// the project root if such a thing is supported in the future). For
// kaijutsu v0.3 we install at <installRoot>/.codex/config.toml.
func codexConfigPath(installRoot string) string {
	return filepath.Join(installRoot, ".codex", "config.toml")
}

// codexHookEntry mirrors one entry inside a [[<EventName>]] block. Codex
// hooks have an outer matcher and an inner hooks array of handlers.
// We map exactly one inner handler per kaijutsu hook.
type codexHookEntry struct {
	Matcher  string             `toml:"matcher,omitempty"`
	Kaijutsu string             `toml:"_kaijutsu,omitempty"`
	Hooks    []codexHookHandler `toml:"hooks"`
}

type codexHookHandler struct {
	Type    string `toml:"type"`
	Command string `toml:"command"`
	Timeout int    `toml:"timeout_sec,omitempty"`
}

// InstallCodex registers all hooks in entries into the Codex config
// file. The [[<EventName>]] tables live under the top-level config
// alongside any other Codex settings; we round-trip through a generic
// map so unrelated keys are preserved verbatim.
func InstallCodex(installRoot string, entries []Entry) error {
	path := codexConfigPath(installRoot)
	cfg, err := loadTOMLConfig(path)
	if err != nil {
		return err
	}

	wantMarkers := map[string]bool{}
	for _, e := range entries {
		wantMarkers[MarkerFor(e.SkillName, e.Hook.ID)] = true
	}

	for _, e := range entries {
		nativeEvent, _ := Translate(e.Hook.Event, Codex)
		if nativeEvent == "" {
			continue
		}
		blocks := tomlGetBlockArray(cfg, nativeEvent)
		blocks = filterOutTOMLMarker(blocks, wantMarkers)
		blocks = append(blocks, map[string]interface{}{
			"matcher":   e.Hook.Matcher,
			"_kaijutsu": MarkerFor(e.SkillName, e.Hook.ID),
			"hooks": []interface{}{
				map[string]interface{}{
					"type":        "command",
					"command":     e.ScriptPath,
					"timeout_sec": TimeoutSeconds(e.Hook),
				},
			},
		})
		cfg[nativeEvent] = blocks
	}
	return writeTOMLConfig(path, cfg)
}

// RemoveCodex drops all kaijutsu-tagged blocks belonging to skillName
// from the Codex config file.
func RemoveCodex(installRoot, skillName string) error {
	path := codexConfigPath(installRoot)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}
	cfg, err := loadTOMLConfig(path)
	if err != nil {
		return err
	}
	prefix := MarkerPrefix + "skill:" + skillName + ":hook:"
	for key, raw := range cfg {
		arr, ok := raw.([]interface{})
		if !ok {
			continue
		}
		// Heuristic: only filter arrays-of-tables that look like hook
		// blocks (have a "hooks" subarray or a "matcher" field).
		if !looksLikeCodexHookArray(arr) {
			continue
		}
		cfg[key] = filterOutTOMLMarkerPrefix(arr, prefix)
	}
	return writeTOMLConfig(path, cfg)
}

func loadTOMLConfig(path string) (map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]interface{}{}, nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return map[string]interface{}{}, nil
	}
	var m map[string]interface{}
	if err := toml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return m, nil
}

func writeTOMLConfig(path string, m map[string]interface{}) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := toml.Marshal(m)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func tomlGetBlockArray(cfg map[string]interface{}, key string) []interface{} {
	if existing, ok := cfg[key].([]interface{}); ok {
		return existing
	}
	return nil
}

func looksLikeCodexHookArray(arr []interface{}) bool {
	for _, item := range arr {
		entry, ok := item.(map[string]interface{})
		if !ok {
			return false
		}
		if _, hasHooks := entry["hooks"]; hasHooks {
			return true
		}
		if _, hasMatcher := entry["matcher"]; hasMatcher {
			return true
		}
	}
	return false
}

func filterOutTOMLMarker(arr []interface{}, markers map[string]bool) []interface{} {
	out := make([]interface{}, 0, len(arr))
	for _, item := range arr {
		entry, ok := item.(map[string]interface{})
		if !ok {
			out = append(out, item)
			continue
		}
		if marker, ok := entry["_kaijutsu"].(string); ok && markers[marker] {
			continue
		}
		out = append(out, entry)
	}
	return out
}

func filterOutTOMLMarkerPrefix(arr []interface{}, prefix string) []interface{} {
	out := make([]interface{}, 0, len(arr))
	for _, item := range arr {
		entry, ok := item.(map[string]interface{})
		if !ok {
			out = append(out, item)
			continue
		}
		if marker, ok := entry["_kaijutsu"].(string); ok && len(marker) >= len(prefix) && marker[:len(prefix)] == prefix {
			continue
		}
		out = append(out, entry)
	}
	return out
}
