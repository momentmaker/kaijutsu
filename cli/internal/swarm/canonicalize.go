package swarm

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

// FileEntry pairs a path with its already-read bytes. Inputs of
// InputKind InputFiles get represented as []FileEntry so the cache-
// key generator and prompt assembler agree on canonical ordering.
type FileEntry struct {
	Path    string
	Content string
}

// CanonicalizePrompt applies the InputPrompt normalization rules
// pinned in IMPLEMENTATION_PLAN_PHASE2.md's locked-decisions row:
// TrimSpace, then collapse runs of internal whitespace to a single
// ASCII space. Unicode NFC normalization is deferred (would require
// golang.org/x/text/unicode/norm; tracked as a polish item in the
// plan's open-questions section). For ASCII-dominant prompt input
// (the common case) the current rules are sufficient to make
// trivial whitespace edits hash to the same cache key.
func CanonicalizePrompt(s string) string {
	s = strings.TrimSpace(s)
	return collapseInternalWhitespace(s)
}

// collapseInternalWhitespace replaces every run of >1 whitespace
// chars (including tabs and newlines) with a single ASCII space.
func collapseInternalWhitespace(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	prevSpace := false
	for _, r := range s {
		if isWhitespace(r) {
			if !prevSpace {
				b.WriteByte(' ')
				prevSpace = true
			}
			continue
		}
		b.WriteRune(r)
		prevSpace = false
	}
	return b.String()
}

func isWhitespace(r rune) bool {
	switch r {
	case ' ', '\t', '\n', '\r', '\v', '\f':
		return true
	}
	return false
}

// SortFilesCanonical returns a copy of entries sorted by Path. The
// cache key generator and the prompt-body assembler must use the
// same order so the same input set produces the same key regardless
// of CLI argument order.
func SortFilesCanonical(entries []FileEntry) []FileEntry {
	out := make([]FileEntry, len(entries))
	copy(out, entries)
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// AssembleFilesBody concatenates sorted file entries into a single
// review body with `--- <path>\n` section prefixes. The same body
// is fed to the cache-key generator and to the agent prompt so
// they agree on what was reviewed.
func AssembleFilesBody(entries []FileEntry) string {
	sorted := SortFilesCanonical(entries)
	var b strings.Builder
	for _, e := range sorted {
		fmt.Fprintf(&b, "--- %s\n", e.Path)
		b.WriteString(e.Content)
		if !strings.HasSuffix(e.Content, "\n") {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// CacheKeyForFiles derives the per-preset cache key for an
// InputFiles run. Salts with preset.Name so two presets reviewing
// the same file set don't collide. Returns the first 12 lowercase
// hex chars of SHA256(preset + canonical body).
func CacheKeyForFiles(preset *Preset, entries []FileEntry) string {
	body := AssembleFilesBody(entries)
	return saltedHashKey(preset.Name, body)
}

// CacheKeyForPrompt derives the per-preset cache key for an
// InputPrompt run. Applies CanonicalizePrompt then salts with
// preset.Name.
func CacheKeyForPrompt(preset *Preset, prompt string) string {
	return saltedHashKey(preset.Name, CanonicalizePrompt(prompt))
}

func saltedHashKey(salt, body string) string {
	h := sha256.New()
	h.Write([]byte(salt))
	h.Write([]byte{0}) // separator so salt+body collisions are avoided
	h.Write([]byte(body))
	full := hex.EncodeToString(h.Sum(nil))
	return full[:12]
}
