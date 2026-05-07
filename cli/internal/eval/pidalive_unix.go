//go:build !windows

package eval

import (
	"os"
	"syscall"
)

func pidAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// Signal 0 = "test if delivery is possible." err == nil → exists;
	// ESRCH → doesn't exist; EPERM → exists but unsignalable (still
	// alive for our purposes).
	err = proc.Signal(syscall.Signal(0))
	if err == nil {
		return true
	}
	if err == syscall.EPERM {
		return true
	}
	return false
}
