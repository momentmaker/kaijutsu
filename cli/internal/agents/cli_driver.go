package agents

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
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
	return runCLI(ctx, opts, "claude", args, prompt)
}

// codexDriver invokes `codex -p` with stdin-piped prompt.
type codexDriver struct{}

func (codexDriver) Name() string       { return "codex" }
func (codexDriver) Driver() DriverKind { return DriverCLI }

func (codexDriver) Invoke(ctx context.Context, prompt string, opts InvokeOpts) (Result, error) {
	return runCLI(ctx, opts, "codex", []string{"-p"}, prompt)
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
		return runCLI(ctx, opts, "gemini", append(base, prompt), "")
	}
	stub := "Read the full prompt + DIFF on stdin. Follow the instructions in it exactly. Return ONLY the JSON array described."
	return runCLI(ctx, opts, "gemini", append(base, stub), prompt)
}

// runCLI executes name with args and prompt piped to stdin. Returns
// captured stdout. stderr is captured into the error on non-zero exit.
// Respects ctx cancellation/timeout AND opts.Timeout (the smaller of
// the two wins).
//
// WaitDelay (Go 1.20+) ensures that if the agent's main process exits
// after ctx-cancellation but child processes (e.g., update-checkers)
// keep stdout/stderr pipes open, the goroutine doesn't hang waiting
// for those pipes to drain. After 5 seconds beyond cancellation, Go
// SIGKILLs the process group and returns from Wait().
func runCLI(ctx context.Context, opts InvokeOpts, name string, args []string, stdin string) (Result, error) {
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	// Build child env: strip nested-agent markers so the spawned CLI
	// doesn't take a different code path when it detects it's running
	// inside another agent's session. Empirically this is the
	// difference between "claude/gemini works from inside Claude Code"
	// and "errors with no obvious cause" — both CLIs have logic that
	// refuses or behaves differently when they see CLAUDECODE=1 etc.
	parentEnv := stripNestedAgentEnv(os.Environ())
	if len(opts.ExtraEnv) > 0 {
		parentEnv = append(parentEnv, formatEnvPairs(opts.ExtraEnv)...)
	}

	// One retry on transient failure (subprocess wait-state races,
	// lockfile contention, brief network blip). Auth/permanent errors
	// fail the same way twice — retry adds at most a few seconds and
	// fixes the flaky cases without masking real problems.
	res, err := runCLIOnce(ctx, name, args, stdin, parentEnv)
	if err != nil && isTransientCLIError(res.Err) {
		time.Sleep(500 * time.Millisecond)
		res2, err2 := runCLIOnce(ctx, name, args, stdin, parentEnv)
		if err2 == nil {
			return res2, nil
		}
	}
	return res, err
}

// runCLIOnce is one attempt at the subprocess spawn — runCLI retries
// on transient errors above.
func runCLIOnce(ctx context.Context, name string, args []string, stdin string, env []string) (Result, error) {
	c := exec.CommandContext(ctx, name, args...)
	c.WaitDelay = 5 * time.Second
	c.Env = env
	// Setpgid: detach into a new process group. Without this the child
	// shares the parent's TTY/job-control state — when the parent is
	// itself a long-lived agent CLI (Claude Code), signals + wait
	// state races can lock the inner CLI's stdio. The new pgid
	// isolates spawned process from parent's TUI.
	c.SysProcAttr = newProcAttr()
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
		return res, errors.New(errMsg)
	}
	return res, nil
}

// stripNestedAgentEnv removes env vars that signal "running inside
// another agent CLI" from a slice of KEY=VALUE entries. The spawned
// CLI sees a clean environment as if it were launched from a fresh
// shell.
func stripNestedAgentEnv(in []string) []string {
	// Strip these prefixes/exact names. Conservative list — only
	// known nested-agent markers that empirically affect behavior.
	stripExact := map[string]struct{}{
		"CLAUDECODE":              {},
		"CLAUDE_CODE_ENTRYPOINT":  {},
		"CLAUDE_CODE_EXECPATH":    {},
		"CLAUDE_CODE_SESSION":     {},
		"CLAUDE_CODE_SESSION_ID":  {},
		"AI_AGENT":                {},
		"CODEX_SESSION":           {},
		"CODEX_SESSION_ID":        {},
		"GEMINI_SESSION":          {},
		"GEMINI_SESSION_ID":       {},
		"JUTSU_NESTED_AGENT":      {},
	}
	stripPrefix := []string{
		"CLAUDE_CODE_",
	}
	out := make([]string, 0, len(in))
	for _, e := range in {
		eq := strings.IndexByte(e, '=')
		if eq <= 0 {
			out = append(out, e)
			continue
		}
		k := e[:eq]
		if _, skip := stripExact[k]; skip {
			continue
		}
		skip := false
		for _, p := range stripPrefix {
			if strings.HasPrefix(k, p) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		out = append(out, e)
	}
	return out
}

// isTransientCLIError heuristically detects retry-worthy errors. Auth
// failures, "command not found", and explicit refusals fail the same
// way twice — only retry the flaky kinds. Empirically: nil-deref
// panics, "context deadline" near boundary, subprocess wait races.
func isTransientCLIError(msg string) bool {
	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "context deadline exceeded"):
		return false // ctx ran out — retry won't help, parent will cancel
	case strings.Contains(lower, "executable file not found"):
		return false // PATH issue — retry won't fix
	case strings.Contains(lower, "permission denied"):
		return false // ACL — retry won't fix
	case strings.Contains(lower, "exit status 1"):
		// Generic exit 1 from claude/gemini under nested-agent
		// conditions — empirically transient.
		return true
	case strings.Contains(lower, "broken pipe"):
		return true
	case strings.Contains(lower, "stream error"):
		return true
	}
	return false
}

func trimErr(s string) string {
	const max = 500
	if len(s) > max {
		return s[:max] + "…"
	}
	return s
}

// formatEnvPairs converts a name→value map to KEY=VALUE entries
// suitable for exec.Cmd.Env. Sorted for deterministic ordering
// (helps debugging + cache-key reproducibility downstream).
func formatEnvPairs(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	for i, k := range out {
		out[i] = k + "=" + m[k]
	}
	return out
}
