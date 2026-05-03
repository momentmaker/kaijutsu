# Multi-agent skill installation

How `jutsu` installs skills for the three CLI agents kaijutsu currently targets:
Claude Code, OpenAI Codex CLI, and Google Gemini CLI.

The good news: all three converge on the **Anthropic Agent Skills** layout
(`<name>/SKILL.md` directory with YAML frontmatter), and Codex and Gemini both
explicitly recognize `.agents/skills/` as a cross-tool alias path. kaijutsu can
install one canonical skill folder and have it picked up by every agent with
near-zero per-agent special-casing.

## Claude Code (baseline, already supported)

| Field | Value |
| ----- | ----- |
| Term | "Skill" |
| Global skill dir | `~/.claude/skills/<name>/SKILL.md` |
| Project skill dir | `<project>/.claude/skills/<name>/SKILL.md` |
| Format | Markdown with YAML frontmatter (`name`, `description`) |
| Loading | Auto-discovered on session start |

## OpenAI Codex CLI

| Field | Value | Source |
| ----- | ----- | ------ |
| Term | "Skill" (alongside `AGENTS.md` for persistent instructions) | [openai/codex docs/skills.md](https://github.com/openai/codex/blob/main/docs/skills.md), [developers.openai.com/codex/skills](https://developers.openai.com/codex/skills) |
| Global config dir | `~/.codex/` (a.k.a. `$CODEX_HOME`) | [docs/agents_md.md](https://github.com/openai/codex/blob/main/docs/agents_md.md) |
| Primary user skill dir | `~/.agents/skills/<name>/SKILL.md` | [core-skills/src/loader.rs L291-L298](https://github.com/openai/codex/blob/main/codex-rs/core-skills/src/loader.rs) |
| Legacy user skill dir | `~/.codex/skills/<name>/SKILL.md` (kept for backwards compat) | [loader.rs L283-L289](https://github.com/openai/codex/blob/main/codex-rs/core-skills/src/loader.rs) |
| Project skill dir | `<project>/.agents/skills/<name>/SKILL.md` (and any `.agents/skills` between project root and CWD) | [loader.rs L327-L357](https://github.com/openai/codex/blob/main/codex-rs/core-skills/src/loader.rs) |
| Admin skill dir | `/etc/codex/skills/` | [loader.rs L308-L316](https://github.com/openai/codex/blob/main/codex-rs/core-skills/src/loader.rs) |
| Skill format | Directory `<name>/` with `SKILL.md` + YAML frontmatter (`name`, `description`); optional `scripts/`, `references/`, `assets/`, `agents/openai.yaml` | [developers.openai.com/codex/skills](https://developers.openai.com/codex/skills), [loader.rs L40-L55, L100-L109](https://github.com/openai/codex/blob/main/codex-rs/core-skills/src/loader.rs) |
| Loading | Auto-discovered on session start with progressive disclosure (name+description initially, full body on activation) | [developers.openai.com/codex/skills](https://developers.openai.com/codex/skills) |
| Activation | Implicit (model picks based on `description`) or explicit via `/skills` and `$skill-name` mentions | [developers.openai.com/codex/skills](https://developers.openai.com/codex/skills) |
| Enable/disable knob | `[[skills.config]] path=... enabled=false` in `~/.codex/config.toml` | [developers.openai.com/codex/skills](https://developers.openai.com/codex/skills) |
| AGENTS.md (separate concern) | `~/.codex/AGENTS.md` (global) and `<project>/AGENTS.md` (concatenated walking root → CWD, capped at 32 KiB) | [developers.openai.com/codex/guides/agents-md](https://developers.openai.com/codex/guides/agents-md) |

**Note on `.codex/skills/`:** Codex's own repo ships skills at
`.codex/skills/code-review/SKILL.md` etc. That works because the project-config
layer's `config_folder` joins `SKILLS_DIR_NAME` ([loader.rs L273-L280](https://github.com/openai/codex/blob/main/codex-rs/core-skills/src/loader.rs)),
i.e. `<project>/.codex/skills/` is a project-scoped equivalent of the legacy
`~/.codex/skills/`. Both paths work; `.agents/skills/` is the recommended
forward-looking location.

### kaijutsu install strategy for Codex

- **User scope:** write to `~/.agents/skills/<name>/SKILL.md`. This is also
  what Gemini reads, so a single write covers both agents.
- **Project scope:** write to `<project>/.agents/skills/<name>/SKILL.md`.
- **AGENTS.md:** treat as a *separate* artifact from skills. If kaijutsu ships
  prompt fragments meant to be persistent (not on-demand), append them to
  `~/.codex/AGENTS.md` (or `<project>/AGENTS.md`) inside a fenced delimited
  block, e.g.

  ```markdown
  <!-- kaijutsu:start name=my-skill version=1.2.0 -->
  ...content...
  <!-- kaijutsu:end name=my-skill -->
  ```

  Remove a skill by deleting the matching block by header. The 32 KiB cap means
  kaijutsu should warn if cumulative kaijutsu-managed content exceeds ~16 KiB.
- **No conflicts** with single-file conventions because skills live in their
  own per-name subdirectory.
- **No JSON schema validation:** the YAML frontmatter validator (`loader.rs`
  L40-L55) only requires `name` + `description`; everything else is optional.

## Google Gemini CLI

| Field | Value | Source |
| ----- | ----- | ------ |
| Term | "Agent Skills" (and "Extensions" — extensions can *bundle* skills) | [docs/cli/skills.md](https://github.com/google-gemini/gemini-cli/blob/main/docs/cli/skills.md), [docs/extensions/index.md](https://github.com/google-gemini/gemini-cli/blob/main/docs/extensions/index.md) |
| Global config dir | `~/.gemini/` (constant `GEMINI_DIR = '.gemini'` in [`paths.ts` L13](https://github.com/google-gemini/gemini-cli/blob/main/packages/core/src/utils/paths.ts)) | source code |
| Primary user skill dir | `~/.gemini/skills/<name>/SKILL.md` *or* `~/.agents/skills/<name>/SKILL.md` (alias) | [docs/cli/skills.md](https://github.com/google-gemini/gemini-cli/blob/main/docs/cli/skills.md) |
| Workspace skill dir | `<project>/.gemini/skills/<name>/SKILL.md` *or* `<project>/.agents/skills/<name>/SKILL.md` (alias) | [docs/cli/skills.md](https://github.com/google-gemini/gemini-cli/blob/main/docs/cli/skills.md) |
| Skill format | Same Anthropic Agent Skills standard: directory with `SKILL.md` + frontmatter | [docs/cli/skills.md](https://github.com/google-gemini/gemini-cli/blob/main/docs/cli/skills.md) ("Based on the [Agent Skills](https://agentskills.io) open standard") |
| Extension dir | `~/.gemini/extensions/<name>/` containing `gemini-extension.json` | [`variables.ts`](https://github.com/google-gemini/gemini-cli/blob/main/packages/cli/src/config/extensions/variables.ts) (`EXTENSIONS_DIRECTORY_NAME`), [docs/extensions/reference.md](https://github.com/google-gemini/gemini-cli/blob/main/docs/extensions/reference.md) |
| Extension manifest | `gemini-extension.json` (JSON, schema includes `name`, `version`, `description`, `mcpServers`, `contextFileName`, `excludeTools`, `settings[]`, `themes[]`, etc.) | [docs/extensions/reference.md](https://github.com/google-gemini/gemini-cli/blob/main/docs/extensions/reference.md) |
| Skills inside extensions | `<extension>/skills/<name>/SKILL.md` | [docs/extensions/reference.md](https://github.com/google-gemini/gemini-cli/blob/main/docs/extensions/reference.md) ("Place skill definitions in a `skills/` directory") |
| Loading | Auto-discovered at session start; activation requires user consent prompt the first time the model calls `activate_skill` | [docs/cli/skills.md](https://github.com/google-gemini/gemini-cli/blob/main/docs/cli/skills.md) |
| Precedence | built-in < extension < user (`~/.gemini/skills` or `~/.agents/skills`) < workspace (`.gemini/skills` or `.agents/skills`); within a tier, `.agents/skills/` wins over `.gemini/skills/` | [docs/cli/skills.md](https://github.com/google-gemini/gemini-cli/blob/main/docs/cli/skills.md) |
| CLI install | `gemini skills install <git-url-or-path> [--scope user|workspace]` | [docs/cli/skills.md](https://github.com/google-gemini/gemini-cli/blob/main/docs/cli/skills.md) |

### kaijutsu install strategy for Gemini

- **User scope:** same write as Codex — `~/.agents/skills/<name>/SKILL.md`.
  Gemini explicitly recommends this path "for managing agent-specific
  expertise that remains compatible across different AI tools."
- **Workspace scope:** `<project>/.agents/skills/<name>/SKILL.md`.
- **Do not write to `~/.gemini/extensions/`** unless kaijutsu is explicitly
  packaging a full extension (manifest + MCP server + commands). For pure
  skills, the dedicated `skills/` dir is the right entrypoint and avoids
  having to author and maintain a `gemini-extension.json` manifest.
- **No conflicts:** Gemini namespaces every extension and every skill by its
  own directory; kaijutsu's per-name folders cannot collide with another
  installer.

## Bottom-line install table

| Agent | User location kaijutsu writes | Project location kaijutsu writes |
| ----- | ----------------------------- | -------------------------------- |
| Claude Code | `~/.claude/skills/<name>/SKILL.md` | `<project>/.claude/skills/<name>/SKILL.md` |
| Codex CLI | `~/.agents/skills/<name>/SKILL.md` | `<project>/.agents/skills/<name>/SKILL.md` |
| Gemini CLI | `~/.agents/skills/<name>/SKILL.md` | `<project>/.agents/skills/<name>/SKILL.md` |

A single write to `~/.agents/skills/<name>/` satisfies both Codex and Gemini at
once. Claude is the only outlier and needs its own write under `~/.claude/`.

## Known unknowns

- **Per-project Codex skills below `<repo>/.codex/skills/`:** Codex's loader
  joins the project layer's `config_folder` with `skills/`, so this works in
  practice and is what the openai/codex repo itself uses. Whether the docs
  promise this as stable API (vs. an implementation detail) is unclear from
  the public reference; kaijutsu should prefer `.agents/skills/` for projects.
- **Codex AGENTS.md merging order with override files:** the public
  `developers.openai.com` doc was paraphrased via WebFetch (couldn't be raw-
  fetched in this session), so the exact precedence between
  `AGENTS.override.md`, `AGENTS.md`, and `project_doc_fallback_filenames` was
  not verified line-by-line against source.
- **Gemini extension auto-update of skills:** `gemini extensions update` is
  documented for full extensions, but it's unclear whether `gemini skills
  install <git-url>` clones once-only or also supports `--auto-update`.
- **Codex `[[skills.config]]` schema:** confirmed via the skills doc but the
  TOML key spelling (`enabled` vs. `disabled`) and whether `path` accepts the
  skill *directory* or the `SKILL.md` file specifically is documented only via
  one example; consult [`config_rules.rs`](https://github.com/openai/codex/blob/main/codex-rs/core-skills/src/config_rules.rs)
  before relying on it programmatically.
- **Symlink handling on Windows:** Codex docs mention "supports symlinked
  skill folders" but Windows behavior (developer-mode requirement, junctions)
  is not spelled out. kaijutsu should default to copy-on-install on Windows.

## Sources

- [openai/codex repository](https://github.com/openai/codex)
- [openai/codex `docs/skills.md`](https://github.com/openai/codex/blob/main/docs/skills.md)
- [openai/codex `docs/agents_md.md`](https://github.com/openai/codex/blob/main/docs/agents_md.md)
- [openai/codex `core-skills/src/loader.rs`](https://github.com/openai/codex/blob/main/codex-rs/core-skills/src/loader.rs) (authoritative path constants)
- [developers.openai.com Codex Skills](https://developers.openai.com/codex/skills)
- [developers.openai.com AGENTS.md guide](https://developers.openai.com/codex/guides/agents-md)
- [google-gemini/gemini-cli repository](https://github.com/google-gemini/gemini-cli)
- [google-gemini/gemini-cli `docs/cli/skills.md`](https://github.com/google-gemini/gemini-cli/blob/main/docs/cli/skills.md)
- [google-gemini/gemini-cli `docs/extensions/index.md`](https://github.com/google-gemini/gemini-cli/blob/main/docs/extensions/index.md)
- [google-gemini/gemini-cli `docs/extensions/reference.md`](https://github.com/google-gemini/gemini-cli/blob/main/docs/extensions/reference.md)
- [google-gemini/gemini-cli `packages/cli/src/config/extensions/variables.ts`](https://github.com/google-gemini/gemini-cli/blob/main/packages/cli/src/config/extensions/variables.ts) (authoritative extension dir constants)
- [Agent Skills standard (agentskills.io)](https://agentskills.io)
