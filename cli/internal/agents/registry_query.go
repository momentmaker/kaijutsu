// registry_query.go — v0.14.0 read-only persona browse surface.
//
// BrowsePersonas merges resolved personas with provenance tracking
// so the cli `jutsu agent persona browse` subcommand can render a
// table with `built-in` / `user:home` / `user:project` source tags.
//
// Spec: docs/specs/2026-05-09-v0.14.0-persona-browse-and-preset-usage.md
package agents

import "sort"

// BrowseFilters narrows the BrowsePersonas result set. Empty fields
// mean "no filter". Multiple set fields combine with AND.
type BrowseFilters struct {
	Tag      string // exact match against persona.Tags; empty = no filter
	Provider string // exact match against persona.Provider
	Source   string // "built-in" | "user:home" | "user:project"
}

// BrowseRow is the per-persona shape returned by BrowsePersonas. The
// cli renderer formats this into TTY table / JSON / paste-yaml.
type BrowseRow struct {
	Name         string   `json:"name"`
	Provider     string   `json:"provider"`
	Tags         []string `json:"tags"`
	Source       string   `json:"source"` // built-in | user:home | user:project
	SystemPrompt string   `json:"system_prompt"`
	Model        string   `json:"model,omitempty"`
}

// BrowsePersonas returns the resolved persona set with source tags
// + filtered by BrowseFilters.
//
// IMPORTANT: shadowing/collision resolution happens upstream in
// `agents.Resolve` (which produces the `Resolved` arg passed in).
// BrowsePersonas itself does NOT resolve collisions — it just
// LABELS each resolved persona with its highest-precedence source.
//
// Source attribution rule (highest precedence first; mirrors
// `agents.Resolve` overlay order: built-ins → home → project):
//
//   - if name in project.Personas → source = "user:project"
//   - else if name in global.Personas → source = "user:home"
//   - else (came from BuiltinPersonas) → source = "built-in"
//
// Built-in shadow case (user agents.yaml redefines a built-in name):
// `Resolve` already replaced the built-in with the user version;
// BrowsePersonas labels with `user:project` or `user:home` so the
// caller can see the override happened.
//
// Returns empty slice if nothing matches; never returns nil.
// Sorted alphabetically by name for stable output.
func BrowsePersonas(resolved *Resolved, global *GlobalConfig, project *ProjectConfig, filters BrowseFilters) []BrowseRow {
	if resolved == nil {
		return []BrowseRow{}
	}
	rows := make([]BrowseRow, 0, len(resolved.Personas))
	for name, p := range resolved.Personas {
		if p == nil {
			continue
		}
		source := personaSource(name, global, project)
		row := BrowseRow{
			Name:         name,
			Provider:     p.Provider,
			Tags:         p.Tags,
			Source:       source,
			SystemPrompt: p.SystemPrompt,
			Model:        p.Model,
		}
		if !browseRowMatches(row, filters) {
			continue
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Name < rows[j].Name })
	return rows
}

// personaSource returns the source tag for a persona name. Project
// wins over home, home wins over built-in (matches Resolve order).
//
// Nil-entry rule: Resolve skips nil persona entries (`foo: ~` in yaml)
// when overlaying user layers, so personaSource must too — otherwise a
// nil project entry would mislabel a built-in or home persona as
// "user:project". Mirrors `Resolve` lines 145-159.
func personaSource(name string, global *GlobalConfig, project *ProjectConfig) string {
	if project != nil {
		if p, ok := project.Personas[name]; ok && p != nil {
			return "user:project"
		}
	}
	if global != nil {
		if p, ok := global.Personas[name]; ok && p != nil {
			return "user:home"
		}
	}
	return "built-in"
}

// browseRowMatches applies the filter set to a row. AND across set
// filter fields; empty field = no constraint.
func browseRowMatches(r BrowseRow, f BrowseFilters) bool {
	if f.Source != "" && r.Source != f.Source {
		return false
	}
	if f.Provider != "" && r.Provider != f.Provider {
		return false
	}
	if f.Tag != "" {
		hasTag := false
		for _, t := range r.Tags {
			if t == f.Tag {
				hasTag = true
				break
			}
		}
		if !hasTag {
			return false
		}
	}
	return true
}
