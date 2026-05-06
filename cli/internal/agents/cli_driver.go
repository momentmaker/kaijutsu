package agents

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"
)

// argSafe is a conservative cap below the smallest ARG_MAX we expect
// to encounter (macOS bash ~256KB; many shells stricter under
// `xargs`-style invocation). Keeping a big margin avoids E2BIG.
const argSafe = 32 * 1024

// claudeDriver invokes `claude -p` with stdin-piped prompt. Honors
// --max-budget-usd when InvokeOpts.MaxBudgetUSD > 0.
type claudeDriver struct{}

func (claudeDriver) Name() string       { return "claude" }
func (claudeDriver) Driver() DriverKind { return DriverCLI }

func (claudeDriver) Invoke(ctx context.Context, prompt string, opts InvokeOpts) (Result, error) {
	args := []string{"-p", "--output-format", "text"}
	if opts.MaxBudgetUSD > 0 {
		args = append(args, "--max-budget-usd", fmt.Sprintf("%.2f", opts.MaxBudgetUSD))
	}
	return runCLI(ctx, "claude", args, prompt)
}

// codexDriver invokes `codex -p` with stdin-piped prompt.
type codexDriver struct{}

func (codexDriver) Name() string       { return "codex" }
func (codexDriver) Driver() DriverKind { return DriverCLI }

func (codexDriver) Invoke(ctx context.Context, prompt string, opts InvokeOpts) (Result, error) {
	return runCLI(ctx, "codex", []string{"-p"}, prompt)
}

// geminiDriver invokes `gemini --approval-mode plan -p`. For prompts
// under argSafe bytes the entire prompt rides as the -p arg. For
// larger prompts (real PR diffs easily exceed 64KB) we keep a stub
// instruction in -p and pipe the bulk via stdin — Gemini's `-p` doc
// states stdin is APPENDED to the prompt arg in non-interactive mode.
//
// --approval-mode plan: read-only mode. Auto-approves reads, blocks
// writes/shell. Without this, gemini's default approval-mode prompts
// for confirmation on every tool call, blocking on stdin (no tty) and
// ultimately timing out.
type geminiDriver struct{}

func (geminiDriver) Name() string       { return "gemini" }
func (geminiDriver) Driver() DriverKind { return DriverCLI }

func (geminiDriver) Invoke(ctx context.Context, prompt string, opts InvokeOpts) (Result, error) {
	base := []string{"--approval-mode", "plan", "-p"}
	if len(prompt) < argSafe {
		return runCLI(ctx, "gemini", append(base, prompt), "")
	}
	stub := "Read the full prompt + DIFF on stdin. Follow the instructions in it exactly. Return ONLY the JSON array described."
	return runCLI(ctx, "gemini", append(base, stub), prompt)
}

// runCLI executes name with args and prompt piped to stdin. Returns
// captured stdout. stderr is captured into the error on non-zero exit.
// Respects ctx cancellation/timeout.
//
// WaitDelay (Go 1.20+) ensures that if the agent's main process exits
// after ctx-cancellation but child processes (e.g., update-checkers)
// keep stdout/stderr pipes open, the goroutine doesn't hang waiting
// for those pipes to drain. After 5 seconds beyond cancellation, Go
// SIGKILLs the process group and returns from Wait().
func runCLI(ctx context.Context, name string, args []string, stdin string) (Result, error) {
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
	res := Result{
		Raw:         stdout.String(),
		Duration:    time.Since(start),
		Driver:      DriverCLI,
		CacheStatus: CacheUnsupported,
	}
	if err != nil {
		errMsg := fmt.Sprintf("%s exited %v after %s: %s", name, err, res.Duration.Round(time.Millisecond), trimErr(stderr.String()))
		res.Err = errMsg
		return res, fmt.Errorf("%s", errMsg)
	}
	return res, nil
}

func trimErr(s string) string {
	const max = 500
	if len(s) > max {
		return s[:max] + "…"
	}
	return s
}
