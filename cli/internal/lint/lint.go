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
	"sort"
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

// TriggerConflict pairs two skills with an overlapping trigger phrase.
type TriggerConflict struct {
	Phrase string
	Skills []string // sorted, len >= 2
}

// CheckTriggerConflicts compares the trigger phrases extracted from
// each skill's description and reports any phrase shared by two or
// more skills. Triggers are slash-command tokens (/foo) and
// double-quoted phrases inside the description string. Use this when
// you're linting a set of skills that will be installed together.
func CheckTriggerConflicts(skills map[string]*skill.Skill) []TriggerConflict {
	owners := map[string]map[string]bool{}
	for name, sk := range skills {
		if sk == nil {
			continue
		}
		for _, p := range ExtractTriggers(sk.Description) {
			if owners[p] == nil {
				owners[p] = map[string]bool{}
			}
			owners[p][name] = true
		}
	}
	var out []TriggerConflict
	for phrase, names := range owners {
		if len(names) < 2 {
			continue
		}
		list := make([]string, 0, len(names))
		for n := range names {
			list = append(list, n)
		}
		sort.Strings(list)
		out = append(out, TriggerConflict{Phrase: phrase, Skills: list})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Phrase < out[j].Phrase })
	return out
}

var (
	// slashCmdRe matches a leading "/" followed by 2+ chars at a word
	// boundary. Group 1 captures what's before the slash (whitespace
	// or start-of-string accept; alphanumeric/underscore/slash reject
	// — those mean we're inside a path/url/comment). Group 3 captures
	// what's right after the slash-cmd token; we reject matches where
	// it's "/" (file path like /tmp/foo) or "-" extending the token.
	slashCmdRe = regexp.MustCompile(`(^|[^A-Za-z0-9_/])(/[a-z][a-z0-9-]+)([/]|[^A-Za-z0-9_/-]|$)`)
	// quotedRe extracts double-quoted phrases. We deliberately don't
	// scan single-quoted phrases — English prose has too many
	// apostrophes and contractions ("the user's flow", "don't") that
	// the regex would mis-pair into garbage matches.
	quotedRe = regexp.MustCompile(`"([^"\\]{2,80})"`)
)

// ExtractTriggers pulls slash-command tokens and quoted phrases out
// of a skill description. Phrases are lowercased for comparison.
func ExtractTriggers(description string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(p string) {
		p = strings.ToLower(strings.TrimSpace(p))
		if p == "" || seen[p] {
			return
		}
		seen[p] = true
		out = append(out, p)
	}
	low := strings.ToLower(description)
	for _, m := range slashCmdRe.FindAllStringSubmatch(low, -1) {
		// m[2] is the slash-command itself; m[3] is the next char.
		// m[3] == "/" means we matched the head of a path like
		// /tmp/foo — drop those.
		if m[3] == "/" {
			continue
		}
		add(m[2])
	}
	for _, m := range quotedRe.FindAllStringSubmatch(description, -1) {
		add(m[1])
	}
	return out
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
