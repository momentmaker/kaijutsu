package hooks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// claudeSettingsPath returns the absolute path to the Claude settings
// file at the given install root. installRoot is "" / homeDir for
// global, project-cwd for project-scope.
func claudeSettingsPath(installRoot string) string {
	return filepath.Join(installRoot, ".claude", "settings.json")
}

// InstallClaude registers all hooks in entries into the Claude
// settings file at installRoot/.claude/settings.json. Idempotent:
// existing kaijutsu-tagged entries with the same skill+id are
// replaced; non-kaijutsu entries are preserved.
func InstallClaude(installRoot string, entries []Entry) error {
	path := claudeSettingsPath(installRoot)
	settings, err := loadJSONSettings(path)
	if err != nil {
		return err
	}
	hooks := getOrCreateMap(settings, "hooks")

	// Collect markers we're about to install so we can remove prior
	// entries with the same marker first (re-install path).
	wantMarkers := map[string]bool{}
	for _, e := range entries {
		wantMarkers[MarkerFor(e.SkillName, e.Hook.ID)] = true
	}

	for _, e := range entries {
		nativeEvent, _ := Translate(e.Hook.Event, Claude)
		if nativeEvent == "" {
			continue
		}
		eventArr := getOrCreateArray(hooks, nativeEvent)
		eventArr = filterOutMarker(eventArr, wantMarkers)
		entry := map[string]interface{}{
			"matcher": e.Hook.Matcher,
			"hooks": []interface{}{
				map[string]interface{}{
					"type":       "command",
					"command":    e.ScriptPath,
					"timeout":    TimeoutSeconds(e.Hook),
					"_kaijutsu":  MarkerFor(e.SkillName, e.Hook.ID),
				},
			},
		}
		eventArr = append(eventArr, entry)
		hooks[nativeEvent] = eventArr
	}
	return writeJSONSettings(path, settings)
}

// RemoveClaude drops all kaijutsu-tagged hook entries belonging to
// skillName from the Claude settings file at installRoot.
func RemoveClaude(installRoot, skillName string) error {
	path := claudeSettingsPath(installRoot)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}
	settings, err := loadJSONSettings(path)
	if err != nil {
		return err
	}
	hooks, ok := settings["hooks"].(map[string]interface{})
	if !ok {
		return nil
	}
	prefix := MarkerPrefix + "skill:" + skillName + ":hook:"
	for event, raw := range hooks {
		arr, ok := raw.([]interface{})
		if !ok {
			continue
		}
		hooks[event] = filterOutPrefix(arr, prefix)
	}
	return writeJSONSettings(path, settings)
}

// loadJSONSettings reads a JSON settings file or returns an empty map
// if the file doesn't exist.
func loadJSONSettings(path string) (map[string]interface{}, error) {
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
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if m == nil {
		m = map[string]interface{}{}
	}
	return m, nil
}

// writeJSONSettings writes m to path with 2-space indent and trailing newline.
// File mode is 0600 — agent settings files often hold OAuth tokens and API
// keys; even though we're only writing hook entries, we shouldn't tighten
// or loosen the perm boundary asymmetrically.
func writeJSONSettings(path string, m map[string]interface{}) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0600)
}

func getOrCreateMap(parent map[string]interface{}, key string) map[string]interface{} {
	if existing, ok := parent[key].(map[string]interface{}); ok {
		return existing
	}
	m := map[string]interface{}{}
	parent[key] = m
	return m
}

func getOrCreateArray(parent map[string]interface{}, key string) []interface{} {
	if existing, ok := parent[key].([]interface{}); ok {
		return existing
	}
	return nil
}

// filterOutMarker drops outer-entries whose inner hooks include any of
// the given markers. Used during install for idempotent replace.
func filterOutMarker(arr []interface{}, markers map[string]bool) []interface{} {
	out := make([]interface{}, 0, len(arr))
	for _, item := range arr {
		entry, ok := item.(map[string]interface{})
		if !ok {
			out = append(out, item)
			continue
		}
		if entryHasAnyMarker(entry, markers) {
			continue
		}
		out = append(out, item)
	}
	return out
}

// filterOutPrefix drops outer-entries whose inner hooks include any
// marker starting with the given prefix. Used during remove.
func filterOutPrefix(arr []interface{}, prefix string) []interface{} {
	out := make([]interface{}, 0, len(arr))
	for _, item := range arr {
		entry, ok := item.(map[string]interface{})
		if !ok {
			out = append(out, item)
			continue
		}
		if entryHasMarkerPrefix(entry, prefix) {
			continue
		}
		out = append(out, item)
	}
	return out
}

func entryHasAnyMarker(entry map[string]interface{}, markers map[string]bool) bool {
	innerArr, ok := entry["hooks"].([]interface{})
	if !ok {
		return false
	}
	for _, h := range innerArr {
		inner, ok := h.(map[string]interface{})
		if !ok {
			continue
		}
		if marker, ok := inner["_kaijutsu"].(string); ok && markers[marker] {
			return true
		}
	}
	return false
}

func entryHasMarkerPrefix(entry map[string]interface{}, prefix string) bool {
	innerArr, ok := entry["hooks"].([]interface{})
	if !ok {
		return false
	}
	for _, h := range innerArr {
		inner, ok := h.(map[string]interface{})
		if !ok {
			continue
		}
		if marker, ok := inner["_kaijutsu"].(string); ok && strings.HasPrefix(marker, prefix) {
			return true
		}
	}
	return false
}
