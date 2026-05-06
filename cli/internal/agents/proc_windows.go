//go:build windows

package agents

import "syscall"

// newProcAttr is a no-op on Windows. The Unix Setpgid trick has no
// direct equivalent; CREATE_NEW_PROCESS_GROUP could be used but
// changes Ctrl-C semantics. Skip for now — Windows users running
// jutsu swarm from inside Claude Code is not the primary use case.
func newProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{}
}
