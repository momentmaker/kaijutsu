package usage

import (
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
	t.Setenv("KAIJUTSU_USAGE_LOG", "")
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

func TestAppend_FailureIsSilent(t *testing.T) {
	t.Setenv("KAIJUTSU_USAGE_LOG_PATH", "/proc/nonexistent/path/u.jsonl")
	t.Setenv("KAIJUTSU_USAGE_LOG", "")
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
