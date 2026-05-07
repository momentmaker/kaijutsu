package findings

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// Fingerprint resolves a stable codebase identity for cwd, used as the
// `codebase_fp` column in findings. The chain (first match wins) per
// the v0.7 spec:
//
//  1. `git remote get-url origin`            → sha256(remote ":" basename)[:16]
//  2. `git remote get-url upstream`          → sha256(remote ":" basename)[:16]
//  3. any remote (alphabetical first)        → sha256(remote ":" basename)[:16]
//  4. inside a git work tree, no remotes     → "local-git:" + sha256(toplevel)[:16]
//  5. not a git repo at all                  → "local-fs:"  + sha256(cwd)[:16]
//
// Steps 1-3 use the work-tree basename so two forks cloned to the same
// directory name share a fingerprint, but a fork cloned with a
// different name doesn't. Steps 4-5 use the absolute path directly
// because there's no remote URL to anchor on.
//
// Fingerprint never returns an error: every cwd resolves to *some*
// stable string. The git binary failing entirely just routes us to
// step 5 (local-fs).
//
// Output is always 16 hex chars OR a typed prefix + ":" + 16 hex chars
// — callers may look at the prefix for diagnostics ("you're not in a
// git repo") but must NOT treat the fingerprint shape as a contract:
// future versions may change the prefix scheme.
func Fingerprint(cwd string) string {
	if remote, err := gitRemoteURL(cwd, "origin"); err == nil && remote != "" {
		return hashRemoteWithBasename(cwd, remote)
	}
	if remote, err := gitRemoteURL(cwd, "upstream"); err == nil && remote != "" {
		return hashRemoteWithBasename(cwd, remote)
	}
	if remote, err := firstAlphabeticalRemote(cwd); err == nil && remote != "" {
		return hashRemoteWithBasename(cwd, remote)
	}
	if top, err := gitToplevel(cwd); err == nil && top != "" {
		// step 4 — git repo without remotes (fresh init, detached
		// worktree). The toplevel abspath is stable across re-runs in
		// the same dir but won't match other machines or other clones.
		return "local-git:" + hash16(top)
	}
	// step 5 — last resort. cwd may be a relative path; resolve to abs
	// so two invocations from different relative starting points still
	// produce the same fingerprint.
	abs, err := filepath.Abs(cwd)
	if err != nil {
		// filepath.Abs only errors when os.Getwd fails AND cwd is
		// relative. Treat the literal cwd string as the anchor — better
		// than crashing.
		abs = cwd
	}
	return "local-fs:" + hash16(abs)
}

func hashRemoteWithBasename(cwd, remote string) string {
	base := ""
	if top, err := gitToplevel(cwd); err == nil {
		base = filepath.Base(top)
	} else {
		// Fallback: try filepath.Abs(cwd) first; only on its rare
		// failure (Getwd failed AND cwd is relative) drop down to
		// the raw cwd basename. Previously we ignored the Abs error
		// and let "" → "." collapse the basename, which would map
		// every cwd lacking a git toplevel to the same fingerprint.
		if abs, err := filepath.Abs(cwd); err == nil {
			base = filepath.Base(abs)
		} else {
			base = filepath.Base(cwd)
		}
		// Even with the fallback, basename of "" or "/" produces "."
		// or "/" — treat both as the unstable case and stamp the cwd
		// itself in to avoid same-fp collisions across truly different
		// roots. Cheap belt-and-suspenders.
		if base == "." || base == "/" || base == "" {
			base = cwd
		}
	}
	return hash16(strings.TrimSpace(remote) + ":" + base)
}

func hash16(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])[:16]
}

// gitRemoteURL returns the URL for the named remote, or empty string
// + error if the remote isn't configured (or git itself is unavailable).
// We treat both "no such remote" and "not a git repo" as the same
// "no value" outcome — caller falls through to the next chain step.
func gitRemoteURL(cwd, name string) (string, error) {
	out, err := runGit(cwd, "remote", "get-url", name)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// firstAlphabeticalRemote enumerates all remotes via `git remote -v`
// (one line per fetch + push pair, so the same name appears twice),
// dedupes to a sorted list, and returns the URL of the first one.
// Used as step 3 of the fallback chain when neither origin nor
// upstream is configured but some other remote (e.g. user added a
// custom name like "kaiju") exists.
func firstAlphabeticalRemote(cwd string) (string, error) {
	out, err := runGit(cwd, "remote", "-v")
	if err != nil {
		return "", err
	}
	type pair struct{ name, url string }
	pairs := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Format: "<name>\t<url> (fetch)" / "(push)". Splitting on
		// tab/space gives us name + url + suffix; the suffix is
		// thrown away.
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		pairs[fields[0]] = fields[1]
	}
	if len(pairs) == 0 {
		return "", errors.New("no remotes configured")
	}
	names := make([]string, 0, len(pairs))
	for n := range pairs {
		names = append(names, n)
	}
	sort.Strings(names)
	return pairs[names[0]], nil
}

// gitToplevel returns the work-tree root (i.e. the directory
// containing .git). Returns an error if cwd is not inside a git
// work-tree — that error is the signal to skip to step 5 (non-git).
func gitToplevel(cwd string) (string, error) {
	out, err := runGit(cwd, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// runGit shells out to `git` with cwd as the working dir. We
// deliberately don't honor GIT_DIR / GIT_WORK_TREE env vars beyond
// what git itself does — fingerprint stability depends on the user's
// real cwd, not any override.
func runGit(cwd string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = cwd
	out, err := cmd.Output()
	return string(out), err
}
