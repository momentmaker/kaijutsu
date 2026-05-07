package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestSlugifyTopic covers the slug derivation: lowercase, non-
// alphanumeric → -, collapse repeats, trim, 50-char cap.
func TestSlugifyTopic(t *testing.T) {
	cases := map[string]string{
		"Should we ship dream v0.8?":                "should-we-ship-dream-v0-8",
		"Multi-Agent SWARM!! With *bold*":           "multi-agent-swarm-with-bold",
		"":                                          "untitled",
		"   ":                                       "untitled",
		strings.Repeat("a", 100):                    strings.Repeat("a", 50),
		"weird___chars!!!and___multiple   spaces":   "weird-chars-and-multiple-spaces",
	}
	for in, want := range cases {
		got := SlugifyTopic(in)
		if got != want {
			t.Errorf("SlugifyTopic(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestSanitizeFP covers the colon-to-underscore swap for filesystem-
// safe codebase fingerprints. local-fs:abc → local-fs_abc.
func TestSanitizeFP(t *testing.T) {
	cases := map[string]string{
		"abc123def4567890":     "abc123def4567890", // hex form unchanged
		"local-fs:abc123":      "local-fs_abc123",
		"local-git:def456":     "local-git_def456",
	}
	for in, want := range cases {
		got := SanitizeFP(in)
		if got != want {
			t.Errorf("SanitizeFP(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestParseDreamFilename covers the round-trip: build a filename
// via WriteDreamSession's pattern, parse it back via
// parseDreamFilename, fields match.
func TestParseDreamFilename(t *testing.T) {
	cases := []struct {
		name      string
		fp        string
		slug      string
		hasMatch  bool
	}{
		{"abc123def4567890-my-topic-20260507-153045.md", "abc123def4567890", "my-topic", true},
		{"local-fs_xyz-a-topic-20260101-120000.md", "local-fs_xyz", "a-topic", true},
		{"random-noise.md", "", "", false},
		{"abc-no-timestamp.md", "", "", false},
		{"abc123def4567890-topic-bad-timestamp.md", "", "", false},
	}
	for _, tc := range cases {
		got := parseDreamFilename(tc.name)
		if (got != nil) != tc.hasMatch {
			t.Errorf("parseDreamFilename(%q): hasMatch = %v, want %v", tc.name, got != nil, tc.hasMatch)
			continue
		}
		if !tc.hasMatch {
			continue
		}
		if got.CodebaseFP != tc.fp {
			t.Errorf("parseDreamFilename(%q).CodebaseFP = %q, want %q", tc.name, got.CodebaseFP, tc.fp)
		}
		if got.TopicSlug != tc.slug {
			t.Errorf("parseDreamFilename(%q).TopicSlug = %q, want %q", tc.name, got.TopicSlug, tc.slug)
		}
	}
}

// TestWriteDreamSession_RoundTrip covers the auto-write happy path:
// create a session, parse it back from the graveyard, fields line up.
func TestWriteDreamSession_RoundTrip(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("KAIJUTSU_DREAM_DIR", tmp)

	path, err := WriteDreamSession("Should we ship dream", "abc123def4567890", "# dream output\n\nbody", []string{"honest", "fit", "gaps", "wild"}, "swarm", false)
	if err != nil {
		t.Fatalf("WriteDreamSession: %v", err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read written: %v", err)
	}
	bodyStr := string(body)
	for _, must := range []string{
		`topic: "Should we ship dream"`,
		"codebase_fp: abc123def4567890",
		"lens_order: [honest, fit, gaps, wild]",
		"mode: swarm",
		"# dream output",
	} {
		if !strings.Contains(bodyStr, must) {
			t.Errorf("missing %q in written body", must)
		}
	}
	st, _ := os.Stat(path)
	if st.Mode().Perm() != 0o600 {
		t.Errorf("file mode = %o, want 0600", st.Mode().Perm())
	}
}

// TestFindRecentDreamForTopic covers the lens-rotation rule's
// dependency: dream session within `within` for same (fp, topic) is
// found; old or different-topic ones are skipped.
func TestFindRecentDreamForTopic(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("KAIJUTSU_DREAM_DIR", tmp)

	// Recent matching dream.
	if _, err := WriteDreamSession("My Big Topic", "abc123def4567890", "body", []string{"honest"}, "swarm", false); err != nil {
		t.Fatalf("write recent: %v", err)
	}
	// Different topic — should NOT match.
	if _, err := WriteDreamSession("Different Idea", "abc123def4567890", "body", []string{"fit"}, "swarm", false); err != nil {
		t.Fatalf("write different: %v", err)
	}

	got, err := FindRecentDreamForTopic("My Big Topic", "abc123def4567890", 7*24*time.Hour)
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if got == nil {
		t.Fatal("expected to find recent dream, got nil")
	}
	if got.TopicSlug != "my-big-topic" {
		t.Errorf("matched wrong slug: %q", got.TopicSlug)
	}

	// 0-duration window — should not match (timestamp is in the past).
	got, _ = FindRecentDreamForTopic("My Big Topic", "abc123def4567890", 0)
	if got != nil {
		t.Errorf("0-window should return nil, got %+v", got)
	}

	// Different fp — no match.
	got, _ = FindRecentDreamForTopic("My Big Topic", "different-fp", 7*24*time.Hour)
	if got != nil {
		t.Errorf("different fp should return nil, got %+v", got)
	}
}

// TestDreamList_EmptyGraveyardRendersHelpfulMessage covers the
// no-dreams case so the user (or agent) gets actionable text instead
// of a blank table.
func TestDreamList_EmptyGraveyardRendersHelpfulMessage(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("KAIJUTSU_DREAM_DIR", tmp)

	var buf bytes.Buffer
	renderDreamList(&buf, nil, "abc123def4567890", false)
	if !strings.Contains(buf.String(), "no dream sessions") {
		t.Errorf("missing helpful empty-state message:\n%s", buf.String())
	}
}

// TestDreamClear_DryRunByDefault covers the destructive-default-off
// behavior. Mirrors `jutsu finding clear` UX.
func TestDreamClear_DryRunByDefault(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("KAIJUTSU_DREAM_DIR", tmp)
	// Seed a file so there's something to clear.
	if _, err := WriteDreamSession("topic", "abc123def4567890", "body", []string{"honest"}, "swarm", false); err != nil {
		t.Fatalf("seed: %v", err)
	}

	out, _, err := runCmdCapture(t, "dream", "clear", "--all-codebases")
	if err != nil {
		t.Fatalf("dream clear (dry-run): %v", err)
	}
	if !strings.Contains(out, "[dry-run]") {
		t.Errorf("expected dry-run output; got: %s", out)
	}
	// File should still exist.
	files, _ := os.ReadDir(tmp)
	if len(files) != 1 {
		t.Errorf("file count = %d, want 1 (dry-run shouldn't delete)", len(files))
	}
}

// TestDreamClear_YesActuallyDeletes covers the destructive path with
// --yes.
func TestDreamClear_YesActuallyDeletes(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("KAIJUTSU_DREAM_DIR", tmp)
	if _, err := WriteDreamSession("topic", "abc123def4567890", "body", []string{"honest"}, "swarm", false); err != nil {
		t.Fatalf("seed: %v", err)
	}

	out, _, err := runCmdCapture(t, "dream", "clear", "--all-codebases", "--yes")
	if err != nil {
		t.Fatalf("dream clear --yes: %v", err)
	}
	if !strings.Contains(out, "deleted 1") {
		t.Errorf("expected delete count; got: %s", out)
	}
	files, _ := os.ReadDir(tmp)
	if len(files) != 0 {
		t.Errorf("file count = %d, want 0 (--yes should delete)", len(files))
	}
}

// TestDurationShort_Boundaries covers the human-readable duration
// rendering used in the dream list table.
func TestDurationShort_Boundaries(t *testing.T) {
	cases := map[time.Duration]string{
		30 * time.Second:        "<1m",
		time.Minute:             "1m",
		90 * time.Second:        "1m",
		61 * time.Minute:        "1h",
		25 * time.Hour:          "1d",
		7 * 24 * time.Hour:      "7d",
	}
	for d, want := range cases {
		got := durationShort(d)
		if got != want {
			t.Errorf("durationShort(%v) = %q, want %q", d, got, want)
		}
	}

	// Path through filepath usage so the import doesn't get pruned
	// by accident in future edits.
	_ = filepath.Join
}
