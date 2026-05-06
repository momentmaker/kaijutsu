package swarm

import (
	"strings"
	"testing"
)

func TestCanonicalizePrompt_TrimAndCollapse(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"  hi   world  ", "hi world"},
		{"hi\t\tworld", "hi world"},
		{"hi\n\nworld", "hi world"},
		{"hi world", "hi world"},
		{"  ", ""},
		{"", ""},
		{"a", "a"},
		{"a  b  c", "a b c"},
	}
	for _, tc := range cases {
		got := CanonicalizePrompt(tc.in)
		if got != tc.want {
			t.Errorf("CanonicalizePrompt(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestCacheKeyForPrompt_StableAcrossWhitespaceVariants(t *testing.T) {
	preset := &Preset{Name: "brainstorm"}
	a := CacheKeyForPrompt(preset, "  how do I  rate-limit  ")
	b := CacheKeyForPrompt(preset, "how do I rate-limit")
	c := CacheKeyForPrompt(preset, "how\tdo\tI\trate-limit")
	if a != b || b != c {
		t.Errorf("expected all three to hash equal, got %q %q %q", a, b, c)
	}
}

func TestCacheKeyForPrompt_DiffersAcrossPresets(t *testing.T) {
	a := CacheKeyForPrompt(&Preset{Name: "brainstorm"}, "x")
	b := CacheKeyForPrompt(&Preset{Name: "doc-review"}, "x")
	if a == b {
		t.Errorf("same prompt under different presets should hash differently, both = %q", a)
	}
}

func TestCacheKeyForFiles_PathOrderIndependent(t *testing.T) {
	preset := &Preset{Name: "refactor-plan"}
	a := CacheKeyForFiles(preset, []FileEntry{
		{Path: "a.go", Content: "alpha"},
		{Path: "b.go", Content: "beta"},
	})
	b := CacheKeyForFiles(preset, []FileEntry{
		{Path: "b.go", Content: "beta"},
		{Path: "a.go", Content: "alpha"},
	})
	if a != b {
		t.Errorf("path order should not affect cache key, got %q vs %q", a, b)
	}
}

func TestCacheKeyForFiles_ContentChangeChangesKey(t *testing.T) {
	preset := &Preset{Name: "refactor-plan"}
	a := CacheKeyForFiles(preset, []FileEntry{{Path: "a.go", Content: "alpha"}})
	b := CacheKeyForFiles(preset, []FileEntry{{Path: "a.go", Content: "beta"}})
	if a == b {
		t.Errorf("content change should change cache key, both = %q", a)
	}
}

func TestMixCacheKeyWithPersonas_EmptyReturnsBaseUnchanged(t *testing.T) {
	got := MixCacheKeyWithPersonas("abc123", nil)
	if got != "abc123" {
		t.Errorf("nil personas: got %q, want %q (legacy v0.5 cache compat)", got, "abc123")
	}
	got2 := MixCacheKeyWithPersonas("abc123", []string{})
	if got2 != "abc123" {
		t.Errorf("empty personas: got %q, want %q", got2, "abc123")
	}
}

func TestMixCacheKeyWithPersonas_FlagOrderInvariant(t *testing.T) {
	a := MixCacheKeyWithPersonas("base", []string{"alice", "bob", "carol"})
	b := MixCacheKeyWithPersonas("base", []string{"carol", "alice", "bob"})
	if a != b {
		t.Errorf("persona ordering should not affect key: %q vs %q", a, b)
	}
}

func TestMixCacheKeyWithPersonas_DifferentMixesDiffer(t *testing.T) {
	a := MixCacheKeyWithPersonas("base", []string{"alice", "bob"})
	b := MixCacheKeyWithPersonas("base", []string{"carol", "dave"})
	if a == b {
		t.Errorf("two different mixes produced the same key: %q (cache collision risk)", a)
	}
}

func TestMixCacheKeyWithPersonas_LegacyMixDiffersFromBare(t *testing.T) {
	// Spec D6: --personas mode invalidates legacy cache. Even passing
	// the v0.5 default-* names explicitly via --personas is a NEW
	// cache key — the user opted into the persona path.
	bare := "abc123"
	withPersonas := MixCacheKeyWithPersonas("abc123", []string{"default-claude", "default-codex", "default-gemini"})
	if bare == withPersonas {
		t.Error("explicit default-* personas should still differ from bare/no-flag mode (user-intent signal)")
	}
}

func TestCacheKeyForFilesWithGoal_GoalAffectsKey(t *testing.T) {
	preset := &Preset{Name: "refactor-plan"}
	files := []FileEntry{{Path: "a.go", Content: "alpha"}}
	a := CacheKeyForFilesWithGoal(preset, files, "extract handler")
	b := CacheKeyForFilesWithGoal(preset, files, "rename module")
	if a == b {
		t.Errorf("different goals should produce different keys, both = %q", a)
	}
}

func TestCacheKeyForFilesWithGoal_EmptyGoalMatchesPlain(t *testing.T) {
	preset := &Preset{Name: "refactor-plan"}
	files := []FileEntry{{Path: "a.go", Content: "alpha"}}
	plain := CacheKeyForFiles(preset, files)
	withEmpty := CacheKeyForFilesWithGoal(preset, files, "")
	if plain != withEmpty {
		t.Errorf("empty goal should match CacheKeyForFiles, got %q vs %q", plain, withEmpty)
	}
}

func TestCacheKeyForFilesWithGoal_GoalWhitespaceNormalized(t *testing.T) {
	preset := &Preset{Name: "refactor-plan"}
	files := []FileEntry{{Path: "a.go", Content: "alpha"}}
	a := CacheKeyForFilesWithGoal(preset, files, "extract handler")
	b := CacheKeyForFilesWithGoal(preset, files, "  extract handler  \n")
	if a != b {
		t.Errorf("goal trim should not affect key, got %q vs %q", a, b)
	}
}

func TestAssembleFilesBody_PrefixesAndSorts(t *testing.T) {
	body := AssembleFilesBody([]FileEntry{
		{Path: "z.go", Content: "z body"},
		{Path: "a.go", Content: "a body\n"},
	})
	// a.go comes first (sorted), then z.go
	if !strings.HasPrefix(body, "--- a.go\n") {
		t.Errorf("expected a.go to come first, got prefix:\n%s", body)
	}
	if !strings.Contains(body, "--- z.go\n") {
		t.Errorf("missing z.go section: %s", body)
	}
	if strings.Index(body, "a.go") > strings.Index(body, "z.go") {
		t.Errorf("ordering wrong:\n%s", body)
	}
	// Each section ends with newline (trailing \n added if missing)
	if !strings.Contains(body, "z body\n") {
		t.Errorf("expected trailing newline appended to z.go body: %s", body)
	}
}

func TestSaltedHashKey_Length12Lowercase(t *testing.T) {
	k := saltedHashKey("salt", "body")
	if len(k) != 12 {
		t.Errorf("want length 12, got %d (%q)", len(k), k)
	}
	if k != strings.ToLower(k) {
		t.Errorf("expected lowercase, got %q", k)
	}
	for _, r := range k {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			t.Errorf("non-hex char %q in key %q", r, k)
		}
	}
}
