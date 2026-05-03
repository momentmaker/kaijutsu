// Package lint validates a skill directory against the kaijutsu schema and
// the Anthropic Agent Skills format. Used by `jutsu lint` (the author UX)
// and by the CI lint workflow.
package lint

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/momentmaker/kaijutsu/cli/internal/skill"
)

// CompatibleLicenses are the SPDX identifiers the registry accepts.
// MIT is canonical; the others are accepted on a case-by-case basis with
// attribution requirements respected.
var CompatibleLicenses = map[string]bool{
	"MIT":          true,
	"BSD-2-Clause": true,
	"BSD-3-Clause": true,
	"ISC":          true,
	"Apache-2.0":   true,
}

// Severity classifies an issue.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

// Issue is a single lint finding.
type Issue struct {
	Severity Severity
	Path     string
	Message  string
}

// Result aggregates the issues for a single skill.
type Result struct {
	SkillDir string
	Issues   []Issue
}

// HasErrors reports whether any issue is fatal.
func (r *Result) HasErrors() bool {
	for _, i := range r.Issues {
		if i.Severity == SeverityError {
			return true
		}
	}
	return false
}

// Lint validates a single skill directory. The directory must contain a
// skill.yaml and a SKILL.md (or, for rich layout, the entry under the
// canonical path).
func Lint(skillDir string) (*Result, error) {
	r := &Result{SkillDir: skillDir}

	yamlPath := filepath.Join(skillDir, "skill.yaml")
	sk, err := skill.Load(yamlPath)
	if err != nil {
		return nil, err
	}

	checkLicense(r, sk, yamlPath)
	checkSkillMd(r, skillDir, sk)
	checkRichLayout(r, skillDir, sk)

	return r, nil
}

func checkLicense(r *Result, sk *skill.Skill, yamlPath string) {
	if !CompatibleLicenses[sk.License] {
		r.Issues = append(r.Issues, Issue{
			Severity: SeverityError,
			Path:     yamlPath,
			Message:  fmt.Sprintf("license %q is not in the kaijutsu allowlist (MIT, BSD-2/3, ISC, Apache-2.0)", sk.License),
		})
	}
}

func checkSkillMd(r *Result, skillDir string, sk *skill.Skill) {
	mdPath := filepath.Join(skillDir, "SKILL.md")
	body, err := os.ReadFile(mdPath)
	if err != nil {
		r.Issues = append(r.Issues, Issue{
			Severity: SeverityError,
			Path:     mdPath,
			Message:  "SKILL.md not found",
		})
		return
	}
	fm, fmErr := parseFrontmatter(body)
	if fmErr != nil {
		r.Issues = append(r.Issues, Issue{
			Severity: SeverityError,
			Path:     mdPath,
			Message:  "missing or malformed YAML frontmatter: " + fmErr.Error(),
		})
		return
	}
	if name := fm["name"]; name == "" {
		r.Issues = append(r.Issues, Issue{
			Severity: SeverityError,
			Path:     mdPath,
			Message:  "frontmatter missing required field: name",
		})
	} else if name != sk.Name {
		r.Issues = append(r.Issues, Issue{
			Severity: SeverityError,
			Path:     mdPath,
			Message:  fmt.Sprintf("frontmatter name %q does not match skill.yaml name %q", name, sk.Name),
		})
	}
	if fm["description"] == "" {
		r.Issues = append(r.Issues, Issue{
			Severity: SeverityError,
			Path:     mdPath,
			Message:  "frontmatter missing required field: description",
		})
	}
}

func checkRichLayout(r *Result, skillDir string, sk *skill.Skill) {
	if sk.Layout != "rich" {
		if len(sk.Shared) > 0 {
			r.Issues = append(r.Issues, Issue{
				Severity: SeverityWarning,
				Path:     filepath.Join(skillDir, "skill.yaml"),
				Message:  "shared field has entries but layout is flat — shared is only meaningful for rich layout",
			})
		}
		return
	}
	for _, dir := range sk.Shared {
		full := filepath.Join(skillDir, dir)
		info, err := os.Stat(full)
		if err != nil || !info.IsDir() {
			r.Issues = append(r.Issues, Issue{
				Severity: SeverityWarning,
				Path:     filepath.Join(skillDir, "skill.yaml"),
				Message:  fmt.Sprintf("shared dir %q declared but not present", dir),
			})
		}
	}
}

var frontmatterDelim = regexp.MustCompile(`(?m)^---\s*$`)

// parseFrontmatter extracts the YAML frontmatter block from a Markdown
// file as a flat key/value map. Only top-level scalar fields are returned;
// nested structures aren't needed for lint purposes.
func parseFrontmatter(body []byte) (map[string]string, error) {
	delims := frontmatterDelim.FindAllIndex(body, 2)
	if len(delims) < 2 {
		return nil, errors.New("no opening/closing --- delimiters")
	}
	if delims[0][0] != 0 {
		return nil, errors.New("frontmatter must be the first block")
	}
	yamlBlock := body[delims[0][1]:delims[1][0]]
	out := map[string]string{}
	for _, line := range strings.Split(string(yamlBlock), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		i := strings.Index(line, ":")
		if i < 0 {
			continue
		}
		key := strings.TrimSpace(line[:i])
		value := strings.TrimSpace(line[i+1:])
		value = strings.Trim(value, "\"'")
		out[key] = value
	}
	return out, nil
}
