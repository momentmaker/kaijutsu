// Package hooks installs and removes per-agent hook entries declared in
// a skill's skill.yaml. The kaijutsu canonical event vocabulary is
// translated per agent; events without a per-agent equivalent are
// skipped with a warning rather than failing the install.
package hooks

import (
	"fmt"

	"github.com/momentmaker/kaijutsu/cli/internal/skill"
)

// Marker prefix used in the agent-native hook entry to identify which
// kaijutsu skill installed it. Removal scans for this marker.
const MarkerPrefix = "kaijutsu:"

// MarkerFor builds the per-skill / per-hook marker string.
//
//	skill:<name>:hook:<id>
func MarkerFor(skillName, hookID string) string {
	return MarkerPrefix + "skill:" + skillName + ":hook:" + hookID
}

// Agent identifies which agent's hook system to write to.
type Agent string

const (
	Claude Agent = "claude"
	Codex  Agent = "codex"
	Gemini Agent = "gemini"
)

// Plan represents the per-skill hook-install plan resolved against a
// single agent. Skipped entries are events the agent doesn't support.
type Plan struct {
	Agent       Agent
	SettingsPath string  // absolute path to the agent settings file we'll mutate
	Entries     []Entry // hooks that will install
	Skipped     []Skipped
}

// Entry is one hook-to-install mapping after canonical-event translation.
type Entry struct {
	SkillName    string
	Hook         skill.Hook
	NativeEvent  string // PreToolUse / BeforeTool / etc — agent-native name
	ScriptPath   string // resolved absolute path to the hook script on disk
}

// Skipped records a hook that couldn't be installed for this agent.
type Skipped struct {
	SkillName string
	HookID    string
	Event     string
	Reason    string
}

// EventTable maps the kaijutsu canonical event name to the agent-native
// event name. An empty string means "this agent has no equivalent".
var EventTable = map[string]map[Agent]string{
	"pre-tool-use":          {Claude: "PreToolUse", Codex: "PreToolUse", Gemini: "BeforeTool"},
	"post-tool-use":         {Claude: "PostToolUse", Codex: "PostToolUse", Gemini: "AfterTool"},
	"session-start":         {Claude: "SessionStart", Codex: "SessionStart", Gemini: "SessionStart"},
	"session-end":           {Claude: "Stop", Codex: "Stop", Gemini: "SessionEnd"},
	"notification":          {Claude: "Notification", Codex: "", Gemini: "Notification"},
	"user-prompt-submit":    {Claude: "UserPromptSubmit", Codex: "UserPromptSubmit", Gemini: ""},
	"pre-compact":           {Claude: "PreCompact", Codex: "", Gemini: "PreCompress"},
	"permission-request":    {Claude: "", Codex: "PermissionRequest", Gemini: ""},
	"before-agent":          {Claude: "", Codex: "", Gemini: "BeforeAgent"},
	"after-agent":           {Claude: "", Codex: "", Gemini: "AfterAgent"},
	"before-model":          {Claude: "", Codex: "", Gemini: "BeforeModel"},
	"after-model":           {Claude: "", Codex: "", Gemini: "AfterModel"},
	"before-tool-selection": {Claude: "", Codex: "", Gemini: "BeforeToolSelection"},
}

// Translate maps a canonical event to the agent-native event name, or
// returns ("", false) if the agent has no equivalent.
func Translate(event string, agent Agent) (string, bool) {
	row, ok := EventTable[event]
	if !ok {
		return "", false
	}
	native, ok := row[agent]
	if !ok || native == "" {
		return "", false
	}
	return native, true
}

// DefaultTimeoutSeconds is applied when the hook entry omits timeout_seconds.
const DefaultTimeoutSeconds = 30

// CanBlock reports whether a hook should default to blocking-capable
// (pre-* events) when the author hasn't explicitly set can_block.
func CanBlock(h skill.Hook) bool {
	if h.CanBlock != nil {
		return *h.CanBlock
	}
	switch h.Event {
	case "pre-tool-use", "user-prompt-submit", "permission-request", "before-tool-selection", "before-model", "before-agent":
		return true
	}
	return false
}

// TimeoutSeconds returns the configured timeout or DefaultTimeoutSeconds.
func TimeoutSeconds(h skill.Hook) int {
	if h.TimeoutSeconds <= 0 {
		return DefaultTimeoutSeconds
	}
	return h.TimeoutSeconds
}

// Errorf wraps fmt.Errorf with a hooks-package prefix.
func Errorf(format string, args ...any) error {
	return fmt.Errorf("hooks: "+format, args...)
}
