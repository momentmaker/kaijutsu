package hooks

import (
	"os"
	"path/filepath"
	"strings"
)

// antigravitySettingsPath returns the absolute path to the Antigravity settings
// file at the given install root.
func antigravitySettingsPath(installRoot string) string {
	return filepath.Join(installRoot, ".gemini/antigravity-cli", "settings.json")
}

// InstallAntigravity registers all hooks in entries into the Antigravity settings
// file at installRoot/.gemini/antigravity-cli/settings.json. Idempotent: existing
// kaijutsu-tagged entries with the same skill+id are replaced.
//
// Antigravity's hook entries are flat (not nested under a matcher wrapper)
// and use millisecond timeouts.
func InstallAntigravity(installRoot string, entries []Entry) error {
	path := antigravitySettingsPath(installRoot)
	settings, err := loadJSONSettings(path)
	if err != nil {
		return err
	}
	hooks := getOrCreateMap(settings, "hooks")

	wantMarkers := map[string]bool{}
	for _, e := range entries {
		wantMarkers[MarkerFor(e.SkillName, e.Hook.ID)] = true
	}

	for _, e := range entries {
		nativeEvent, _ := Translate(e.Hook.Event, Antigravity)
		if nativeEvent == "" {
			continue
		}
		eventArr := getOrCreateArray(hooks, nativeEvent)
		eventArr = filterOutFlatMarker(eventArr, wantMarkers)
		entry := map[string]interface{}{
			"name":      "kaijutsu:" + e.SkillName + ":" + e.Hook.ID,
			"type":      "command",
			"command":   e.ScriptPath,
			"timeout":   TimeoutSeconds(e.Hook) * 1000, // Antigravity uses ms
			"_kaijutsu": MarkerFor(e.SkillName, e.Hook.ID),
		}
		// Pass through matcher when supported (Antigravity's BeforeTool etc
		// accept a matcher field for tool-name filtering).
		if e.Hook.Matcher != "" {
			entry["matcher"] = e.Hook.Matcher
		}
		eventArr = append(eventArr, entry)
		hooks[nativeEvent] = eventArr
	}
	return writeJSONSettings(path, settings)
}

// RemoveAntigravity drops all kaijutsu-tagged hook entries belonging to
// skillName from the Antigravity settings file.
func RemoveAntigravity(installRoot, skillName string) error {
	path := antigravitySettingsPath(installRoot)
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
		hooks[event] = filterOutFlatMarkerPrefix(arr, prefix)
	}
	return writeJSONSettings(path, settings)
}

// filterOutFlatMarker drops entries whose top-level _kaijutsu marker
// matches one of the given markers. Used for Antigravity's flat entry shape.
func filterOutFlatMarker(arr []interface{}, markers map[string]bool) []interface{} {
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

func filterOutFlatMarkerPrefix(arr []interface{}, prefix string) []interface{} {
	out := make([]interface{}, 0, len(arr))
	for _, item := range arr {
		entry, ok := item.(map[string]interface{})
		if !ok {
			out = append(out, item)
			continue
		}
		if marker, ok := entry["_kaijutsu"].(string); ok && strings.HasPrefix(marker, prefix) {
			continue
		}
		out = append(out, entry)
	}
	return out
}
