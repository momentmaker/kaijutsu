// sitegen reads skills/core/*/skill.yaml and registry/index.json from the
// repo root and writes:
//
//   docs/skills.json — machine-readable catalog for downstream tools
//   docs/index.html  — human-facing catalog page served at kaijutsu.dev
//
// The landing page (hero, failure-mode demo, quickstart, etc.) is
// generated from docs/landing-content.json — a hand-edited content
// file, mirroring the skills.json data-vs-code separation pattern.
// Editing copy is a JSON edit, not a Go rebuild.
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
	Source      string   `json:"source"` // "core" | "third-party"
	Repo        string   `json:"repo"`   // momentmaker/kaijutsu OR addyosmani/agent-skills etc.
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

// LandingContent is the v0.14.0 hand-edited content schema for the
// kaijutsu.dev landing page. Lives in docs/landing-content.json so a
// copy edit is a JSON edit, not a Go rebuild.
//
// Spec: docs/specs/2026-05-09-landing-page-content.md
// ADR:  docs/decisions/2026-05-09-landing-page-direction.md
type LandingContent struct {
	Version      int               `json:"version"`
	Hero         landingHero       `json:"hero"`
	FailureDemo  landingFailure    `json:"failure_demo"`
	Quickstart   landingQuickstart `json:"quickstart"`
	Portability  landingPortable   `json:"portability"`
	Cost         landingCost       `json:"cost"`
	Trust        landingTrust      `json:"trust"`
	Contributing landingContrib    `json:"contributing"`
}

type landingHero struct {
	Headline        string   `json:"headline"`
	SubCopy         string   `json:"sub_copy"`
	TaglineLines    []string `json:"tagline_lines"`
	InstallOneliner string   `json:"install_oneliner"`
}

type landingFailure struct {
	Intro         string `json:"intro"`
	Outro         string `json:"outro"`
	LeftCaption   string `json:"left_caption"`
	LeftBody      string `json:"left_body"`
	RightCaption  string `json:"right_caption"`
	RightBody     string `json:"right_body"`
	ArtifactLink  string `json:"artifact_link"`
	ArtifactLabel string `json:"artifact_label"`
}

type landingQuickstart struct {
	Intro              string `json:"intro"`
	Step1Label         string `json:"step1_label"`
	Step1Intro         string `json:"step1_intro"`
	UserSaysLabel      string `json:"user_says_label"`
	UserSays           string `json:"user_says"`
	AgentDoesLabel     string `json:"agent_does_label"`
	SnippetInstall     string `json:"snippet_install"`
	Step2Label         string `json:"step2_label"`
	Step2Intro         string `json:"step2_intro"`
	UserSays2          string `json:"user_says_2"`
	SnippetRun         string `json:"snippet_run"`
	SnippetOutputLabel string `json:"snippet_output_label"`
	SnippetOutput      string `json:"snippet_output"`
	MoreLink           string `json:"more_link"`
	ManualNote         string `json:"manual_note"`
}

type landingPortable struct {
	Intro        string `json:"intro"`
	SnippetYaml  string `json:"snippet_yaml"`
	SnippetRun   string `json:"snippet_run"`
	CalloutTitle string `json:"callout_title"`
	CalloutBody  string `json:"callout_body"`
}

type landingCost struct {
	Intro          string      `json:"intro"`
	TableCaption   string      `json:"table_caption"`
	Rows           []landingKV `json:"rows"`
	ObjectionTitle string      `json:"objection_title"`
	ObjectionBody  string      `json:"objection_body"`
}

type landingKV struct {
	Metric string `json:"metric"`
	Value  string `json:"value"`
}

type landingTrust struct {
	Parts          []landingTrustPart `json:"parts"`
	WrapperTitle   string             `json:"wrapper_title"`
	WrapperIntro   string             `json:"wrapper_intro"`
	WrapperInside  string             `json:"wrapper_inside"`
	WrapperOutside []string           `json:"wrapper_outside"`
}

type landingTrustPart struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

type landingContrib struct {
	Intro string `json:"intro"`
	Body  string `json:"body"`
}

