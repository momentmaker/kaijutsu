package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/momentmaker/kaijutsu/cli/internal/fetch"
	"github.com/momentmaker/kaijutsu/cli/internal/registry"
	"github.com/momentmaker/kaijutsu/cli/internal/skill"
	"github.com/momentmaker/kaijutsu/cli/internal/source"
)

// loaded carries the materialized skill source ready for install.Install.
type loaded struct {
	skill   *skill.Skill
	dir     string // path to skill directory on local filesystem
	source  string // canonical "owner/repo", or "local" for --registry mode
	ref     string // commit SHA for remote, or "local"
	path    string // path inside source repo (e.g., "skills/core/decide")
	version string // semver if available
	tag     string // raw tag name (e.g., "v0.3.0") — used for sig bundle fetch
	hash    string // sha256-... integrity for remote, "" for local
	cleanup func()
}

// loadLocal materializes a skill from a local kaijutsu monorepo checkout.
// Used by `--registry <path>` (Stage 2 dev workflow).
func loadLocal(registryPath, skillName string) (*loaded, error) {
	if err := skill.ValidateName(skillName); err != nil {
		return nil, err
	}
	dir := filepath.Join(registryPath, "skills", "core", skillName)
	yamlPath := filepath.Join(dir, "skill.yaml")
	if _, err := os.Stat(yamlPath); err != nil {
		return nil, fmt.Errorf("skill %q not found in registry at %s", skillName, registryPath)
	}
	sk, err := skill.Load(yamlPath)
	if err != nil {
		return nil, err
	}
	return &loaded{
		skill:   sk,
		dir:     dir,
		source:  "local",
		ref:     "local",
		path:    filepath.ToSlash(filepath.Join("skills", "core", skillName)),
		version: sk.Version,
		hash:    "",
		cleanup: func() {},
	}, nil
}

// loadRemote fetches + extracts the skill from a GitHub source.
// constraint is a semver range expression ("^1.0", "~1.2", "1.2.3") or empty
// for "latest". If constraint is a literal SHA-like string (40 hex chars),
// it is treated as a pinned ref. Returns *loaded with .cleanup() to call.
func loadRemote(ctx context.Context, stderr io.Writer, fetcher *fetch.Fetcher, defaultRegistry, skillName, constraint string) (*loaded, error) {
	if err := skill.ValidateName(skillName); err != nil {
		return nil, err
	}
	defReg, err := source.Parse(defaultRegistry)
	if err != nil {
		return nil, fmt.Errorf("default registry %q: %w", defaultRegistry, err)
	}

	idx, _ := loadIndex(ctx, fetcher, defReg)
	res, err := registry.Resolve(idx, defaultRegistry, skillName)
	if err != nil {
		return nil, err
	}

	ref, version, tag, err := resolveRef(ctx, fetcher, res.Source, constraint)
	if err != nil {
		return nil, err
	}

	data, integrity, err := fetcher.Tarball(ctx, res.Source, ref)
	if err != nil {
		return nil, err
	}

	tmp, err := os.MkdirTemp("", "kaijutsu-fetch-")
	if err != nil {
		return nil, err
	}
	cleanup := func() { _ = os.RemoveAll(tmp) }

	top, err := fetch.Extract(data, tmp)
	if err != nil {
		cleanup()
		return nil, err
	}

	skillDir := top
	if res.Path != "" {
		skillDir = filepath.Join(top, filepath.FromSlash(res.Path))
	}
	sk, err := loadSkillOrSynthesize(stderr, skillDir, top, skillName, res.Source.String(), ref)
	if err != nil {
		cleanup()
		return nil, err
	}

	return &loaded{
		skill:   sk,
		dir:     skillDir,
		source:  res.Source.String(),
		ref:     ref,
		path:    res.Path,
		version: version,
		tag:     tag,
		hash:    integrity,
		cleanup: cleanup,
	}, nil
}

// loadSkillOrSynthesize tries skill.yaml first; if absent, falls back to
// synthesizing a minimal Skill from the SKILL.md frontmatter (vanilla
// Anthropic Agent Skills compat — used for installing skills from
// collections like addyosmani/agent-skills that ship only SKILL.md).
//
// Synthesized skills get safe defaults (bash=false, network=false,
// fs-write=false, hooks=false) and the repo LICENSE is checked against
// the kaijutsu compatibility allowlist. The caller passes a stderr
// writer so the synthesized-defaults disclaimer is surfaced (synthesis
// is opt-in trust — the user is installing something whose authoring
// repo didn't declare permissions explicitly).
func loadSkillOrSynthesize(stderr io.Writer, skillDir, repoTop, skillName, src, ref string) (*skill.Skill, error) {
	yamlPath := filepath.Join(skillDir, "skill.yaml")
	if _, err := os.Stat(yamlPath); err == nil {
		return skill.Load(yamlPath)
	}
	skillMDPath := filepath.Join(skillDir, "SKILL.md")
	if _, err := os.Stat(skillMDPath); err != nil {
		return nil, fmt.Errorf("neither skill.yaml nor SKILL.md found at %s in %s@%s", skillDir, src, ref)
	}
	licensePath := ""
	for _, candidate := range []string{"LICENSE", "LICENSE.md", "LICENSE.txt"} {
		p := filepath.Join(repoTop, candidate)
		if _, err := os.Stat(p); err == nil {
			licensePath = p
			break
		}
	}
	sk, err := skill.SynthesizeFromSKILLMD(skillMDPath, licensePath)
	if err != nil {
		return nil, err
	}
	if sk.Name != skillName {
		return nil, fmt.Errorf("synthesize: SKILL.md frontmatter name %q does not match install request %q", sk.Name, skillName)
	}
	if err := sk.Validate(); err != nil {
		return nil, fmt.Errorf("synthesize: %w", err)
	}
	if stderr != nil {
		fmt.Fprintf(stderr,
			"note: %s ships only SKILL.md (no kaijutsu skill.yaml). Synthesized minimal manifest with safe defaults: bash=false, network=false, fs-write=false, hooks=false. Inspect the source repo before relying on it for sensitive work.\n",
			sk.Name)
	}
	return sk, nil
}

