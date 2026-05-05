package swarm

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"
)

// Agent is a single backing CLI we can invoke non-interactively.
type Agent interface {
	Name() AgentName
	// Run sends prompt to the agent and returns its raw stdout. The
	// caller is responsible for parsing — implementations should not
	// post-process beyond capturing the bytes.
	Run(ctx context.Context, prompt string, maxBudgetUSD float64) (raw string, err error)
}

// AgentFor returns the concrete Agent implementation for a name.
func AgentFor(name AgentName) Agent {
	switch name {
	case AgentClaude:
		return &claudeAgent{}
	case AgentCodex:
		return &codexAgent{}
	case AgentGemini:
		return &geminiAgent{}
	}
	return nil
}

// claudeAgent invokes `claude -p` with stdin-piped prompt. Honors
// --max-budget-usd when budget > 0.
type claudeAgent struct{}

func (claudeAgent) Name() AgentName { return AgentClaude }
func (claudeAgent) Run(ctx context.Context, prompt string, budget float64) (string, error) {
	args := []string{"-p", "--output-format", "text"}
	if budget > 0 {
		args = append(args, "--max-budget-usd", fmt.Sprintf("%.2f", budget))
	}
	return runWithStdin(ctx, "claude", args, prompt)
}

// codexAgent invokes `codex -p` with stdin-piped prompt.
type codexAgent struct{}

func (codexAgent) Name() AgentName { return AgentCodex }
func (codexAgent) Run(ctx context.Context, prompt string, budget float64) (string, error) {
	return runWithStdin(ctx, "codex", []string{"-p"}, prompt)
}

// geminiAgent passes prompt as -p arg + diff context on stdin. Stage 1
// concatenates everything into the prompt arg and ignores stdin to
// keep the contract identical across agents.
type geminiAgent struct{}

func (geminiAgent) Name() AgentName { return AgentGemini }
func (geminiAgent) Run(ctx context.Context, prompt string, budget float64) (string, error) {
	return runWithStdin(ctx, "gemini", []string{"-p", prompt}, "")
}

// runWithStdin executes name with args and prompt piped to stdin.
// Returns the captured stdout. stderr is captured into the error on
// non-zero exit. Respects ctx cancellation/timeout.
func runWithStdin(ctx context.Context, name string, args []string, stdin string) (string, error) {
	c := exec.CommandContext(ctx, name, args...)
	if stdin != "" {
		c.Stdin = bytes.NewBufferString(stdin)
	}
	var stdout, stderr bytes.Buffer
	c.Stdout = &stdout
	c.Stderr = &stderr
	start := time.Now()
	err := c.Run()
	if err != nil {
		return stdout.String(), fmt.Errorf("%s exited %v after %s: %s", name, err, time.Since(start).Round(time.Millisecond), trimErr(stderr.String()))
	}
	return stdout.String(), nil
}

func trimErr(s string) string {
	// keep things bounded so swarm output stays readable
	const max = 500
	if len(s) > max {
		return s[:max] + "…"
	}
	return s
}
