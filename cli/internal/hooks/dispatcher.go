package hooks

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/momentmaker/kaijutsu/cli/internal/skill"
)

// InstallForSkill installs every hook in sk.Hooks into every active
// agent's settings file under installRoot. Hooks for events the agent
// doesn't support are skipped with a stderr note.
//
// The resolveScriptPath function maps a skill-relative script path
// (e.g. "hooks/dcg.sh") to the absolute path on disk where install.go
// has just placed the script. Different agents see different paths
// because Claude installs to .claude/skills/<name>/ and Codex/Gemini
// install to .agents/skills/<name>/.
func InstallForSkill(stderr io.Writer, installRoot string, agents []string, sk *skill.Skill) error {
	if len(sk.Hooks) == 0 {
		return nil
	}
	if !sk.Permissions.Hooks {
		return Errorf("skill %q ships hooks but does not declare permissions.hooks: true", sk.Name)
	}
	for _, ag := range agents {
		var (
			err    error
			agent  Agent
			script func(skillName, hookScript string) string
		)
		switch ag {
		case "claude":
			agent = Claude
			script = func(name, h string) string {
				return filepath.Join(installRoot, ".claude", "skills", name, h)
			}
		case "codex", "gemini":
			// Both codex + gemini share .agents/skills/<name>/
			if ag == "codex" {
				agent = Codex
			} else {
				agent = Gemini
			}
			script = func(name, h string) string {
				return filepath.Join(installRoot, ".agents", "skills", name, h)
			}
		default:
			continue
		}
		entries, skipped := planForAgent(sk, agent, script)
		for _, s := range skipped {
			fmt.Fprintf(stderr, "note: skill %s hook %s: event %q has no equivalent on %s — skipping.\n",
				sk.Name, s.HookID, s.Event, ag)
		}
		switch agent {
		case Claude:
			err = InstallClaude(installRoot, entries)
		case Codex:
			err = InstallCodex(installRoot, entries)
		case Gemini:
			err = InstallGemini(installRoot, entries)
		}
		if err != nil {
			return Errorf("install hooks for %s on %s: %w", sk.Name, ag, err)
		}
	}
	return nil
}

// RemoveForSkill removes every kaijutsu-tagged hook entry for the named
// skill from every active agent's settings file under installRoot.
func RemoveForSkill(installRoot string, agents []string, skillName string) error {
	for _, ag := range agents {
		var err error
		switch ag {
		case "claude":
			err = RemoveClaude(installRoot, skillName)
		case "codex":
			err = RemoveCodex(installRoot, skillName)
		case "gemini":
			err = RemoveGemini(installRoot, skillName)
		}
		if err != nil {
			return Errorf("remove hooks for %s on %s: %w", skillName, ag, err)
		}
	}
	return nil
}

func planForAgent(sk *skill.Skill, agent Agent, scriptPath func(skillName, hookScript string) string) ([]Entry, []Skipped) {
	var entries []Entry
	var skipped []Skipped
	for _, h := range sk.Hooks {
		nativeEvent, ok := Translate(h.Event, agent)
		if !ok || nativeEvent == "" {
			skipped = append(skipped, Skipped{
				SkillName: sk.Name,
				HookID:    h.ID,
				Event:     h.Event,
				Reason:    "agent has no equivalent event",
			})
			continue
		}
		entries = append(entries, Entry{
			SkillName:   sk.Name,
			Hook:        h,
			NativeEvent: nativeEvent,
			ScriptPath:  scriptPath(sk.Name, h.Script),
		})
	}
	return entries, skipped
}
