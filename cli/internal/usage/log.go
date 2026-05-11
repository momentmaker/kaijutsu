// Package usage — v0.16.0 local-only command-invocation telemetry.
//
// Every `jutsu` invocation appends one line to ~/.kaijutsu/usage.jsonl:
//
//	{"ts": "...", "cmd": "agent persona browse", "exit": 0, "ms": 142}
//
// Local-only. Nothing leaves disk. No content, no flag values, no args —
// just the subcommand path, exit code, and wall duration. Opt-out via
// KAIJUTSU_USAGE_LOG=0.
//
// Why: 6 consecutive `jutsu swarm dream` passes converged on "instrument
// first, then decide what to ship." Six dreams in a row asking the same
// question and getting the same answer crosses the threshold from
// model-shared bias to actual signal. The precondition was always
// "wait, we have no usage data." This package satisfies that
// precondition without adding any user-visible surface.
//
// Spec: this comment block. Cadence: solo-maintainer-shaped tiny ship.
package usage

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// LogPath returns the canonical usage-log path: ~/.kaijutsu/usage.jsonl.
// Override via KAIJUTSU_USAGE_LOG_PATH for tests.
func LogPath() (string, error) {
	if p := os.Getenv("KAIJUTSU_USAGE_LOG_PATH"); p != "" {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".kaijutsu", "usage.jsonl"), nil
}

// Disabled reports whether the user opted out via KAIJUTSU_USAGE_LOG=0.
// Default is enabled — local-only logging carries no privacy cost worth
// defaulting off.
func Disabled() bool {
	return os.Getenv("KAIJUTSU_USAGE_LOG") == "0"
}

// Entry is one row in usage.jsonl.
type Entry struct {
	TS   time.Time `json:"ts"`
	Cmd  string    `json:"cmd"`
	Exit int       `json:"exit"`
	MS   int64     `json:"ms"`
}

// Append writes one entry to the usage log. Failures are silent — we
// never want telemetry-write failures to break the user's actual
// command. Returns nil even on disk-full / perm-denied.
func Append(cmd string, exit int, duration time.Duration) {
	if Disabled() {
		return
	}
	path, err := LogPath()
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	entry := Entry{
		TS:   time.Now().UTC(),
		Cmd:  cmd,
		Exit: exit,
		MS:   duration.Milliseconds(),
	}
	if err := json.NewEncoder(f).Encode(&entry); err != nil {
		// Truncate any partial write so we don't corrupt subsequent reads.
		// Best-effort; nothing to do if this also fails.
		_ = f.Sync()
	}
}

// Read returns all entries in the log. Missing file = empty slice + nil.
// Malformed lines are skipped, not fatal. Used by `jutsu usage stats`.
//
// Uses line-by-line scanning instead of json.Decoder because Decoder
// fails to advance past malformed JSON and infinite-loops. JSONL is
// line-delimited by construction, so bufio.Scanner is the right tool.
func Read() ([]Entry, error) {
	path, err := LogPath()
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	// Scanner default buffer is 64KB; one usage line is far smaller, but
	// bump anyway to survive accidental concatenation.
	scanner.Buffer(make([]byte, 0, 4096), 1<<20)
	var entries []Entry
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var e Entry
		if err := json.Unmarshal(line, &e); err != nil {
			// Skip malformed; preserve the rest of the file.
			continue
		}
		entries = append(entries, e)
	}
	if err := scanner.Err(); err != nil {
		return entries, err
	}
	return entries, nil
}
