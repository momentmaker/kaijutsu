package manifest

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

// LockEntry pins a single skill to a specific source + ref + content hash.
//
// Version is the skill's internal version from its skill.yaml (the
// `version:` field). Tag is the registry tag the skill was resolved
// from (e.g. "v0.3.1"). Ref is the commit SHA the tag points at.
// Together they pin the skill three ways: by skill-author intent
// (Version), by registry release (Tag), and by content (Ref +
// Integrity).
//
// Path is the directory inside the source repo where the skill lives;
// empty means the skill is at the repo root (used by single-skill third-
// party repos). Populated from registry/index.json or set to
// "skills/core/<name>" for the default kaijutsu monorepo.
//
// InstalledAs records why the skill was installed:
//   - "" or "direct" — user installed this skill explicitly
//   - "dep:<parent>" — pulled in to satisfy <parent>'s deps.skills
type LockEntry struct {
	Version     *string `json:"version"`
	Tag         string  `json:"tag,omitempty"`
	Source      string  `json:"source"`
	Ref         string  `json:"ref"`
	Path        string  `json:"path,omitempty"`
	Integrity   string  `json:"integrity"`
	InstalledAs string  `json:"installedAs,omitempty"`
}

// Lockfile mirrors kaijutsu.lock.json.
type Lockfile struct {
	Version int                  `json:"version"`
	Agents  []string             `json:"agents"`
	Skills  map[string]LockEntry `json:"skills"`
}

func NewLockfile(agents []string) *Lockfile {
	return &Lockfile{
		Version: SchemaVersion,
		Agents:  agents,
		Skills:  map[string]LockEntry{},
	}
}

func LoadLockfile(path string) (*Lockfile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var lf Lockfile
	if err := json.Unmarshal(data, &lf); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if lf.Version < 1 {
		return nil, fmt.Errorf("invalid lockfile version %d in %s (must be >= 1)", lf.Version, path)
	}
	if lf.Version > SchemaVersion {
		fmt.Fprintf(os.Stderr, "warning: %s declares schema version %d but this CLI supports up to %d; some fields may be ignored. Consider upgrading jutsu.\n", path, lf.Version, SchemaVersion)
	}
	if lf.Skills == nil {
		lf.Skills = map[string]LockEntry{}
	}
	return &lf, nil
}

// Validate returns an error if the lockfile is structurally invalid.
func (lf *Lockfile) Validate() error {
	if lf.Version != SchemaVersion {
		return errors.New("invalid version")
	}
	for name, entry := range lf.Skills {
		if entry.Source == "" {
			return fmt.Errorf("skill %q: source is required", name)
		}
		if entry.Ref == "" {
			return fmt.Errorf("skill %q: ref is required", name)
		}
	}
	return nil
}

func (lf *Lockfile) Save(path string) error {
	data, err := json.MarshalIndent(lf, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0644)
}
