package hooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/momentmaker/kaijutsu/cli/internal/skill"
	"github.com/pelletier/go-toml/v2"
)

func TestTranslate(t *testing.T) {
	cases := []struct {
		canonical string
		agent     Agent
		want      string
		ok        bool
	}{
		{"pre-tool-use", Claude, "PreToolUse", true},
		{"pre-tool-use", Codex, "PreToolUse", true},
		{"pre-tool-use", Gemini, "BeforeTool", true},
		{"session-end", Claude, "Stop", true},
		{"session-end", Gemini, "SessionEnd", true},
		{"notification", Codex, "", false},
		{"permission-request", Claude, "", false},
		{"permission-request", Codex, "PermissionRequest", true},
		{"before-agent", Claude, "", false},
		{"before-agent", Gemini, "BeforeAgent", true},
		{"bogus-event", Claude, "", false},
	}
	for _, c := range cases {
		got, ok := Translate(c.canonical, c.agent)
		if ok != c.ok || got != c.want {
			t.Errorf("Translate(%q, %s) = (%q, %v); want (%q, %v)", c.canonical, c.agent, got, ok, c.want, c.ok)
		}
	}
}

func TestClaudeInstallRemoveRoundTrip(t *testing.T) {
	root := t.TempDir()
	canBlock := true
	hook := skill.Hook{
		ID: "block-rm-rf", Event: "pre-tool-use", Matcher: "Bash",
		Script: "hooks/dcg.sh", TimeoutSeconds: 10, CanBlock: &canBlock,
	}
	entries := []Entry{{
		SkillName:   "dcg",
		Hook:        hook,
		NativeEvent: "PreToolUse",
		ScriptPath:  "/path/to/dcg.sh",
	}}
	if err := InstallClaude(root, entries); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(filepath.Join(root, ".claude", "settings.json"))
	var got map[string]interface{}
	json.Unmarshal(body, &got)
	hooks := got["hooks"].(map[string]interface{})
	pre := hooks["PreToolUse"].([]interface{})
	if len(pre) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(pre))
	}
	entry := pre[0].(map[string]interface{})
	if entry["matcher"] != "Bash" {
		t.Errorf("matcher = %v", entry["matcher"])
	}

	// Idempotency: re-install replaces, doesn't duplicate.
	if err := InstallClaude(root, entries); err != nil {
		t.Fatal(err)
	}
	body, _ = os.ReadFile(filepath.Join(root, ".claude", "settings.json"))
	json.Unmarshal(body, &got)
	pre = got["hooks"].(map[string]interface{})["PreToolUse"].([]interface{})
	if len(pre) != 1 {
		t.Fatalf("after re-install: expected 1 entry, got %d", len(pre))
	}

	// Remove drops the entry.
	if err := RemoveClaude(root, "dcg"); err != nil {
		t.Fatal(err)
	}
	body, _ = os.ReadFile(filepath.Join(root, ".claude", "settings.json"))
	json.Unmarshal(body, &got)
	pre = got["hooks"].(map[string]interface{})["PreToolUse"].([]interface{})
	if len(pre) != 0 {
		t.Errorf("after remove: expected 0 entries, got %d", len(pre))
	}
}

func TestClaudePreservesUnrelatedSettings(t *testing.T) {
	root := t.TempDir()
	settingsPath := filepath.Join(root, ".claude", "settings.json")
	os.MkdirAll(filepath.Dir(settingsPath), 0755)
	os.WriteFile(settingsPath, []byte(`{
  "theme": "dark",
  "hooks": {
    "PreToolUse": [
      {"matcher": "Edit", "hooks": [{"type": "command", "command": "/users/own/script.sh"}]}
    ]
  }
}`), 0644)

	entries := []Entry{{
		SkillName: "dcg", Hook: skill.Hook{ID: "block", Event: "pre-tool-use", Matcher: "Bash", Script: "hooks/dcg.sh"},
		NativeEvent: "PreToolUse", ScriptPath: "/abs/dcg.sh",
	}}
	if err := InstallClaude(root, entries); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(settingsPath)
	var got map[string]interface{}
	json.Unmarshal(body, &got)
	if got["theme"] != "dark" {
		t.Errorf("theme lost: %v", got["theme"])
	}
	pre := got["hooks"].(map[string]interface{})["PreToolUse"].([]interface{})
	if len(pre) != 2 {
		t.Errorf("expected user's hook + ours = 2 entries, got %d", len(pre))
	}

	// Remove only our entry; user's stays.
	if err := RemoveClaude(root, "dcg"); err != nil {
		t.Fatal(err)
	}
	body, _ = os.ReadFile(settingsPath)
	json.Unmarshal(body, &got)
	pre = got["hooks"].(map[string]interface{})["PreToolUse"].([]interface{})
	if len(pre) != 1 {
		t.Errorf("expected user's hook to survive, got %d entries", len(pre))
	}
	user := pre[0].(map[string]interface{})
	if user["matcher"] != "Edit" {
		t.Errorf("user's hook munged: %v", user)
	}
}

func TestCodexTOMLRoundTrip(t *testing.T) {
	root := t.TempDir()
	entries := []Entry{{
		SkillName: "dcg",
		Hook:      skill.Hook{ID: "block-rm", Event: "pre-tool-use", Matcher: "tool_name == 'shell_command'", Script: "hooks/dcg.sh", TimeoutSeconds: 5},
		NativeEvent: "PreToolUse",
		ScriptPath:  "/abs/dcg.sh",
	}}
	if err := InstallCodex(root, entries); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(filepath.Join(root, ".codex", "config.toml"))
	var got map[string]interface{}
	if err := toml.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	pre, ok := got["PreToolUse"].([]interface{})
	if !ok || len(pre) != 1 {
		t.Fatalf("expected 1 PreToolUse block, got %v", got["PreToolUse"])
	}
	if err := RemoveCodex(root, "dcg"); err != nil {
		t.Fatal(err)
	}
	body, _ = os.ReadFile(filepath.Join(root, ".codex", "config.toml"))
	toml.Unmarshal(body, &got)
	pre, _ = got["PreToolUse"].([]interface{})
	if len(pre) != 0 {
		t.Errorf("after remove: expected empty PreToolUse, got %d entries", len(pre))
	}
}

func TestGeminiRoundTripWithMillisecondTimeout(t *testing.T) {
	root := t.TempDir()
	entries := []Entry{{
		SkillName: "dcg",
		Hook:      skill.Hook{ID: "block", Event: "pre-tool-use", Matcher: "Bash", Script: "hooks/dcg.sh", TimeoutSeconds: 5},
		NativeEvent: "BeforeTool",
		ScriptPath:  "/abs/dcg.sh",
	}}
	if err := InstallGemini(root, entries); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(filepath.Join(root, ".gemini", "settings.json"))
	var got map[string]interface{}
	json.Unmarshal(body, &got)
	before := got["hooks"].(map[string]interface{})["BeforeTool"].([]interface{})
	if len(before) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(before))
	}
	entry := before[0].(map[string]interface{})
	// Gemini timeout is in ms — 5s -> 5000ms.
	if entry["timeout"].(float64) != 5000 {
		t.Errorf("expected timeout 5000ms, got %v", entry["timeout"])
	}
}
