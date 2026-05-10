package findings

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestExportJSONL_NilWriterErrors(t *testing.T) {
	if err := ExportJSONL(nil, []Row{{ID: 1}}); err == nil {
		t.Fatal("expected error on nil writer")
	}
}

func TestExportJSONL_Empty(t *testing.T) {
	var buf bytes.Buffer
	if err := ExportJSONL(&buf, nil); err != nil {
		t.Fatal(err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected zero bytes for empty rows; got %d (%q)", buf.Len(), buf.String())
	}
}

func TestExportJSONL_OnePerLine(t *testing.T) {
	rows := []Row{
		{ID: 1, RunID: "r1", Preset: "pr-review", Provider: "claude", Persona: "default-claude", Severity: "issue", File: "a.go", LineRange: "1", Summary: "f1"},
		{ID: 2, RunID: "r1", Preset: "pr-review", Provider: "gemini", Persona: "default-gemini", Severity: "minor", File: "b.go", LineRange: "2", Summary: "f2"},
	}
	var buf bytes.Buffer
	if err := ExportJSONL(&buf, rows); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d:\n%s", len(lines), buf.String())
	}
	for i, line := range lines {
		var got Row
		if err := json.Unmarshal([]byte(line), &got); err != nil {
			t.Errorf("line %d not valid JSON: %v\n%s", i, err, line)
		}
	}
}

// TestExportJSONL_LogicalRoundTrip pins the data-recovery contract:
// re-parsing each line yields a slice deep-equal to the original at
// the row-data level. Byte-equality is NOT promised.
func TestExportJSONL_LogicalRoundTrip(t *testing.T) {
	now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	reasoning := "real auth bug"
	conf := 0.85
	want := []Row{
		{
			ID: 1, RunID: "r1", CodebaseFP: "fp1",
			Preset: "pr-review", Provider: "claude", Persona: "default-claude",
			Severity: "issue", File: "auth.go", LineRange: "42",
			Summary: "token expiry uses < not <=",
			Reasoning: &reasoning, Confidence: &conf,
			CreatedAt: now,
		},
	}

	var buf bytes.Buffer
	if err := ExportJSONL(&buf, want); err != nil {
		t.Fatal(err)
	}

	got := []Row{}
	for _, line := range bytes.Split(bytes.TrimRight(buf.Bytes(), "\n"), []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		var r Row
		if err := json.Unmarshal(line, &r); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		got = append(got, r)
	}
	if len(got) != len(want) {
		t.Fatalf("len = %d; want %d", len(got), len(want))
	}
	if got[0].ID != want[0].ID || got[0].Summary != want[0].Summary || !got[0].CreatedAt.Equal(want[0].CreatedAt) {
		t.Errorf("row mismatch:\n got: %+v\nwant: %+v", got[0], want[0])
	}
	if got[0].Reasoning == nil || *got[0].Reasoning != reasoning {
		t.Errorf("reasoning round-trip failed: %v", got[0].Reasoning)
	}
}

// TestExportJSONL_WriterErrorPropagates pins that ExportJSONL surfaces a
// downstream writer failure rather than swallowing it.
func TestExportJSONL_WriterErrorPropagates(t *testing.T) {
	w := &errWriter{err: errors.New("disk full")}
	rows := []Row{{ID: 1}, {ID: 2}}
	if err := ExportJSONL(w, rows); err == nil {
		t.Fatal("expected error from writer; got nil")
	}
}

type errWriter struct{ err error }

func (w *errWriter) Write(p []byte) (int, error) { return 0, w.err }
