package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/momentmaker/kaijutsu/cli/internal/usage"
)

func TestUsageStats_EmptyLogFriendlyMessage(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("KAIJUTSU_USAGE_LOG_PATH", filepath.Join(tmp, "missing.jsonl"))
	out := runCmd(t, "usage", "stats")
	if !strings.Contains(out, "no usage recorded yet") {
		t.Errorf("expected friendly empty message; got: %s", out)
	}
}

func TestUsageStats_GroupsByCommand(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("KAIJUTSU_USAGE_LOG_PATH", filepath.Join(tmp, "u.jsonl"))
	t.Setenv("KAIJUTSU_USAGE_LOG", "1") // explicit enable; "" works today only because Disabled() matches literal "0"

	usage.Append("jutsu finding stats", 0, 100*time.Millisecond)
	usage.Append("jutsu finding stats", 0, 120*time.Millisecond)
	usage.Append("jutsu finding stats", 2, 5*time.Millisecond)
	usage.Append("jutsu install pr-review", 0, 1500*time.Millisecond)

	out := runCmd(t, "usage", "stats")
	if !strings.Contains(out, "jutsu finding stats") {
		t.Errorf("expected finding stats row; got:\n%s", out)
	}
	if !strings.Contains(out, "jutsu install pr-review") {
		t.Errorf("expected install row; got:\n%s", out)
	}
	if !strings.Contains(out, "2 cmd(s)") {
		t.Errorf("expected summary line '2 cmd(s)'; got:\n%s", out)
	}
}

func TestUsageStats_BadSinceRejectedExit2(t *testing.T) {
	cmd := newUsageCmd()
	cmd.SetArgs([]string{"stats", "--since", "0d"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for --since 0d")
	}
	if got := ExitCode(err); got != ExitUsage {
		t.Errorf("ExitCode = %d; want %d (usage)", got, ExitUsage)
	}
}

func TestUsageStats_SinceWindowFilters(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("KAIJUTSU_USAGE_LOG_PATH", filepath.Join(tmp, "u.jsonl"))
	t.Setenv("KAIJUTSU_USAGE_LOG", "1") // explicit enable; "" works today only because Disabled() matches literal "0"

	// Fresh entry
	usage.Append("jutsu finding stats", 0, 100*time.Millisecond)
	// Backdate an old entry by writing direct
	old := time.Now().UTC().Add(-30 * 24 * time.Hour)
	if err := writeRawEntry(filepath.Join(tmp, "u.jsonl"),
		`{"ts":"`+old.Format(time.RFC3339Nano)+`","cmd":"jutsu install old","exit":0,"ms":50}`); err != nil {
		t.Fatal(err)
	}

	out := runCmd(t, "usage", "stats", "--since", "7d")
	if strings.Contains(out, "jutsu install old") {
		t.Errorf("expected old entry filtered out by --since 7d; got:\n%s", out)
	}
	if !strings.Contains(out, "jutsu finding stats") {
		t.Errorf("expected fresh entry; got:\n%s", out)
	}
}

func writeRawEntry(path, line string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(line + "\n")
	return err
}
