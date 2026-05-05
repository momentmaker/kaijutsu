package swarm

import (
	"os"
	"path/filepath"
)

// LoadPresetWithSkillOverrides starts from the built-in preset and
// overlays per-agent + synthesizer + debate prompts loaded from the
// installed pr-review skill's prompts/ directory. Falls back to the
// built-in template silently when a file isn't present.
//
// Skill lookup order:
//  1. <project>/.claude/skills/pr-review/prompts/<name>.md
//  2. <project>/.agents/skills/pr-review/prompts/<name>.md
//  3. ~/.claude/skills/pr-review/prompts/<name>.md
//  4. ~/.agents/skills/pr-review/prompts/<name>.md
func LoadPresetWithSkillOverrides(projectRoot, presetName string) (*Preset, error) {
	base, err := PresetFor(presetName)
	if err != nil {
		return nil, err
	}
	if presetName != "pr-review" {
		return base, nil
	}
	dirs := skillPromptDirs(projectRoot, presetName)
	out := *base // shallow copy
	out.PerAgent = make(map[AgentName]string, len(base.PerAgent))
	for k, v := range base.PerAgent {
		out.PerAgent[k] = v
	}
	for _, name := range AllAgents {
		if override := readPrompt(dirs, string(name)+".md"); override != "" {
			out.PerAgent[name] = override
		}
	}
	if s := readPrompt(dirs, "synthesizer.md"); s != "" {
		out.Synthesizer = s
	}
	if d := readPrompt(dirs, "debate.md"); d != "" {
		out.Debate = d
	}
	return &out, nil
}

func skillPromptDirs(projectRoot, presetName string) []string {
	home, _ := os.UserHomeDir()
	candidates := []string{
		filepath.Join(projectRoot, ".claude", "skills", presetName, "prompts"),
		filepath.Join(projectRoot, ".agents", "skills", presetName, "prompts"),
	}
	if home != "" {
		candidates = append(candidates,
			filepath.Join(home, ".claude", "skills", presetName, "prompts"),
			filepath.Join(home, ".agents", "skills", presetName, "prompts"),
		)
	}
	return candidates
}

func readPrompt(dirs []string, file string) string {
	for _, d := range dirs {
		full := filepath.Join(d, file)
		data, err := os.ReadFile(full)
		if err != nil {
			continue
		}
		return string(data)
	}
	return ""
}
