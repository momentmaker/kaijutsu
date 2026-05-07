//go:build windows

package eval

// pidAlive on Windows: conservative — always reports the PID as
// alive, forcing the lock-clear decision back to the mtime gate
// (2h threshold). Windows process introspection via os.FindProcess
// returns a Process for ANY PID, including dead ones, and there's
// no syscall.Signal(0) equivalent. The 2h mtime cap already covers
// the realistic crash-recovery scenarios (a crashed runner's lock
// becomes stale long before the workspace is reused), so this is
// acceptable degradation for v0.10 Windows builds.
//
// v0.10.x candidate: use OpenProcess + GetExitCodeProcess via
// golang.org/x/sys/windows for precise liveness when the lock
// throughput on Windows justifies the dependency.
func pidAlive(pid int) bool {
	return true
}