// loadByLockEntry re-fetches a skill at a previously-pinned ref and verifies
// the integrity against the lockfile. Used by `jutsu install` (no args).
// path is the in-repo path stored in the lockfile; empty means try the
// canonical core layout, then fall back to repo root.
func loadByLockEntry(ctx context.Context, stderr io.Writer, fetcher *fetch.Fetcher, skillName, src, ref, path, expectedIntegrity string) (*loaded, error) {
	if err := skill.ValidateName(skillName); err != nil {
		return nil, err
	}
	parsed, err := source.Parse(src)
	if err != nil {
		return nil, err
	}
	data, integrity, err := fetcher.Tarball(ctx, parsed, ref)
	if err != nil {
		return nil, err
	}
	if expectedIntegrity != "" && integrity != expectedIntegrity {
		return nil, fmt.Errorf("integrity mismatch for %s: lockfile says %s, fetched %s", skillName, expectedIntegrity, integrity)
	}

	tmp, err := os.MkdirTemp("", "kaijutsu-sync-")
	if err != nil {
		return nil, err
	}
	cleanup := func() { _ = os.RemoveAll(tmp) }

	top, err := fetch.Extract(data, tmp)
	if err != nil {
		cleanup()
		return nil, err
	}

	candidates := []string{}
	if path != "" {
		candidates = append(candidates, filepath.Join(top, filepath.FromSlash(path)))
	}
	candidates = append(candidates,
		filepath.Join(top, "skills", "core", skillName),
		top,
	)
	var skillDir string
	for _, c := range candidates {
		if _, err := os.Stat(filepath.Join(c, "skill.yaml")); err == nil {
			skillDir = c
			break
		}
		if _, err := os.Stat(filepath.Join(c, "SKILL.md")); err == nil {
			skillDir = c
			break
		}
	}
	if skillDir == "" {
		cleanup()
		return nil, fmt.Errorf("neither skill.yaml nor SKILL.md found for %s in fetched tarball", skillName)
	}
	sk, err := loadSkillOrSynthesize(stderr, skillDir, top, skillName, parsed.String(), ref)
	if err != nil {
		cleanup()
		return nil, err
	}
	return &loaded{
		skill:   sk,
		dir:     skillDir,
		source:  parsed.String(),
		ref:     ref,
		path:    path,
		version: sk.Version,
		hash:    integrity,
		cleanup: cleanup,
	}, nil
}

// loadIndex tries to fetch registry/index.json from defaultRegistry's
// default branch. Returns nil index without error when absent so callers
// fall back to the core-path resolution.
func loadIndex(ctx context.Context, fetcher *fetch.Fetcher, src *source.Source) (*registry.Index, error) {
	sha, err := fetcher.DefaultBranchSHA(ctx, src)
	if err != nil {
		return nil, err
	}
	body, err := fetcher.GetFile(ctx, src, sha, "registry/index.json")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	return registry.LoadFromBytes(body)
}

// resolveRef picks the commit SHA to install based on a semver constraint
// (or "" for highest). Returns ref (commit SHA), version (cleaned
// semver string), tag (raw tag name like "v0.3.0", needed for sig
// bundle fetch), error.
func resolveRef(ctx context.Context, fetcher *fetch.Fetcher, src *source.Source, constraint string) (ref, version, tag string, err error) {
	tags, err := fetcher.ListTags(ctx, src)
	if err != nil {
		return "", "", "", err
	}
	if len(tags) == 0 {
		// No semver tags — fall back to default branch HEAD.
		sha, err := fetcher.DefaultBranchSHA(ctx, src)
		if err != nil {
			return "", "", "", err
		}
		return sha, "", "", nil
	}

	chosen := ""
	if constraint == "" {
		chosen = tags[0] // tags are pre-sorted highest-first
	} else {
		c, err := semver.NewConstraint(constraint)
		if err != nil {
			return "", "", "", fmt.Errorf("invalid version constraint %q: %w", constraint, err)
		}
		for _, t := range tags {
			v, err := semver.NewVersion(t)
			if err != nil {
				continue
			}
			if c.Check(v) {
				chosen = t
				break
			}
		}
		if chosen == "" {
			return "", "", "", fmt.Errorf("no tag satisfies constraint %q (available: %s)", constraint, strings.Join(tags, ", "))
		}
	}

	sha, err := fetcher.ResolveTagSHA(ctx, src, chosen)
	if err != nil {
		return "", "", "", err
	}
	// `chosen` was selected from ListTags, which only returns semver-parseable
	// tags, so this parse is guaranteed to succeed.
	v, _ := semver.NewVersion(chosen)
	return sha, v.String(), chosen, nil
}

// parseSpec splits "name" or "name@constraint" into its parts.
func parseSpec(spec string) (name, constraint string) {
	if i := strings.Index(spec, "@"); i >= 0 {
		return spec[:i], spec[i+1:]
	}
	return spec, ""
}
