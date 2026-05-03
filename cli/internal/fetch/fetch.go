// Package fetch downloads GitHub tarballs, lists tags, and extracts skill
// source trees to a temporary working directory. Used by jutsu install
// and upgrade for remote sources.
//
// Authentication: if GITHUB_TOKEN is set in the environment, all API calls
// include `Authorization: Bearer <token>`. Public repos work unauthenticated
// but at a 60/hr rate limit.
package fetch

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/momentmaker/kaijutsu/cli/internal/source"
)

const (
	// MaxTarballBytes caps how much we read from a tarball download to
	// prevent DoS via an unboundedly large response.
	MaxTarballBytes = 100 * 1024 * 1024 // 100 MiB
	// MaxExtractedBytes caps the total uncompressed size we extract to
	// guard against zip bombs.
	MaxExtractedBytes = 200 * 1024 * 1024 // 200 MiB
	// MaxFileBytes caps any single file fetched via GetFile (e.g.
	// registry/index.json) to keep that endpoint cheap.
	MaxFileBytes = 5 * 1024 * 1024 // 5 MiB
)

// Fetcher performs HTTP requests against GitHub. Construct with New().
type Fetcher struct {
	HTTP  *http.Client
	Token string
}

func New() *Fetcher {
	return &Fetcher{
		HTTP:  &http.Client{Timeout: 60 * time.Second},
		Token: os.Getenv("GITHUB_TOKEN"),
	}
}

// ListTags returns the repo's tags newest-first by semver. Non-semver tags
// are skipped. Stage 3 tolerates leading "v" prefixes.
func (f *Fetcher) ListTags(ctx context.Context, src *source.Source) ([]string, error) {
	var tags []struct {
		Name string `json:"name"`
	}
	if err := f.getJSON(ctx, src.APIBase()+"/tags?per_page=100", &tags); err != nil {
		return nil, err
	}
	type semverTag struct {
		raw string
		v   *semver.Version
	}
	parsed := make([]semverTag, 0, len(tags))
	for _, t := range tags {
		v, err := semver.NewVersion(t.Name)
		if err != nil {
			continue
		}
		parsed = append(parsed, semverTag{raw: t.Name, v: v})
	}
	sort.Slice(parsed, func(i, j int) bool {
		return parsed[i].v.GreaterThan(parsed[j].v)
	})
	out := make([]string, len(parsed))
	for i, p := range parsed {
		out[i] = p.raw
	}
	return out, nil
}

// DefaultBranchSHA returns the latest commit SHA on the repo's default branch.
func (f *Fetcher) DefaultBranchSHA(ctx context.Context, src *source.Source) (string, error) {
	var repo struct {
		DefaultBranch string `json:"default_branch"`
	}
	if err := f.getJSON(ctx, src.APIBase(), &repo); err != nil {
		return "", err
	}
	if repo.DefaultBranch == "" {
		return "", errors.New("default_branch missing in API response")
	}
	var ref struct {
		Object struct {
			SHA string `json:"sha"`
		} `json:"object"`
	}
	if err := f.getJSON(ctx, src.APIBase()+"/git/ref/heads/"+repo.DefaultBranch, &ref); err != nil {
		return "", err
	}
	if ref.Object.SHA == "" {
		return "", errors.New("commit SHA missing in API response")
	}
	return ref.Object.SHA, nil
}

// ResolveTagSHA returns the commit SHA a tag points at. Annotated tags are
// dereferenced to their target commit.
func (f *Fetcher) ResolveTagSHA(ctx context.Context, src *source.Source, tag string) (string, error) {
	var ref struct {
		Object struct {
			Type string `json:"type"`
			SHA  string `json:"sha"`
			URL  string `json:"url"`
		} `json:"object"`
	}
	if err := f.getJSON(ctx, src.APIBase()+"/git/ref/tags/"+tag, &ref); err != nil {
		return "", err
	}
	if ref.Object.Type == "tag" && ref.Object.URL != "" {
		// Annotated tag — dereference to commit.
		var deref struct {
			Object struct {
				SHA string `json:"sha"`
			} `json:"object"`
		}
		if err := f.getJSON(ctx, ref.Object.URL, &deref); err != nil {
			return "", err
		}
		return deref.Object.SHA, nil
	}
	return ref.Object.SHA, nil
}