// pageData is the union shape the HTML template consumes: catalog
// fields (Skills, Total) plus the landing content blocks.
type pageData struct {
	*catalog
	Landing LandingContent
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
	landing, err := loadLandingContent(filepath.Join(repoRoot, "docs", "landing-content.json"))
	if err != nil {
		return fmt.Errorf("load landing-content.json: %w", err)
	}
	page := &pageData{catalog: cat, Landing: landing}
	if err := writeHTML(filepath.Join(repoRoot, "docs", "index.html"), page); err != nil {
		return err
	}
	fmt.Printf("sitegen: %d skills written to docs/skills.json + docs/index.html\n", cat.Total)
	return nil
}

// loadLandingContent reads docs/landing-content.json. Hard-fails on
// missing or malformed file — a missing landing-content.json would
// otherwise silently render an empty hero, which is worse than a
// fast CI fail.
func loadLandingContent(path string) (LandingContent, error) {
	var lc LandingContent
	data, err := os.ReadFile(path)
	if err != nil {
		return lc, err
	}
	if err := json.Unmarshal(data, &lc); err != nil {
		return lc, fmt.Errorf("parse %s: %w", path, err)
	}
	return lc, nil
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

	// Stable sort with tie-break on Source so duplicate-name entries
	// (e.g. a kaijutsu-core skill that shares a name with a third-
	// party registry pointer) produce deterministic output across
	// runs. Without this, Go map iteration order leaks into the
	// catalog and CI lint diff-checks flap. Per AGENTS.md design
	// principle: deterministic file outputs.
	sort.SliceStable(cat.Skills, func(i, j int) bool {
		if cat.Skills[i].Name != cat.Skills[j].Name {
			return cat.Skills[i].Name < cat.Skills[j].Name
		}
		return cat.Skills[i].Source < cat.Skills[j].Source
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

func writeHTML(path string, page *pageData) error {
	tmpl, err := template.New("page").Funcs(template.FuncMap{
		"join":  strings.Join,
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
	return tmpl.Execute(f, page)
}

const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>kaijutsu — open arts for AI coding agents</title>
  <meta name="description" content="{{.Landing.Hero.SubCopy}}">
  <meta property="og:title" content="kaijutsu — {{.Landing.Hero.Headline}}">
  <meta property="og:description" content="{{.Landing.Hero.SubCopy}}">
  <meta property="og:type" content="website">
  <meta property="og:url" content="https://kaijutsu.dev">
  <link rel="icon" type="image/x-icon" href="/favicon.ico">
  <link rel="icon" type="image/svg+xml" href="/favicon.svg">
  <link rel="icon" type="image/png" sizes="96x96" href="/favicon-96x96.png">
  <link rel="apple-touch-icon" sizes="180x180" href="/apple-touch-icon.png">
  <link rel="manifest" href="/site.webmanifest">
  <meta name="theme-color" content="#100D09" media="(prefers-color-scheme: dark)">
  <meta name="theme-color" content="#FFFFFF" media="(prefers-color-scheme: light)">
  <script>
    /* Apply persisted theme BEFORE first paint to avoid flash. Three values:
       null/"auto" = follow system; "light" = force light; "dark" = force dark.
       Stored in localStorage under "kaijutsu-theme". */
    (function () {
      try {
        var t = localStorage.getItem("kaijutsu-theme");
        if (t === "light" || t === "dark") {
          document.documentElement.setAttribute("data-theme", t);
        }
      } catch (e) { /* localStorage blocked — fall through to prefers-color-scheme */ }
    })();
  </script>
  <style>
    /* light mode (default) */
    :root {
      --ink: #1A1A1A;
      --ink-soft: #2E2E2E;
      --paper: #F5EDE0;
      --bg: #FFFFFF;
      --bg-soft: #FAF6EE;
      --muted: #6B6B6B;
      --border: #ECE5D6;
      --mint: #3DDC97;
      --mint-dark: #2BB37C;
      --rust: #C18450;
      --rust-soft: #C18450;
      --code-bg: #1A1A1A;
      --code-fg: #F5EDE0;
      --diff-add: #3DDC97;
      --diff-rem: #E0A574;

      /* spacing scale (after pilgrim) */
      --space-xs: 0.25rem;
      --space-sm: 0.5rem;
      --space-md: 1rem;
      --space-lg: 1.25rem;
      --space-xl: 1.75rem;
      --space-2xl: 2.25rem;

      --content-width: 760px;
      --transition-colors: color 0.25s ease, background-color 0.25s ease, border-color 0.25s ease;
    }
    /* dark mode — toggled via prefers-color-scheme OR [data-theme="dark"] override */
    @media (prefers-color-scheme: dark) {
      :root:not([data-theme="light"]) {
        --ink: #F5EDE0;
        --ink-soft: #DCD4C4;
        --paper: #221D17;
        --bg: #100D09;
        --bg-soft: #1A1612;
        --muted: #8A8278;
        --border: #2C2620;
        --mint: #3DDC97;
        --mint-dark: #5BE3A8;
        --rust: #E0A574;
        --rust-soft: #C18450;
        --code-bg: #050402;
        --code-fg: #F5EDE0;
        --diff-add: #3DDC97;
        --diff-rem: #E0A574;
      }
    }
    /* explicit dark via toggle */
    :root[data-theme="dark"] {
      --ink: #F5EDE0;
      --ink-soft: #DCD4C4;
      --paper: #221D17;
      --bg: #100D09;
      --bg-soft: #1A1612;
      --muted: #8A8278;
      --border: #2C2620;
      --mint: #3DDC97;
      --mint-dark: #5BE3A8;
      --rust: #E0A574;
      --rust-soft: #C18450;
      --code-bg: #050402;
      --code-fg: #F5EDE0;
      --diff-add: #3DDC97;
      --diff-rem: #E0A574;
    }
    * { box-sizing: border-box; }
    html { scroll-behavior: smooth; }
    body {
      margin: 0;
      font-family: ui-sans-serif, system-ui, -apple-system, "Helvetica Neue", sans-serif;
      background: var(--bg);
      color: var(--ink);
      line-height: 1.6;
      -webkit-font-smoothing: antialiased;
      transition: var(--transition-colors);
    }
    section, header, footer, .demo-card, .trust-card, .wrapper-defense, .callout, .objection, .install-box, .card, .filter, pre, .topnav {
      transition: var(--transition-colors);
    }

    /* theme toggle (top-right) — circular icon button. Glyphs:
       ○ = light, ● = dark, ◐ = auto/system. The kanji 術 (skill) lives in the
       hero; the moon-phase glyphs feel of-a-piece without being mascot-y. */
    .theme-toggle {
      position: fixed;
      top: 16px;
      right: 16px;
      z-index: 50;
      width: 36px;
      height: 36px;
      background: var(--bg-soft);
      border: 1px solid var(--border);
      color: var(--ink-soft);
      border-radius: 50%;
      padding: 0;
      font-size: 17px;
      line-height: 1;
      cursor: pointer;
      display: inline-flex;
      align-items: center;
      justify-content: center;
      transition: var(--transition-colors), transform 0.15s;
    }
    .theme-toggle:hover { color: var(--mint-dark); border-color: var(--mint); transform: translateY(-1px); }
    .theme-toggle:focus-visible { outline: 2px solid var(--mint); outline-offset: 2px; }
    a { color: var(--mint-dark); text-decoration: none; }
    a:hover { text-decoration: underline; }
    code, pre, kbd, samp { font-family: ui-monospace, SFMono-Regular, "SF Mono", Menlo, monospace; }
    pre {
      background: var(--code-bg);
      color: var(--code-fg);
      padding: 16px 18px;
      border-radius: 8px;
      overflow-x: auto;
      font-size: 13px;
      line-height: 1.55;
      margin: 16px 0;
    }
    pre code { color: inherit; }
    .container { max-width: 1080px; margin: 0 auto; padding: 0 20px; }
    .container > * { max-width: var(--content-width); }
    .container > .grid, .container > .demo-grid, .container > .trust-grid, .container > .catalog-controls { max-width: none; }

    /* hero — soft mint gradient anchors the page; carries the "open + monster + improvement" trinity */
    .hero {
      text-align: center;
      padding: var(--space-2xl) 20px var(--space-xl);
      background: linear-gradient(180deg, rgba(61, 220, 151, 0.10), transparent 80%);
    }
    .hero-mascot {
      width: 120px;
      height: 120px;
      margin: 0 auto 4px;
      display: block;
    }
    .hero-kanji {
      font-size: 22px;
      color: var(--rust);
      opacity: 0.6;
      letter-spacing: 0.08em;
      font-family: "Hiragino Mincho ProN", "Yu Mincho", serif;
    }
    .hero h1 {
      font-size: clamp(30px, 4.8vw, 52px);
      margin: 14px auto 14px;
      letter-spacing: -0.02em;
      line-height: 1.12;
      max-width: 20ch;
      font-weight: 700;
    }
    .hero .subcopy {
      font-size: clamp(15px, 1.6vw, 18px);
      color: var(--ink-soft);
      margin: 0 auto 14px;
      max-width: 58ch;
    }
    .hero .tagline {
      list-style: none;
      padding: 0;
      margin: 0 auto var(--space-xl);
      font-size: 13px;
      color: var(--muted);
      max-width: 60ch;
      letter-spacing: 0.01em;
    }
    .hero .tagline li { margin: 4px 0; }
    .hero .tagline li:first-child {
      color: var(--rust);
      font-family: "Hiragino Mincho ProN", "Yu Mincho", serif;
      font-size: 14px;
    }
    .install-box {
      display: inline-flex;
      gap: 10px;
      align-items: center;
      background: var(--code-bg);
      color: var(--code-fg);
      padding: 12px 16px;
      border-radius: 10px;
      font-family: ui-monospace, monospace;
      font-size: 14px;
      cursor: pointer;
      max-width: 100%;
      overflow-x: auto;
    }
    .install-box code { color: var(--mint); }
    .install-box .copy-hint {
      background: var(--mint);
      color: #1A1A1A;
      border: none;
      padding: 4px 10px;
      border-radius: 4px;
      font-weight: 600;
      font-size: 11px;
      letter-spacing: 0.04em;
      text-transform: uppercase;
      flex-shrink: 0;
    }
    .quick-links {
      margin-top: 18px;
      font-size: 13px;
      color: var(--muted);
      display: flex;
      gap: 14px;
      justify-content: center;
      flex-wrap: wrap;
    }
    .quick-links a { color: var(--muted); text-decoration: none; }
    .quick-links a:hover { color: var(--ink); text-decoration: underline; }

    /* sections — visual rhythm via spacing, no hard borders. Symmetric padding
       gives roughly 9rem between sections (4.5rem bottom of N + 4.5rem top of
       N+1) — generous breathing room without hard dividers.
       Higher specificity (section.container) needed because plain section
       loses to .container on specificity. */
    section.container { padding: var(--space-2xl) 20px; }
    section.container:first-of-type { padding-top: var(--space-xl); }
    section h2 {
      font-size: clamp(22px, 2.6vw, 28px);
      margin: 0 0 var(--space-md);
      letter-spacing: -0.01em;
    }
    section h3 { font-size: 17px; margin: var(--space-lg) 0 var(--space-sm); }
    section p { margin: var(--space-md) 0; max-width: 68ch; }
    section ul { max-width: 68ch; }
    section li { margin-bottom: 6px; }

    /* failure demo */
    .demo-grid {
      display: grid;
      grid-template-columns: 1fr 1fr;
      gap: 16px;
      margin: 24px 0 12px;
    }
    @media (max-width: 768px) { .demo-grid { grid-template-columns: 1fr; } }
    .demo-card {
      border: 1px solid var(--border);
      border-radius: 10px;
      padding: 18px 20px;
      background: var(--bg-soft);
    }
    .demo-card.disagree { border-color: var(--mint); }
    .demo-card .label {
      font-size: 11px;
      font-weight: 600;
      text-transform: uppercase;
      letter-spacing: 0.06em;
      color: var(--muted);
      margin-bottom: 8px;
    }
    .demo-card.disagree .label { color: var(--mint-dark); }
    .demo-card .body { font-size: 14px; line-height: 1.6; color: var(--ink-soft); }
    .demo-card .body.code {
      font-family: ui-monospace, SFMono-Regular, "SF Mono", Menlo, monospace;
      font-size: 12px;
      white-space: pre-wrap;
      word-break: break-word;
      background: var(--code-bg);
      color: var(--code-fg);
      padding: 12px 14px;
      border-radius: 6px;
      margin-top: 8px;
      line-height: 1.5;
    }
    .demo-card .body.code .add { color: var(--diff-add); }
    .demo-card .body.code .rem { color: var(--diff-rem); }
    .demo-caption { font-size: 13px; color: var(--muted); margin-top: 6px; }

    /* quickstart — agent-first prompt card */
    .prompt-card {
      background: var(--bg-soft);
      border: 1px solid var(--border);
      border-left: 3px solid var(--mint);
      border-radius: 0 10px 10px 0;
      padding: 14px 18px;
      margin: var(--space-md) 0;
      max-width: 68ch;
    }
    .prompt-card .label {
      font-size: 11px;
      font-weight: 600;
      text-transform: uppercase;
      letter-spacing: 0.06em;
      color: var(--mint-dark);
      margin-bottom: 6px;
    }
    .prompt-card .prompt-body {
      font-size: 15px;
      color: var(--ink);
      font-style: italic;
      line-height: 1.5;
    }
    .agent-does {
      font-size: 12px;
      text-transform: uppercase;
      letter-spacing: 0.08em;
      color: var(--muted);
      margin: var(--space-md) 0 var(--space-xs);
    }
    .manual-note {
      font-size: 13px;
      color: var(--muted);
      font-style: italic;
      margin: var(--space-md) 0 var(--space-sm);
      max-width: 68ch;
    }

    /* portability callout */
    .callout {
      background: var(--bg-soft);
      border-left: 3px solid var(--mint);
      padding: 16px 20px;
      margin: 20px 0;
      border-radius: 0 8px 8px 0;
    }
    .callout strong { color: var(--ink); }

    /* cost table */
    .cost-table {
      border-collapse: collapse;
      width: 100%;
      max-width: 600px;
      margin: 16px 0;
      font-size: 14px;
    }
    .cost-table th, .cost-table td {
      text-align: left;
      padding: 10px 14px;
      border-bottom: 1px solid var(--border);
    }
    .cost-table th { color: var(--muted); font-weight: 500; font-size: 12px; text-transform: uppercase; letter-spacing: 0.05em; }
    .cost-table td.value { font-family: ui-monospace, monospace; }
    .objection {
      background: var(--paper);
      padding: 16px 20px;
      border-radius: 8px;
      margin: 20px 0;
    }
    .objection .q { font-weight: 600; margin-bottom: 8px; }

    /* trust parts */
    .trust-grid {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
      gap: 16px;
      margin: 20px 0 28px;
    }
    .trust-card {
      border: 1px solid var(--border);
      border-radius: 10px;
      padding: 16px 18px;
      background: var(--bg-soft);
    }
    .trust-card h3 { margin: 0 0 6px; font-size: 15px; }
    .trust-card p { font-size: 14px; color: var(--ink-soft); margin: 0; }
    .wrapper-defense {
      background: var(--bg-soft);
      border: 1px solid var(--border);
      border-radius: 10px;
      padding: 20px 22px;
      margin-top: 16px;
    }
    .wrapper-defense h3 { margin-top: 0; }
    .wrapper-defense .pair { display: grid; grid-template-columns: max-content 1fr; gap: 8px 16px; margin-top: 12px; }
    .wrapper-defense .pair dt { font-weight: 600; color: var(--rust); }

    /* catalog */
    .catalog-controls {
      display: flex;
      gap: 12px;
      flex-wrap: wrap;
      margin-bottom: 18px;
    }
    .catalog-controls input[type="search"] {
      flex: 1 1 280px;
      padding: 10px 14px;
      border: 1px solid var(--border);
      border-radius: 8px;
      font-size: 14px;
      background: var(--bg-soft);
    }
    .catalog-controls input[type="search"]:focus {
      outline: none;
      border-color: var(--mint);
      box-shadow: 0 0 0 3px rgba(61, 220, 151, 0.18);
    }
    .filter-group { display: flex; gap: 6px; flex-wrap: wrap; }
    .filter {
      padding: 8px 14px;
      border: 1px solid var(--border);
      border-radius: 999px;
      background: var(--bg-soft);
      font-size: 13px;
      cursor: pointer;
      color: var(--ink-soft);
    }
    .filter:hover { border-color: var(--mint); }
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
      grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
      gap: 14px;
    }
    .card {
      border: 1px solid var(--border);
      border-radius: 10px;
      padding: 16px 18px;
      background: var(--bg-soft);
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
      font-size: 16px;
      font-family: ui-monospace, SFMono-Regular, "SF Mono", Menlo, monospace;
      line-height: 1.3;
    }
    .card .desc { font-size: 13px; color: var(--ink-soft); flex: 1; }
    .card .row {
      display: flex;
      justify-content: space-between;
      align-items: flex-start;
      gap: 8px;
      font-size: 12px;
      color: var(--muted);
    }
    .badge {
      display: inline-block;
      padding: 2px 8px;
      border-radius: 999px;
      font-size: 10px;
      font-weight: 600;
      letter-spacing: 0.04em;
      white-space: nowrap;
      flex-shrink: 0;
      text-transform: uppercase;
    }
    .badge-core { background: rgba(61, 220, 151, 0.15); color: var(--mint-dark); }
    .badge-third { background: rgba(193, 132, 80, 0.15); color: var(--rust); }
    .badge-hooks { background: rgba(26, 26, 26, 0.08); color: var(--ink); }
    .agents { color: var(--muted); }
    .card-install {
      font-family: ui-monospace, monospace;
      font-size: 11px;
      background: var(--paper);
      padding: 6px 10px;
      border-radius: 6px;
      cursor: pointer;
      user-select: all;
      color: var(--ink);
    }
    .empty {
      text-align: center;
      padding: 40px 20px;
      color: var(--muted);
      font-style: italic;
    }
    .more-link {
      display: inline-block;
      margin-top: 8px;
      font-size: 13px;
      color: var(--muted);
    }

    footer {
      margin-top: var(--space-2xl);
      padding: var(--space-xl) 20px var(--space-2xl);
      text-align: center;
      color: var(--muted);
      font-size: 13px;
      background: var(--paper);
      border-top: 1px solid var(--border);
    }
    footer p { margin: 6px 0; }
    footer a { color: var(--ink-soft); }
    .etymology {
      max-width: 640px;
      margin: 0 auto var(--space-md);
      font-size: 13px;
    }
    .etymology summary {
      cursor: pointer;
      color: var(--rust);
      font-family: "Hiragino Mincho ProN", "Yu Mincho", serif;
      font-size: 14px;
      letter-spacing: 0.02em;
      list-style: none;
      padding: 6px 0;
      text-align: center;
    }
    .etymology summary::-webkit-details-marker { display: none; }
    .etymology summary::before { content: "術 "; opacity: 0.55; }
    .etymology summary:hover { color: var(--rust-soft); }
    .etymology[open] summary { margin-bottom: var(--space-sm); }
    .etymology dl { margin: 0 auto; padding: 0 var(--space-md); text-align: left; max-width: 560px; }
    .etymology dt {
      font-family: "Hiragino Mincho ProN", "Yu Mincho", serif;
      color: var(--rust);
      margin-top: var(--space-sm);
      font-size: 13px;
    }
    .etymology dd { margin: 4px 0 0; padding: 0; color: var(--ink-soft); line-height: 1.55; }
  </style>
</head>
<body id="top">

<button class="theme-toggle" id="theme-toggle" type="button" aria-label="Cycle color theme: auto, light, dark"><span aria-hidden="true">◐</span></button>

<header class="hero">
  <img class="hero-mascot" src="/web-app-manifest-192x192.png" alt="kaijutsu chibi monster mascot">
  <div class="hero-kanji" aria-hidden="true">術</div>
  <h1>{{.Landing.Hero.Headline}}</h1>
  <p class="subcopy">{{.Landing.Hero.SubCopy}}</p>
  <ul class="tagline">
    {{range .Landing.Hero.TaglineLines}}<li>{{.}}</li>{{end}}
  </ul>
  <div class="install-box" onclick="navigator.clipboard.writeText('{{.Landing.Hero.InstallOneliner}}')" title="Click to copy">
    <code>{{.Landing.Hero.InstallOneliner}}</code>
    <span class="copy-hint">copy</span>
  </div>
  <div class="quick-links">
    <a href="https://github.com/momentmaker/kaijutsu">GitHub</a>
    <a href="https://github.com/momentmaker/kaijutsu/blob/main/README.md">README</a>
    <a href="https://github.com/momentmaker/kaijutsu/blob/main/SCHEMA.md">Schema</a>
    <a href="https://github.com/momentmaker/kaijutsu/blob/main/ROADMAP.md">Roadmap</a>
    <a href="https://github.com/momentmaker/kaijutsu/blob/main/SECURITY.md">Security</a>
    <a href="/skills.json">skills.json</a>
  </div>
</header>

<main>

<section id="failure-demo" class="container">
  <h2>One agent says yes. Three say no.</h2>
  <p>{{.Landing.FailureDemo.Intro}}</p>
  <div class="demo-grid">
    <div class="demo-card">
      <div class="label">{{.Landing.FailureDemo.LeftCaption}}</div>
      <div class="body">{{.Landing.FailureDemo.LeftBody}}</div>
    </div>
    <div class="demo-card disagree">
      <div class="label">{{.Landing.FailureDemo.RightCaption}}</div>
      <div class="body code">{{.Landing.FailureDemo.RightBody}}</div>
    </div>
  </div>
  <p class="demo-caption"><a href="{{.Landing.FailureDemo.ArtifactLink}}">{{.Landing.FailureDemo.ArtifactLabel}}</a></p>
  <p>{{.Landing.FailureDemo.Outro}}</p>
</section>

<section id="quickstart" class="container">
  <h2>Quickstart — agent-first</h2>
  <p>{{.Landing.Quickstart.Intro}}</p>

  <h3>{{.Landing.Quickstart.Step1Label}}</h3>
  <p>{{.Landing.Quickstart.Step1Intro}}</p>
  <div class="prompt-card">
    <div class="label">{{.Landing.Quickstart.UserSaysLabel}}</div>
    <div class="prompt-body">“{{.Landing.Quickstart.UserSays}}”</div>
  </div>
  <p class="agent-does">{{.Landing.Quickstart.AgentDoesLabel}}</p>
  <pre><code>{{.Landing.Quickstart.SnippetInstall}}</code></pre>

  <h3>{{.Landing.Quickstart.Step2Label}}</h3>
  <p>{{.Landing.Quickstart.Step2Intro}}</p>
  <div class="prompt-card">
    <div class="label">{{.Landing.Quickstart.UserSaysLabel}}</div>
    <div class="prompt-body">“{{.Landing.Quickstart.UserSays2}}”</div>
  </div>
  <p class="agent-does">{{.Landing.Quickstart.AgentDoesLabel}}</p>
  <pre><code>{{.Landing.Quickstart.SnippetRun}}</code></pre>
  <p class="agent-does">{{.Landing.Quickstart.SnippetOutputLabel}}</p>
  <pre><code>{{.Landing.Quickstart.SnippetOutput}}</code></pre>

  <p class="manual-note">{{.Landing.Quickstart.ManualNote}}</p>
  <a class="more-link" href="{{.Landing.Quickstart.MoreLink}}">Full quickstart guide →</a>
</section>

<section id="portability" class="container">
  <h2>One config. Every vendor.</h2>
  <p>{{.Landing.Portability.Intro}}</p>
  <pre><code>{{.Landing.Portability.SnippetYaml}}</code></pre>
  <pre><code>{{.Landing.Portability.SnippetRun}}</code></pre>
  <div class="callout">
    <strong>{{.Landing.Portability.CalloutTitle}}</strong>
    <p>{{.Landing.Portability.CalloutBody}}</p>
  </div>
</section>

<section id="cost" class="container">
  <h2>Cost per bug found</h2>
  <p>{{.Landing.Cost.Intro}}</p>
  <p class="demo-caption">{{.Landing.Cost.TableCaption}}</p>
  <table class="cost-table">
    <thead><tr><th>Metric</th><th>Value</th></tr></thead>
    <tbody>
    {{range .Landing.Cost.Rows}}
      <tr><td>{{.Metric}}</td><td class="value">{{.Value}}</td></tr>
    {{end}}
    </tbody>
  </table>
  <div class="objection">
    <div class="q">{{.Landing.Cost.ObjectionTitle}}</div>
    <div>{{.Landing.Cost.ObjectionBody}}</div>
  </div>
</section>

<section id="trust" class="container">
  <h2>Trust model</h2>
  <p>kaijutsu's trust surface has four parts:</p>
  <div class="trust-grid">
  {{range .Landing.Trust.Parts}}
    <div class="trust-card">
      <h3>{{.Title}}</h3>
      <p>{{.Body}}</p>
    </div>
  {{end}}
  </div>
  <div class="wrapper-defense">
    <h3>{{.Landing.Trust.WrapperTitle}}</h3>
    <p>{{.Landing.Trust.WrapperIntro}}</p>
    <dl class="pair">
      <dt>Wrapper:</dt><dd>{{.Landing.Trust.WrapperInside}}</dd>
      <dt>Not wrapper:</dt><dd>
        <ul style="margin: 0; padding-left: 18px;">
        {{range .Landing.Trust.WrapperOutside}}
          <li>{{.}}</li>
        {{end}}
        </ul>
      </dd>
    </dl>
  </div>
</section>

<section id="catalog" class="container">
  <h2>Skill catalog</h2>
  <p>Browseable list of every skill kaijutsu ships, plus vendored third-party catalogs. Filter by source, search by name or tag.</p>
  <div class="catalog-controls">
    <input type="search" id="search" placeholder="Search skills by name, description, tag…" autocomplete="off" aria-label="Search skills">
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
</section>

<section id="contributing" class="container">
  <h2>Contributing</h2>
  <p>{{.Landing.Contributing.Intro}}</p>
  <p>{{.Landing.Contributing.Body}}</p>
</section>

</main>

<footer>
  <details class="etymology">
    <summary>kaijutsu — three readings</summary>
    <dl>
      <dt>kai (開) + jutsu (術)</dt><dd>open + skill — the primary etymology. MIT-licensed registry of agent skills, install on any provider, no lock-in.</dd>
      <dt>kaiju (怪獣) + jutsu</dt><dd>monster + arts — the chibi mascot, the playful edge. The monster eats unrecoverable shell commands.</dd>
      <dt>kaizen (改善) + jutsu</dt><dd>continuous improvement + arts — adversarial review, per-(provider, persona, preset) precision tracking, dream lens-blindspots, polish loops.</dd>
    </dl>
  </details>
  <p>kaijutsu is MIT-licensed. <a href="https://github.com/momentmaker/kaijutsu/blob/main/LICENSE">LICENSE</a>.</p>
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

  /* theme toggle: auto → light → dark → auto */
  (function () {
    const btn = document.getElementById('theme-toggle');
    if (!btn) return;
    const root = document.documentElement;
    function read() {
      try { return localStorage.getItem('kaijutsu-theme') || 'auto'; } catch (e) { return 'auto'; }
    }
    const glyph = { auto: '◐', light: '○', dark: '●' };
    const labels = {
      auto:  'Theme: auto (follow system) — click for light',
      light: 'Theme: light — click for dark',
      dark:  'Theme: dark — click for auto'
    };
    function paint(state) {
      const span = btn.querySelector('span');
      if (span) span.textContent = glyph[state] || '◐';
      btn.title = labels[state] || labels.auto;
      btn.setAttribute('aria-label', 'Theme: ' + state + ' (click to cycle)');
    }
    function apply(state) {
      if (state === 'auto') {
        root.removeAttribute('data-theme');
        try { localStorage.removeItem('kaijutsu-theme'); } catch (e) {}
      } else {
        root.setAttribute('data-theme', state);
        try { localStorage.setItem('kaijutsu-theme', state); } catch (e) {}
      }
      paint(state);
    }
    paint(read());
    btn.addEventListener('click', () => {
      const cur = read();
      const next = cur === 'auto' ? 'light' : cur === 'light' ? 'dark' : 'auto';
      apply(next);
    });
  })();
</script>

</body>
</html>
`
