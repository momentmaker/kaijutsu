package usage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAppend_NoOpWhenDisabled(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("KAIJUTSU_USAGE_LOG_PATH", filepath.Join(tmp, "u.jsonl"))
	t.Setenv("KAIJUTSU_USAGE_LOG", "0")
	Append("jutsu agent list", 0, 12*time.Millisecond)
	entries, err := Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries when disabled; got %d", len(entries))
	}
}

func TestAppend_WritesJSONL(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("KAIJUTSU_USAGE_LOG_PATH", filepath.Join(tmp, "u.jsonl"))
	t.Setenv("KAIJUTSU_USAGE_LOG", "1") // explicit enable
	Append("jutsu finding stats", 0, 142*time.Millisecond)
	Append("jutsu finding stats", 2, 5*time.Millisecond)
	Append("jutsu install no-such-skill", 3, 99*time.Millisecond)

	entries, err := Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries; got %d", len(entries))
	}
	if entries[0].Cmd != "jutsu finding stats" || entries[0].Exit != 0 || entries[0].MS != 142 {
		t.Errorf("entry[0] mismatch: %+v", entries[0])
	}
	if entries[2].Exit != 3 {
		t.Errorf("entry[2] exit code mismatch: got %d, want 3", entries[2].Exit)
	}
}

func TestRead_MissingFileReturnsEmpty(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("KAIJUTSU_USAGE_LOG_PATH", filepath.Join(tmp, "does-not-exist.jsonl"))
	entries, err := Read()
	if err != nil {
		t.Fatal(err)
	}
	if entries != nil {
		t.Errorf("expected nil slice for missing file; got %d entries", len(entries))
	}
}

func TestRead_SkipsMalformedLines(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("KAIJUTSU_USAGE_LOG_PATH", filepath.Join(tmp, "mixed.jsonl"))
	// Mix valid + malformed manually
	Append("jutsu agent list", 0, 10*time.Millisecond)
	// Write a malformed line
	path, _ := LogPath()
	if err := appendRaw(path, "not-json\n"); err != nil {
		t.Fatal(err)
	}
	Append("jutsu finding stats", 0, 20*time.Millisecond)

	entries, err := Read()
	if err != nil {
		t.Fatalf("Read should swallow malformed: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 valid entries (malformed skipped); got %d", len(entries))
	}
}

// TestAppend_LineSchemaPinsPrivacyContract reads back a logged line as
// a flat map and asserts the JSON keys are EXACTLY {ts, cmd, exit, ms}.
// Pins the package's documented privacy contract: no flag values, no
// args, no content. If a future contributor adds a field to Entry (e.g.
// args, env, file paths) the test fails on the raw JSON shape, not just
// the typed struct round-trip — catches accidental leaks at the wire
// level.
func TestAppend_LineSchemaPinsPrivacyContract(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("KAIJUTSU_USAGE_LOG_PATH", filepath.Join(tmp, "schema.jsonl"))
	t.Setenv("KAIJUTSU_USAGE_LOG", "1")
	Append("jutsu agent list", 0, 50*time.Millisecond)

	path, _ := LogPath()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line; got %d", len(lines))
	}
	var asMap map[string]interface{}
	if err := json.Unmarshal([]byte(lines[0]), &asMap); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	wantKeys := map[string]bool{"ts": true, "cmd": true, "exit": true, "ms": true}
	for k := range asMap {
		if !wantKeys[k] {
			t.Errorf("unexpected key %q in usage log entry — privacy contract violation; keys must be exactly {ts, cmd, exit, ms}", k)
		}
	}
	for k := range wantKeys {
		if _, ok := asMap[k]; !ok {
			t.Errorf("missing required key %q in usage log entry", k)
		}
	}
}

func TestAppend_FailureIsSilent(t *testing.T) {
	t.Setenv("KAIJUTSU_USAGE_LOG_PATH", "/proc/nonexistent/path/u.jsonl")
	t.Setenv("KAIJUTSU_USAGE_LOG", "1") // explicit enable
	// Should not panic, return, or block. Telemetry-write failure must not
	// affect the calling command.
	Append("jutsu whatever", 0, time.Millisecond)
	// Just reaching here is the assertion.
}

// appendRaw is a test-only helper for injecting malformed lines.
func appendRaw(path, content string) error {
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(content)
	return err
}
