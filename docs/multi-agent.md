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

## v0.6 — Multi-provider swarm (driver abstraction)

The Skills layout above describes how `jutsu install <skill>` writes to disk
for the three native CLIs. v0.6 expands `jutsu swarm` to dispatch to providers
beyond those three CLIs via a **driver abstraction**. Skills are unchanged;
only swarm's runtime gains new participants.

Spec: [`specs/2026-05-05-v0.6.0-multi-provider-agents.md`](specs/2026-05-05-v0.6.0-multi-provider-agents.md).
ADR: [`decisions/2026-05-05-driver-abstraction.md`](decisions/2026-05-05-driver-abstraction.md).

### Drivers

| Kind | Mechanism | Use case |
|---|---|---|
| `cli` | shell out to native CLI binary | claude / codex / gemini (existing v0.5 behavior) |
| `http` | direct OpenAI-compat or Anthropic-compat HTTP via `net/http` | DeepSeek, GLM, Kimi, local Ollama, any compat endpoint |
| `cli-compat` | wraps a base CLI with `BASE_URL` + `KEY` override + telemetry-kill envs | "claude harness routing through DeepSeek" — opt-in with one-shot warning about metadata leak via undocumented telemetry endpoints |
| `mcp` | JSON-RPC 2.0 over stdio against MCP servers | deterministic peers (semgrep, eslint, custom analyzers) tagged `[deterministic]` in disagreement table; free at the API level |

### HTTP-driver providers

The vendored catalog (`cli/internal/agents/catalog.go`) ships entries for:

- **claude / codex / gemini** — the original three CLIs (driver: `cli`).
- **deepseek** — `https://api.deepseek.com/v1`, `deepseek-coder` model, OpenAI-compat. Set `DEEPSEEK_API_KEY` env.
- **glm** — `https://open.bigmodel.cn/api/paas/v4`, `glm-4.6` model, OpenAI-compat. Set `GLM_API_KEY`.
- **kimi** — `https://api.moonshot.cn/v1`, `moonshot-v1-32k` model, OpenAI-compat. Set `KIMI_API_KEY`.
- **ollama-local** — `http://localhost:11434/v1` (no auth required for default localhost setup), `qwen2.5-coder:14b` model, OpenAI-compat. Free at the API level for local inference.

Add to your project via:

```bash
jutsu agent add deepseek          # writes catalog default to .kaijutsu/agents.yaml
export DEEPSEEK_API_KEY=sk-...
jutsu agent enable deepseek       # adds to the project's enabled list
jutsu agent test deepseek         # /models GET (or 1-token completion fallback)
```

To declare an HTTP provider not in the catalog:

```bash
jutsu agent add my-provider \
    --driver http \
    --protocol openai-compat \
    --base-url https://api.my-provider.com/v1 \
    --model my-model \
    --api-key-env MY_PROVIDER_API_KEY
```

### cli-compat (route claude through deepseek)

Use when you specifically want claude CLI's harness behavior with a different model. Bears a runtime warning about telemetry metadata leakage to the harness vendor (Anthropic in this case).

```yaml
# <repo>/.kaijutsu/agents.yaml
providers:
  deepseek-via-claude:
    driver: cli-compat
    base_cli: claude
    env:
      ANTHROPIC_BASE_URL: https://api.deepseek.com
    env_key:
      ANTHROPIC_API_KEY: DEEPSEEK_API_KEY
```

The `env:` block sets literal env vars; `env_key:` resolves env-var indirection (`ANTHROPIC_API_KEY` is set on the child to the value of the local `DEEPSEEK_API_KEY`). Default telemetry-kill envs (`DISABLE_TELEMETRY=1` etc.) are layered in automatically.

### MCP-as-peer

See [`../cli/internal/agents/mcp_examples.md`](../cli/internal/agents/mcp_examples.md) for the full reference (semgrep-mcp, eslint-mcp, custom analyzer protocol). One YAML block:

```yaml
providers:
  semgrep-mcp:
    driver: mcp
    transport: stdio
    command: npx
    args: ["semgrep-mcp"]
    tool_name: analyze
```

Driver implements the JSON-RPC 2.0 handshake (initialize + notifications/initialized + tools/call). Server's `analyze` tool receives `{preset, severity_vocab, input}` and returns a JSON findings array conforming to the preset's vocabulary. `Result.CostUSD = 0` always (deterministic local execution).

### Personas

A persona is a `(provider, optional model override, system prompt, tags)` tuple — the unit of swarm participant identity. `jutsu agent list --personas` shows the 7 built-ins:

- `default-claude` / `default-codex` / `default-gemini` — empty system prompts; reproduce v0.5 cache-key behavior byte-for-byte.
- `paranoid-security-claude` / `pragmatic-codex` / `architecture-purist-gemini` / `brainstorm-creative-claude` — reference flavored personas demonstrating tag-driven dispatch.

Author your own in `agents.yaml`:

```yaml
personas:
  cautious-claude:
    provider: claude
    system_prompt: |
      Lean conservative. Flag any change that touches auth or session state.
    tags: [security, conservative]
```

Then dispatch via:

```bash
jutsu swarm pr-review --personas cautious-claude,pragmatic-codex,default-gemini
```

System prompts are prepended to the per-agent skill prompt via the sentinel `\n\n<<<USER>>>\n\n` — the HTTP driver splits on this and routes the system half into the protocol's first-class `system` field; the cli driver passes the concatenated prompt through unchanged.

