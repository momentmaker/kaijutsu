package swarm

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// shaRe matches a 7-to-64-char lowercase hex string. CacheDir and
// LoadCachedResults reject anything else so a hostile `--replay` arg
// cannot escape the cache root via path traversal (e.g.
// `--replay ../../../etc`).
var shaRe = regexp.MustCompile(`^[0-9a-f]{7,64}$`)

// ValidateSHA returns an error if sha is not in the form expected for
// a git commit hash (lowercase hex, 7–64 chars). Exposed so callers
// can fail-fast on user-supplied input before computing paths.
func ValidateSHA(sha string) error {
	if !shaRe.MatchString(sha) {
		return fmt.Errorf("invalid SHA %q: must be lowercase hex, 7–64 chars", sha)
	}
	return nil
}

// CacheDir returns the on-disk path for a per-key run cache. Layout:
//
//	<projectRoot>/.kaijutsu/<preset.cachePathSegment()>/<key>/
//	  claude.json
//	  codex.json
//	  antigravity.json
//	  synthesis.md
//
// Used by --replay <key> to re-render markdown without re-calling
// the model APIs. Phase 2 generalizes from the Phase-1 hardcoded
// "pr-review-runs/" path so each preset gets its own subdir.
func CacheDir(projectRoot string, preset *Preset, key string) string {
	return filepath.Join(projectRoot, ".kaijutsu", preset.cachePathSegment(), key)
}

// CacheRun writes per-agent results + the synthesis markdown into
// the per-key cache directory. Best-effort — cache failure should
// not abort the run, just warn. Validates key shape so a surprising
// upstream value (e.g. empty or path-traversal-y) never produces a
// write outside the cache root.
//
// Stage 1 only validates SHA-shaped keys (pr-review uses head SHA).
// Stage 2 will accept hashed-input keys for InputFiles/InputPrompt
// presets — same hex-only constraint, different generator.
func CacheRun(projectRoot string, preset *Preset, key string, results []AgentResult, synthesisMarkdown string) error {
	if err := ValidateSHA(key); err != nil {
		return err
	}
	dir := CacheDir(projectRoot, preset, key)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	for _, r := range results {
		out, err := json.MarshalIndent(r, "", "  ")
		if err != nil {
			return err
		}
		path := filepath.Join(dir, sanitizeAgent(r.Agent)+".json")
		if err := os.WriteFile(path, out, 0644); err != nil {
			return err
		}
	}
	if synthesisMarkdown != "" {
		if err := os.WriteFile(filepath.Join(dir, "synthesis.md"), []byte(synthesisMarkdown), 0644); err != nil {
			return err
		}
	}
	return nil
}

// LoadCachedResults reads every <agent>.json from the per-key cache
// directory. Used by --replay. Returns an error if the key is
// malformed, the directory doesn't exist, or no .json files inside.
func LoadCachedResults(projectRoot string, preset *Preset, key string) ([]AgentResult, error) {
	if err := ValidateSHA(key); err != nil {
		return nil, err
	}
	dir := CacheDir(projectRoot, preset, key)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("cache dir %s: %w", dir, err)
	}
	var results []AgentResult
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, rerr := os.ReadFile(filepath.Join(dir, e.Name()))
		if rerr != nil {
			continue
		}
		var r AgentResult
		if jerr := json.Unmarshal(data, &r); jerr != nil {
			continue
		}
		results = append(results, r)
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("no cached results found in %s", dir)
	}
	return results, nil
}

func sanitizeAgent(name string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= '0' && r <= '9':
			return r
		case r == '-' || r == '_':
			return r
		}
		return -1
	}, name)
}
