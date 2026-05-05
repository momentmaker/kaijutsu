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

// geminiAgent invokes `gemini -p`. For prompts under argSafe bytes
// the entire prompt rides as the -p arg. For larger prompts (real PR
// diffs easily exceed 64KB) we keep a stub instruction in -p and pipe
// the bulk via stdin — Gemini's `-p` doc states stdin is APPENDED to
// the prompt arg in non-interactive mode.
type geminiAgent struct{}

// argSafe is a conservative cap below the smallest ARG_MAX we expect
// to encounter (macOS bash ~256KB; many shells stricter under
// `xargs`-style invocation). Keeping a big margin avoids E2BIG.
const argSafe = 32 * 1024

func (geminiAgent) Name() AgentName { return AgentGemini }
func (geminiAgent) Run(ctx context.Context, prompt string, budget float64) (string, error) {
	if len(prompt) < argSafe {
		return runWithStdin(ctx, "gemini", []string{"-p", prompt}, "")
	}
	stub := "Read the full prompt + DIFF on stdin. Follow the instructions in it exactly. Return ONLY the JSON array described."
	return runWithStdin(ctx, "gemini", []string{"-p", stub}, prompt)
}

// runWithStdin executes name with args and prompt piped to stdin.
// Returns the captured stdout. stderr is captured into the error on
// non-zero exit. Respects ctx cancellation/timeout.
//
// WaitDelay (Go 1.20+) ensures that if the agent's main process exits
// after ctx-cancellation but child processes (e.g., update-checkers)
// keep stdout/stderr pipes open, the goroutine doesn't hang waiting
// for those pipes to drain. After 5 seconds beyond cancellation, Go
// SIGKILLs the process group and returns from Wait(). Without this,
// a hung daemon child holds the FanOut goroutine indefinitely.
func runWithStdin(ctx context.Context, name string, args []string, stdin string) (string, error) {
	c := exec.CommandContext(ctx, name, args...)
	c.WaitDelay = 5 * time.Second
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