// Tarball downloads the tarball at ref. Returns the raw bytes and an
// integrity string of the form "sha256-<base64>".
func (f *Fetcher) Tarball(ctx context.Context, src *source.Source, ref string) ([]byte, string, error) {
	url := src.TarballURL(ref)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", err
	}
	if f.Token != "" {
		req.Header.Set("Authorization", "Bearer "+f.Token)
	}
	resp, err := f.HTTP.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, "", fmt.Errorf("tarball %s: HTTP %d: %s", url, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	limited := io.LimitReader(resp.Body, MaxTarballBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, "", err
	}
	if int64(len(data)) > MaxTarballBytes {
		return nil, "", fmt.Errorf("tarball exceeds %d bytes (likely too large or malicious)", MaxTarballBytes)
	}
	sum := sha256.Sum256(data)
	integrity := "sha256-" + base64.StdEncoding.EncodeToString(sum[:])
	return data, integrity, nil
}

// Extract a gzipped tar archive into dst. Returns the top-level directory
// name created during extraction (GitHub tarballs have a single top-level
// dir like `<owner>-<repo>-<short_sha>`).
func Extract(tarGzData []byte, dst string) (string, error) {
	gz, err := gzip.NewReader(bytes.NewReader(tarGzData))
	if err != nil {
		return "", err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	if err := os.MkdirAll(dst, 0755); err != nil {
		return "", err
	}
	var (
		top       string
		extracted int64
	)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		// Skip PAX metadata entries; they aren't files and would corrupt
		// top-level-dir detection if treated as content.
		if h.Typeflag == tar.TypeXGlobalHeader || h.Typeflag == tar.TypeXHeader {
			continue
		}
		// Path-traversal guard: reject absolute paths and any with "..".
		clean := filepath.Clean(h.Name)
		if filepath.IsAbs(clean) || strings.HasPrefix(clean, "..") || strings.Contains(clean, "/../") {
			return "", fmt.Errorf("tar entry escapes archive root: %q", h.Name)
		}
		// Identify top-level dir as the first path segment of the first
		// real content entry.
		if top == "" {
			parts := strings.SplitN(clean, string(filepath.Separator), 2)
			top = parts[0]
		}
		target := filepath.Join(dst, clean)
		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return "", err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return "", err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(h.Mode)&0777)
			if err != nil {
				return "", err
			}
			limited := io.LimitReader(tr, MaxExtractedBytes-extracted+1)
			n, err := io.Copy(out, limited)
			out.Close()
			if err != nil {
				return "", err
			}
			extracted += n
			if extracted > MaxExtractedBytes {
				return "", fmt.Errorf("extracted size exceeds %d bytes (likely zip bomb)", MaxExtractedBytes)
			}
		case tar.TypeSymlink:
			// Skip symlinks — extra security, fine for skills which shouldn't need them.
			continue
		}
	}
	if top == "" {
		return "", errors.New("empty tarball")
	}
	return filepath.Join(dst, top), nil
}

// GetFile fetches a single file at a specific ref via raw.githubusercontent.com.
// Used for cheap lookups (e.g., registry/index.json) without downloading a full tarball.
func (f *Fetcher) GetFile(ctx context.Context, src *source.Source, ref, path string) ([]byte, error) {
	url := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s", src.Owner, src.Repo, ref, path)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if f.Token != "" {
		req.Header.Set("Authorization", "Bearer "+f.Token)
	}
	resp, err := f.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, os.ErrNotExist
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("GET %s: HTTP %d: %s", url, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	limited := io.LimitReader(resp.Body, MaxFileBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > MaxFileBytes {
		return nil, fmt.Errorf("file at %s exceeds %d bytes", path, MaxFileBytes)
	}
	return body, nil
}

func (f *Fetcher) getJSON(ctx context.Context, url string, out interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if f.Token != "" {
		req.Header.Set("Authorization", "Bearer "+f.Token)
	}
	resp, err := f.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("GET %s: HTTP %d: %s", url, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
