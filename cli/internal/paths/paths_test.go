package paths

import "testing"

func TestAgentSkillsDir(t *testing.T) {
	cases := []struct {
		root, agent, want string
	}{
		{"/proj", "claude", "/proj/.claude/skills"},
		{"/proj", "codex", "/proj/.agents/skills"},
		{"/proj", "gemini", "/proj/.agents/skills"},
		{"/proj", "unknown", ""},
		{"/home/u", "claude", "/home/u/.claude/skills"},
	}
	for _, c := range cases {
		if got := AgentSkillsDir(c.root, c.agent); got != c.want {
			t.Errorf("AgentSkillsDir(%q, %q) = %q; want %q", c.root, c.agent, got, c.want)
		}
	}
}
