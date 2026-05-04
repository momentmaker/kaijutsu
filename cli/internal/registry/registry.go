// Package registry handles the kaijutsu third-party index (registry/index.json
// in the canonical kaijutsu monorepo) and resolves skill names to their
// upstream sources.
package registry

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/momentmaker/kaijutsu/cli/internal/source"
)

const SchemaVersion = 1

// IndexEntry maps a skill name to a third-party source repo.
type IndexEntry struct {
	// Source is in the form "owner/repo" or any form Source.Parse accepts.
	Source string `json:"source"`
	// Path is the directory inside the repo where the skill lives.
	// Empty / "." means the skill is at the repo root.
	Path string `json:"path,omitempty"`
}

// Index mirrors registry/index.json in the kaijutsu monorepo.
type Index struct {
	Version int                   `json:"version"`
	Skills  map[string]IndexEntry `json:"skills"`
}

func Load(path string) (*Index, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return LoadFromBytes(data)
}

// LoadFromBytes parses the registry index from raw JSON bytes.
func LoadFromBytes(data []byte) (*Index, error) {
	var idx Index
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, fmt.Errorf("parse registry index: %w", err)
	}
	if idx.Version < 1 {
		return nil, fmt.Errorf("invalid registry version %d (must be >= 1)", idx.Version)
	}
	if idx.Version > SchemaVersion {
		fmt.Fprintf(os.Stderr, "warning: registry index declares schema version %d but this CLI supports up to %d; some fields may be ignored. Consider upgrading jutsu.\n", idx.Version, SchemaVersion)
	}
	if idx.Skills == nil {
		idx.Skills = map[string]IndexEntry{}
	}
	return &idx, nil
}

// Resolution describes where a skill comes from.
type Resolution struct {
	Source *source.Source
	// Path inside the source repo where the skill lives. Empty means root.
	Path string
}

// CoreSkillResolution returns the canonical resolution for a core skill in
// the default kaijutsu monorepo. Used when an index lookup misses.
func CoreSkillResolution(defaultRegistry string, skillName string) (*Resolution, error) {
	src, err := source.Parse(defaultRegistry)
	if err != nil {
		return nil, fmt.Errorf("default registry: %w", err)
	}
	return &Resolution{
		Source: src,
		Path:   filepath.ToSlash(filepath.Join("skills", "core", skillName)),
	}, nil
}

// Resolve looks up a skill name in the index, falling back to the default
// monorepo's core path if not present.
func Resolve(idx *Index, defaultRegistry, skillName string) (*Resolution, error) {
	if idx != nil {
		if entry, ok := idx.Skills[skillName]; ok {
			src, err := source.Parse(entry.Source)
			if err != nil {
				return nil, fmt.Errorf("registry entry %q: %w", skillName, err)
			}
			path := entry.Path
			if path == "" || path == "." {
				path = ""
			}
			return &Resolution{Source: src, Path: path}, nil
		}
	}
	return CoreSkillResolution(defaultRegistry, skillName)
}

// SourceFromManifest extracts the source URL from a manifest's registry
// config. If unset, returns the canonical default.
func SourceFromManifest(registryDefault string) string {
	if registryDefault == "" {
		return "https://github.com/momentmaker/kaijutsu"
	}
	return registryDefault
}

var ErrNoIndex = errors.New("no registry index")
