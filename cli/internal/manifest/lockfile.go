package manifest

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

// LockEntry pins a single skill to a specific source + ref + content hash.
// Path is the directory inside the source repo where the skill lives;
// empty means the skill is at the repo root (used by single-skill third-party
// repos). Populated from registry/index.json or set to "skills/core/<name>"
// for the default kaijutsu monorepo.
type LockEntry struct {
	Version   *string `json:"version"`
	Source    string  `json:"source"`
	Ref       string  `json:"ref"`
	Path      string  `json:"path,omitempty"`
	Integrity string  `json:"integrity"`
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
	if lf.Version != SchemaVersion {
		return nil, fmt.Errorf("unsupported lockfile version %d", lf.Version)
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
