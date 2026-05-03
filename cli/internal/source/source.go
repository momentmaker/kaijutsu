// Package source parses and normalizes skill source identifiers.
//
// kaijutsu identifies upstream sources by `owner/repo` strings. Stage 3
// supports GitHub only. Inputs accepted by Parse:
//
//   - "owner/repo"
//   - "github.com/owner/repo"
//   - "https://github.com/owner/repo"
//   - "https://github.com/owner/repo.git"
package source

import (
	"errors"
	"fmt"
	"strings"
)

type Source struct {
	Host  string // "github.com"
	Owner string
	Repo  string
}

// Parse normalizes a source identifier. Stage 3 accepts only GitHub sources.
func Parse(s string) (*Source, error) {
	if s == "" {
		return nil, errors.New("empty source")
	}
	x := s
	x = strings.TrimPrefix(x, "https://")
	x = strings.TrimPrefix(x, "http://")
	x = strings.TrimSuffix(x, ".git")
	x = strings.TrimPrefix(x, "github.com/")
	parts := strings.Split(x, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, fmt.Errorf("invalid source %q: expected owner/repo", s)
	}
	return &Source{Host: "github.com", Owner: parts[0], Repo: parts[1]}, nil
}

// String returns the canonical "owner/repo" form used in lockfiles and indexes.
func (s *Source) String() string {
	return s.Owner + "/" + s.Repo
}

// APIBase returns the base URL for the GitHub REST API for this repo.
func (s *Source) APIBase() string {
	return fmt.Sprintf("https://api.github.com/repos/%s/%s", s.Owner, s.Repo)
}

// TarballURL returns the codeload URL for a tarball at ref. codeload returns
// a redirect-free .tar.gz suitable for streaming download.
func (s *Source) TarballURL(ref string) string {
	return fmt.Sprintf("https://codeload.github.com/%s/%s/tar.gz/%s", s.Owner, s.Repo, ref)
}

// HumanURL returns the browser URL for the repo.
func (s *Source) HumanURL() string {
	return fmt.Sprintf("https://github.com/%s/%s", s.Owner, s.Repo)
}
