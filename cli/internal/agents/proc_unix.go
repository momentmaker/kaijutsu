//go:build !windows

package agents

import "syscall"

// newProcAttr returns SysProcAttr that detaches the child into its own
// process group. Without this the child shares the parent's TTY/job-
// control state — when the parent is itself an interactive agent CLI
// (Claude Code), spawned subprocess signals + wait races can lock the
// child's stdio. New pgid breaks the coupling.
func newProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Setpgid: true,
	}
}