Skills can require persona tags via `requires_persona_tags: [security]` in `skill.yaml`; swarm dispatch hard-fails with a hint if no enabled persona satisfies.

### Cost projection

`jutsu swarm <preset> --estimate` runs without invoking agents — prints a per-persona table of `{input tokens, output estimate, projected cost}` plus TOTAL. Char-count tokenizer (±20% accuracy). Stale rate-card warning at 90+ days. Vendored providers carry rate cards as of `2026-05-06`; refresh via the v0.6.x cron + bench harness work.

## v0.7 — Quality fingerprinting + confidence-weighted synthesizer

After ~10 swarm runs against a given codebase, you start noticing patterns: claude is great on auth, gemini's style nits get ignored, deepseek's perf finds are gold. v0.7 captures that signal automatically. Every finding goes into a local SQLite store; accepting or dismissing a finding feeds per-(provider, persona, preset, codebase) precision math; the synthesizer weights each agent's vote on the next run by its observed precision.

### Where it lives

- **Database**: `~/.kaijutsu/findings.db` — one file per user, mode `0600`, never in any repo. Pure-Go SQLite via `modernc.org/sqlite` (no CGo).
- **Per-row schema**: `(run_id, codebase_fp, preset, provider, persona, severity, file, line_range, summary, reasoning, confidence, created_at, user_action, action_at, action_reason)`. Forward-only migrations under `cli/internal/findings/migrations/NNNN_*.sql`.
- **Codebase fingerprint** is a 16-char hex hash resolved via a 5-step chain: git origin → upstream → alphabetical-first remote → `local-git:` → `local-fs:`. Anchored on `(remote, work-tree-basename)` so identical clones across machines collide; forks cloned to differently-named directories don't.

### Privacy boundary

- **No network**: `cli/internal/cli/finding.go` MUST NOT import `net`, `net/*`, or `golang.org/x/net/*`. Enforced by an import-list test that fails the build on accidental drag-ins.
- **`jutsu finding clear`** wipes targeted rows + runs `VACUUM` to reclaim disk.
- **`jutsu finding export`** writes local JSON only — never POSTs anywhere. Use this for your own backup pipeline.
- **Same-machine boundary**: the DB lives in `$HOME` alongside `agents.yaml`. Multi-user homedir / auto-syncing backup pipelines that surface the file at non-default permissions get an honest "best-effort" boundary, not a guarantee.
- **No telemetry / phone-home in v0.7**. Anonymized aggregate sharing is deferred to v0.8 with its own spec + ADR — every step toward telemetry is irreversible, so we do nothing rather than do it badly.

### Weight algorithm (3 states)

Per `(provider, persona, preset, codebase_fp)` tuple, measured over the most recent 200 actioned findings (`action_at` desc):

| State | Condition | Weight |
|---|---|---|
| Cold-start | DB absent OR `actioned_count == 0` for tuple | **1.0** (byte-identical v0.6.2 behavior) |
| Bootstrap | `1 ≤ actioned < 10` | **0.7** (slight skepticism without dismissing) |
| Mature | `actioned ≥ 10` | **`accepted / (accepted+dismissed)`**, clamped to `[0.05, 1.0]` |

The synthesizer's `clusterFindings` uses `weighted_consensus = sum(unique reporter weights)` as the secondary sort key. A high-precision agent's lone finding can outrank a low-precision chorus. Cold-start collapses byte-for-byte to v0.6 ordering.

### CLI usage

```bash
# Run swarm — DB is created on first run that produces findings.
jutsu swarm pr-review --personas paranoid-security-claude,default-gemini

# Inspect what got recorded for the current codebase.
jutsu finding list                        # most recent run, current cwd's codebase
jutsu finding list --pending              # only unactioned
jutsu finding list --run <cache-key>      # specific swarm run

# Action findings — the cross-codebase guard refuses if your cwd's
# fingerprint doesn't match the finding's; --cross-codebase overrides.
jutsu finding accept 5  --reason "real auth bug"
jutsu finding dismiss 12 --reason "stylistic nit"

# Inspect per-(provider, persona, preset) precision for current codebase.
jutsu finding stats
# PROVIDER  PERSONA                     PRESET     PRECISION  WEIGHT          ACCEPT/DISMISS  PENDING
# claude    paranoid-security-claude    pr-review  0.85       0.85 (mature)   17/3            22
# deepseek  performance-deepseek        pr-review  0.92       0.92 (mature)   12/1            8
# gemini    default-gemini              pr-review  (insuff.)  0.70 (bootstrap) 4/2            14

# Optional: see weights in the swarm output's disagreement-table headers.
jutsu swarm pr-review --personas ... --show-weights

# Housekeeping. Defaults to dry-run.
jutsu finding clear --older-than 365d --yes
jutsu finding export ~/findings-backup.json
```

### Opt-out

There is no `--no-fingerprinting` flag in v0.7. The two opt-outs are:

1. **Don't action findings**: cold-start state (weight = 1.0) makes synthesis byte-identical to v0.6.2. Recording still happens (rows accumulate); they just don't affect weights until you `accept`/`dismiss`.
2. **`jutsu finding clear --all-codebases --yes`**: nukes every recorded finding. The DB schema persists (cheap) but every tuple goes back to cold-start.

A persistent disable will arrive in v0.7.x once we see whether the cold-start state is sufficient (it should be — recording without actioning has zero behavioral effect).

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
