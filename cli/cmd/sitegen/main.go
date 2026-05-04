// sitegen reads skills/core/*/skill.yaml and registry/index.json from the
// repo root and writes:
//
//   docs/skills.json — machine-readable catalog for downstream tools
//   docs/index.html  — human-facing catalog page served at kaijutsu.dev
//
// Run from the repo root:
//
//   go run ./cli/cmd/sitegen
//
// CI verifies no drift between the committed files and a fresh
// regeneration via .github/workflows/lint-skills.yml.
package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/momentmaker/kaijutsu/cli/internal/registry"
	"github.com/momentmaker/kaijutsu/cli/internal/skill"
)

type catalogEntry struct {
	Name        string   `json:"name"`
	Version     string   `json:"version,omitempty"`
	License     string   `json:"license,omitempty"`
	Description string   `json:"description"`
	Agents      []string `json:"agents,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Source      string   `json:"source"`     // "core" | "third-party"
	Repo        string   `json:"repo"`       // momentmaker/kaijutsu OR addyosmani/agent-skills etc.
	Path        string   `json:"path,omitempty"`
	Homepage    string   `json:"homepage,omitempty"`
	Upstream    string   `json:"upstream,omitempty"`
	Author      string   `json:"author,omitempty"`
	HasHooks    bool     `json:"has_hooks"`
	Permissions struct {
		Bash    bool        `json:"bash"`
		Network bool        `json:"network"`
		FsWrite interface{} `json:"fs_write"`
	} `json:"permissions"`
}

type catalog struct {
	// SchemaVersion is the catalog format version. Bump on breaking
	// changes; downstream consumers can branch on it.
	SchemaVersion int `json:"schema_version"`
	// Total skills in the catalog.
	Total int `json:"total"`
	// Skills is the sorted list of every skill known at generation time
	// (skills/core, skills/community, registry/index.json third-party).
	Skills []catalogEntry `json:"skills"`
}

func main() {
	repoRoot := "."
	if len(os.Args) > 1 {
		repoRoot = os.Args[1]
	}
	if err := run(repoRoot); err != nil {
		fmt.Fprintln(os.Stderr, "sitegen:", err)
		os.Exit(1)
	}
}

func run(repoRoot string) error {
	cat, err := buildCatalog(repoRoot)
	if err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(repoRoot, "docs", "skills.json"), cat); err != nil {
		return err
	}
	if err := writeHTML(filepath.Join(repoRoot, "docs", "index.html"), cat); err != nil {
		return err
	}
	fmt.Printf("sitegen: %d skills written to docs/skills.json + docs/index.html\n", cat.Total)
	return nil
}

func buildCatalog(repoRoot string) (*catalog, error) {
	cat := &catalog{
		SchemaVersion: 1,
		Skills:        []catalogEntry{},
	}
	for _, sub := range []string{"core", "community"} {
		dir := filepath.Join(repoRoot, "skills", sub)
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("read %s: %w", dir, err)
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			skillDir := filepath.Join(dir, e.Name())
			yamlPath := filepath.Join(skillDir, "skill.yaml")
			if _, err := os.Stat(yamlPath); err != nil {
				continue
			}
			sk, err := skill.Load(yamlPath)
			if err != nil {
				return nil, fmt.Errorf("load %s: %w", yamlPath, err)
			}
			cat.Skills = append(cat.Skills, fromSkill(sk, sub, "momentmaker/kaijutsu", "skills/"+sub+"/"+sk.Name))
		}
	}

	// Third-party entries from registry/index.json
	idxPath := filepath.Join(repoRoot, "registry", "index.json")
	if _, err := os.Stat(idxPath); err == nil {
		idx, err := registry.Load(idxPath)
		if err != nil {
			return nil, fmt.Errorf("load %s: %w", idxPath, err)
		}
		for name, entry := range idx.Skills {
			cat.Skills = append(cat.Skills, catalogEntry{
				Name:        name,
				Description: fmt.Sprintf("Third-party skill from %s; install via `jutsu install %s`.", entry.Source, name),
				Source:      "third-party",
				Repo:        entry.Source,
				Path:        entry.Path,
				Agents:      []string{"claude", "codex", "gemini"},
			})
		}
	}

	sort.Slice(cat.Skills, func(i, j int) bool {
		return cat.Skills[i].Name < cat.Skills[j].Name
	})
	cat.Total = len(cat.Skills)
	return cat, nil
}

func fromSkill(sk *skill.Skill, source, repo, path string) catalogEntry {
	e := catalogEntry{
		Name:        sk.Name,
		Version:     sk.Version,
		License:     sk.License,
		Description: sk.Description,
		Agents:      sk.Agents,
		Tags:        sk.Tags,
		Source:      source,
		Repo:        repo,
		Path:        path,
		Homepage:    sk.Homepage,
		Upstream:    sk.Upstream,
		Author:      sk.Author,
		HasHooks:    len(sk.Hooks) > 0,
	}
	e.Permissions.Bash = sk.Permissions.Bash
	e.Permissions.Network = sk.Permissions.Network
	e.Permissions.FsWrite = sk.Permissions.FsWrite
	return e
}

func writeJSON(path string, cat *catalog) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cat, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0644)
}

func writeHTML(path string, cat *catalog) error {
	tmpl, err := template.New("page").Funcs(template.FuncMap{
		"join": strings.Join,
		"upper": strings.ToUpper,
	}).Parse(htmlTemplate)
	if err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return tmpl.Execute(f, cat)
}

const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>kaijutsu — open skills for AI coding agents</title>
  <meta name="description" content="MIT-licensed, agent-agnostic registry and CLI for AI coding agent skills. One source of truth for Claude, Codex, and Gemini.">
  <link rel="icon" type="image/x-icon" href="/favicon.ico">
  <link rel="icon" type="image/svg+xml" href="/favicon.svg">
  <link rel="icon" type="image/png" sizes="96x96" href="/favicon-96x96.png">
  <link rel="apple-touch-icon" sizes="180x180" href="/apple-touch-icon.png">
  <link rel="manifest" href="/site.webmanifest">
  <meta name="theme-color" content="#3DDC97">
  <style>
    :root {
      --mint: #3DDC97;
      --mint-dark: #2BB37C;
      --rust: #C18450;
      --ink: #1A1A1A;
      --paper: #F5EDE0;
      --bg: #FFFFFF;
      --border: #E5E5E5;
      --muted: #6B6B6B;
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      font-family: ui-sans-serif, system-ui, -apple-system, "Helvetica Neue", sans-serif;
      background: var(--bg);
      color: var(--ink);
      line-height: 1.55;
    }
    a { color: var(--mint-dark); text-decoration: none; }
    a:hover { text-decoration: underline; }
    .container { max-width: 1080px; margin: 0 auto; padding: 32px 20px; }
    header {
      text-align: center;
      padding: 48px 20px 32px;
      background: linear-gradient(180deg, rgba(61, 220, 151, 0.08), transparent);
    }
    header img { width: 160px; height: 160px; }
    h1 {
      font-size: clamp(32px, 5vw, 56px);
      margin: 16px 0 8px;
      letter-spacing: -0.02em;
    }
    .tagline {
      font-size: 18px;
      color: var(--muted);
      margin: 0 auto 24px;
      max-width: 680px;
    }
    .install {
      display: inline-flex;
      gap: 8px;
      align-items: center;
      background: var(--ink);
      color: #fff;
      padding: 10px 16px;
      border-radius: 8px;
      font-family: ui-monospace, SFMono-Regular, "SF Mono", Menlo, monospace;
      font-size: 14px;
      cursor: pointer;
      user-select: text;
    }
    .install code { color: var(--mint); }
    .install button {
      background: var(--mint);
      color: var(--ink);
      border: none;
      padding: 6px 12px;
      border-radius: 4px;
      font-weight: 600;
      cursor: pointer;
      font-size: 12px;
    }
    .quick-links {
      margin-top: 16px;
      font-size: 14px;
      color: var(--muted);
    }
    .quick-links a { margin: 0 6px; }
    .controls {
      display: flex;
      gap: 12px;
      flex-wrap: wrap;
      margin-bottom: 24px;
    }
    .controls input[type="search"] {
      flex: 1 1 280px;
      padding: 10px 14px;
      border: 1px solid var(--border);
      border-radius: 8px;
      font-size: 15px;
    }
    .controls input[type="search"]:focus {
      outline: none;
      border-color: var(--mint);
      box-shadow: 0 0 0 3px rgba(61, 220, 151, 0.2);
    }
    .filter-group { display: flex; gap: 6px; flex-wrap: wrap; }
    .filter {
      padding: 8px 14px;
      border: 1px solid var(--border);
      border-radius: 999px;
      background: #fff;
      font-size: 13px;
      cursor: pointer;
    }
    .filter.active {
      background: var(--ink);
      color: #fff;
      border-color: var(--ink);
    }
    .stats {
      font-size: 13px;
      color: var(--muted);
      margin-bottom: 12px;
    }
    .grid {
      display: grid;
      grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
      gap: 16px;
    }
    .card {
      border: 1px solid var(--border);
      border-radius: 12px;
      padding: 18px 20px;
      background: #fff;
      display: flex;
      flex-direction: column;
      gap: 10px;
      transition: border-color 0.15s, transform 0.15s;
    }
    .card:hover {
      border-color: var(--mint);
      transform: translateY(-1px);
    }
    .card h3 {
      margin: 0;
      font-size: 18px;
      font-family: ui-monospace, SFMono-Regular, "SF Mono", Menlo, monospace;
    }
    .card .desc {
      font-size: 14px;
      color: #333;
      flex: 1;
    }
    .card .row {
      display: flex;
      justify-content: space-between;
      align-items: flex-start;
      gap: 8px;
      font-size: 12px;
      color: var(--muted);
    }
    .card .row h3 { line-height: 1.25; }
    .card .row .badge { margin-top: 4px; }
    .badge {
      display: inline-block;
      padding: 2px 8px;
      border-radius: 999px;
      font-size: 11px;
      font-weight: 600;
      letter-spacing: 0.02em;
      white-space: nowrap;
      flex-shrink: 0;
    }
    .badge-core { background: rgba(61, 220, 151, 0.15); color: var(--mint-dark); }
    .badge-third { background: rgba(193, 132, 80, 0.15); color: var(--rust); }
    .badge-hooks { background: rgba(26, 26, 26, 0.08); color: var(--ink); }
    .agents { color: var(--muted); }
    .card-install {
      font-family: ui-monospace, monospace;
      font-size: 12px;
      background: var(--paper);
      padding: 6px 10px;
      border-radius: 6px;
      cursor: pointer;
      user-select: all;
    }
    footer {
      margin: 64px auto 32px;
      padding: 24px 20px;
      text-align: center;
      color: var(--muted);
      font-size: 13px;
      border-top: 1px solid var(--border);
    }
    footer a { color: var(--ink); }
    .empty {
      text-align: center;
      padding: 48px 20px;
      color: var(--muted);
      font-style: italic;
    }
  </style>
</head>
<body>

<header>
  <img src="/web-app-manifest-192x192.png" alt="kaijutsu mascot">
  <h1>kaijutsu</h1>
  <p class="tagline">Open skills for AI coding agents. Install once, run on Claude Code, Codex, Gemini — all from the same registry.</p>
  <div class="install" onclick="navigator.clipboard.writeText('curl -fsSL https://kaijutsu.dev/install.sh | sh')" title="Click to copy">
    <code>curl -fsSL https://kaijutsu.dev/install.sh | sh</code>
    <button>Copy</button>
  </div>
  <div class="quick-links">
    <a href="https://github.com/momentmaker/kaijutsu">GitHub</a> ·
    <a href="https://github.com/momentmaker/kaijutsu/blob/main/README.md">README</a> ·
    <a href="https://github.com/momentmaker/kaijutsu/blob/main/SCHEMA.md">Schema</a> ·
    <a href="https://github.com/momentmaker/kaijutsu/blob/main/ROADMAP.md">Roadmap</a> ·
    <a href="https://github.com/momentmaker/kaijutsu/blob/main/SECURITY.md">Security</a> ·
    <a href="/skills.json">skills.json</a>
  </div>
</header>

<main class="container">
  <div class="controls">
    <input type="search" id="search" placeholder="Search skills by name, description, tag…" autocomplete="off">
    <div class="filter-group" id="filters">
      <button class="filter active" data-filter="all">All ({{.Total}})</button>
      <button class="filter" data-filter="core">Core</button>
      <button class="filter" data-filter="third-party">Third-party</button>
      <button class="filter" data-filter="hooks">Hooks</button>
    </div>
  </div>
  <p class="stats" id="stats">Showing all {{.Total}} skills.</p>

  <div class="grid" id="grid">
    {{range .Skills}}
    <article class="card" data-source="{{.Source}}" data-hooks="{{.HasHooks}}" data-search="{{.Name}} {{.Description}} {{join .Tags " "}} {{join .Agents " "}}">
      <div class="row">
        <h3>{{.Name}}{{if .Version}}<span style="color:var(--muted);font-weight:normal"> {{.Version}}</span>{{end}}</h3>
        <span class="badge badge-{{if eq .Source "core"}}core{{else}}third{{end}}">{{upper .Source}}</span>
      </div>
      <p class="desc">{{.Description}}</p>
      <div class="row">
        <span class="agents">{{if .Agents}}{{join .Agents " · "}}{{end}}</span>
        {{if .HasHooks}}<span class="badge badge-hooks">HOOKS</span>{{end}}
      </div>
      <div class="card-install" onclick="navigator.clipboard.writeText('jutsu install {{.Name}}')" title="Click to copy">jutsu install {{.Name}}</div>
    </article>
    {{end}}
  </div>
  <p class="empty" id="empty" style="display:none">No skills match your filters.</p>
</main>

<footer>
  <p>kaijutsu is MIT-licensed. The mascot, monster eats unrecoverable shell commands.</p>
  <p>Generated from <a href="https://github.com/momentmaker/kaijutsu/tree/main/skills/core">skills/core/</a> + <a href="https://github.com/momentmaker/kaijutsu/blob/main/registry/index.json">registry/index.json</a>. <a href="/skills.json">JSON catalog</a>.</p>
</footer>

<script>
  const search = document.getElementById('search');
  const filters = document.getElementById('filters');
  const grid = document.getElementById('grid');
  const stats = document.getElementById('stats');
  const empty = document.getElementById('empty');
  const cards = Array.from(grid.children);
  let activeFilter = 'all';

  function render() {
    const q = search.value.trim().toLowerCase();
    let shown = 0;
    for (const c of cards) {
      const matches = !q || c.dataset.search.toLowerCase().includes(q);
      const filter = activeFilter === 'all'
        || (activeFilter === 'core' && c.dataset.source === 'core')
        || (activeFilter === 'third-party' && c.dataset.source === 'third-party')
        || (activeFilter === 'hooks' && c.dataset.hooks === 'true');
      const visible = matches && filter;
      c.style.display = visible ? '' : 'none';
      if (visible) shown++;
    }
    stats.textContent = ` + "`Showing ${shown} of ${cards.length} skills.`" + `;
    empty.style.display = shown === 0 ? '' : 'none';
  }

  search.addEventListener('input', render);
  filters.addEventListener('click', e => {
    if (!e.target.matches('button[data-filter]')) return;
    activeFilter = e.target.dataset.filter;
    for (const b of filters.querySelectorAll('button')) {
      b.classList.toggle('active', b === e.target);
    }
    render();
  });
</script>

</body>
</html>
`
