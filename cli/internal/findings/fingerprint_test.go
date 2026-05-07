package findings

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestFingerprint_Origin covers chain step 1 — a freshly init'd repo
// with an "origin" remote produces a 16-hex fingerprint anchored on
// (remote-url, basename).
func TestFingerprint_Origin(t *testing.T) {
	skipIfNoGit(t)
	repo := mkRepo(t, "myproj")
	gitDo(t, repo, "remote", "add", "origin", "git@github.com:acme/myproj.git")

	fp := Fingerprint(repo)
	if !looksLikeHex16(fp) {
		t.Fatalf("fp=%q, want 16 hex chars", fp)
	}

	// Determinism: same repo + remote → same fp.
	if fp2 := Fingerprint(repo); fp2 != fp {
		t.Fatalf("non-deterministic: %q vs %q", fp, fp2)
	}
}

// TestFingerprint_Upstream covers chain step 2 — origin missing,
// upstream present.
func TestFingerprint_Upstream(t *testing.T) {
	skipIfNoGit(t)
	repo := mkRepo(t, "fork")
	gitDo(t, repo, "remote", "add", "upstream", "git@github.com:acme/fork.git")

	fp := Fingerprint(repo)
	if !looksLikeHex16(fp) {
		t.Fatalf("fp=%q, want 16 hex chars", fp)
	}
}

// TestFingerprint_AlphabeticalFirst covers chain step 3 — neither
// origin nor upstream; pick the alphabetically-first remote.
func TestFingerprint_AlphabeticalFirst(t *testing.T) {
	skipIfNoGit(t)
	repo := mkRepo(t, "weird")
	gitDo(t, repo, "remote", "add", "zeta", "git@github.com:z/weird.git")
	gitDo(t, repo, "remote", "add", "alpha", "git@github.com:a/weird.git")

	fp := Fingerprint(repo)
	// Verify it picked "alpha" (alphabetical first) by reproducing the
	// hash manually.
	want := hash16("git@github.com:a/weird.git" + ":" + filepath.Base(repo))
	if fp != want {
		t.Fatalf("fp=%q, want %q (from alphabetical-first remote)", fp, want)
	}
}

// TestFingerprint_LocalGit covers chain step 4 — git repo with no
// remotes configured.
func TestFingerprint_LocalGit(t *testing.T) {
	skipIfNoGit(t)
	repo := mkRepo(t, "no-remotes")

	fp := Fingerprint(repo)
	if !strings.HasPrefix(fp, "local-git:") {
		t.Fatalf("fp=%q, want local-git: prefix", fp)
	}
	if !looksLikeHex16(strings.TrimPrefix(fp, "local-git:")) {
		t.Fatalf("fp=%q, hash part not 16 hex", fp)
	}
}

// TestFingerprint_LocalFS covers chain step 5 — not a git repo at all.
func TestFingerprint_LocalFS(t *testing.T) {
	tmp := t.TempDir()
	fp := Fingerprint(tmp)
	if !strings.HasPrefix(fp, "local-fs:") {
		t.Fatalf("fp=%q, want local-fs: prefix", fp)
	}
	if !looksLikeHex16(strings.TrimPrefix(fp, "local-fs:")) {
		t.Fatalf("fp=%q, hash part not 16 hex", fp)
	}
}

// TestFingerprint_DifferentBasename verifies the basename anchoring:
// two clones of the same upstream URL but with different directory
// names produce DIFFERENT fingerprints. This is the v0.7 fork-aware
// design — tracking quality stats separately for users who maintain
// distinct working copies.
func TestFingerprint_DifferentBasename(t *testing.T) {
	skipIfNoGit(t)
	repoA := mkRepo(t, "name-A")
	gitDo(t, repoA, "remote", "add", "origin", "git@github.com:acme/proj.git")
	repoB := mkRepo(t, "name-B")
	gitDo(t, repoB, "remote", "add", "origin", "git@github.com:acme/proj.git")

	fpA := Fingerprint(repoA)
	fpB := Fingerprint(repoB)
	if fpA == fpB {
		t.Fatalf("fps must differ for different basenames; both = %q", fpA)
	}
}

// TestFingerprint_SameBasenameSameRemote conversely verifies that two
// directories with matching basename + matching remote URL yield the
// SAME fingerprint — the cross-machine stability property.
func TestFingerprint_SameBasenameSameRemote(t *testing.T) {
	skipIfNoGit(t)
	parentA := t.TempDir()
	parentB := t.TempDir()
	repoA := mkRepoIn(t, parentA, "shared")
	repoB := mkRepoIn(t, parentB, "shared")
	gitDo(t, repoA, "remote", "add", "origin", "git@github.com:acme/proj.git")
	gitDo(t, repoB, "remote", "add", "origin", "git@github.com:acme/proj.git")

	if a, b := Fingerprint(repoA), Fingerprint(repoB); a != b {
		t.Fatalf("fps must match across machines for same (remote, basename); A=%q B=%q", a, b)
	}
}

// --- helpers ---

func skipIfNoGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}
}

func mkRepo(t *testing.T, name string) string {
	t.Helper()
	return mkRepoIn(t, t.TempDir(), name)
}

func mkRepoIn(t *testing.T, parent, name string) string {
	t.Helper()
	dir := filepath.Join(parent, name)
	if err := exec.Command("git", "init", "-q", dir).Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}
	return dir
}

func gitDo(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v in %s: %v: %s", args, dir, err, out)
	}
}

func looksLikeHex16(s string) bool {
	if len(s) != 16 {
		return false
	}
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
		case r >= 'a' && r <= 'f':
		default:
			return false
		}
	}
	return true
}
