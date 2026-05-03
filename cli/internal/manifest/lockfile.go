package manifest

import (
	"encoding/json"
	"fmt"
	"os"
)

// LockEntry pins a single skill to a specific source + ref + content hash.
type LockEntry struct {
	Version   *string `json:"version"`
	Source    string  `json:"source"`
	Ref       string  `json:"ref"`
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

func (lf *Lockfile) Save(path string) error {
	data, err := json.MarshalIndent(lf, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0644)
}
