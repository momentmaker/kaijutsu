<p align="center">
  <img src="./docs/assets/logo-256.png" alt="kaijutsu mascot" width="200" height="200">
</p>

<h1 align="center">kaijutsu</h1>

<p align="center">
  <strong>Write your dev workflow once. Run it on any LLM.</strong><br>
  Open agent skills · multi-model swarm · a local findings store that learns which models you can trust.
</p>

<p align="center">
  <a href="https://kaijutsu.dev">kaijutsu.dev</a> ·
  <a href="./SCHEMA.md">Schema</a> ·
  <a href="./docs/multi-agent.md">Multi-agent</a> ·
  <a href="./ROADMAP.md">Roadmap</a> ·
  <a href="./CHANGELOG.md">Changelog</a>
</p>

<p align="center">
  <a href="https://github.com/momentmaker/kaijutsu/releases/latest"><img src="https://img.shields.io/github/v/release/momentmaker/kaijutsu?display_name=tag&color=3DDC97" alt="Latest release"></a>
  <a href="./LICENSE"><img src="https://img.shields.io/badge/license-MIT-3DDC97.svg" alt="License: MIT"></a>
  <a href="https://github.com/momentmaker/kaijutsu/actions/workflows/lint-skills.yml"><img src="https://github.com/momentmaker/kaijutsu/actions/workflows/lint-skills.yml/badge.svg" alt="Lint skills"></a>
  <a href="https://github.com/momentmaker/kaijutsu/actions/workflows/release-jutsu.yml"><img src="https://github.com/momentmaker/kaijutsu/actions/workflows/release-jutsu.yml/badge.svg" alt="Release"></a>
</p>

```sh
brew install momentmaker/tap/jutsu       # macOS / Linux
curl -fsSL https://kaijutsu.dev/install.sh | sh
jutsu init && jutsu install pr-review    # try it
```

---

## The 30-second pitch

**Skills are markdown.** A kaijutsu skill is one `SKILL.md` with YAML frontmatter — the [Anthropic Agent Skills](https://agentskills.io) open standard. Same file works in Claude Code, Codex CLI, Antigravity CLI, and any agent that reads `.agents/skills/`.

**Models are swappable.** Declare your providers in one `agents.yaml`: native CLIs (Claude / Codex / Antigravity), HTTP endpoints (Anthropic / OpenAI / Google / DeepSeek / Qwen / Kimi / Mistral / any OpenAI-compat), local servers (Ollama / vLLM / llama.cpp), or MCP tool servers. Skills don't change when you swap models.

**Routing learns from itself.** Every `jutsu swarm` run logs findings to a local SQLite store. Each finding's outcome (actioned / dismissed) becomes precision signal — per (provider, persona, codebase) tuple. `jutsu finding precision --recommend` tells you which models to dispatch to next time, on this codebase, for this preset. Your data, your disk, no SaaS.

## The loop

```
   ┌──────────────────────────────────────────────────┐
   │  Author a skill                                  │
   │  (SKILL.md + YAML frontmatter)                   │
   └────────────────────────┬─────────────────────────┘
                            │
                            ▼
   ┌──────────────────────────────────────────────────┐
   │  jutsu swarm pr-review --pr 42                   │
   │    ├─ claude    ──┐                              │
   │    ├─ agy        ──┼─► synthesizer                │
   │    ├─ deepseek  ──┤   + disagreement table       │
   │    └─ your-llm  ──┘                              │
   └────────────────────────┬─────────────────────────┘
                            │
                            ▼
   ┌──────────────────────────────────────────────────┐
   │  findings.db (local SQLite)                      │
   │    scores precision per                          │
   │    (provider, persona, codebase, preset)         │
   └────────────────────────┬─────────────────────────┘
                            │
                            ▼
   ┌──────────────────────────────────────────────────┐
   │  jutsu finding precision --recommend             │
   │    → next dispatch routes to the                 │
   │      (provider, persona) pairs that actually     │
   │      flag issues you act on                      │
   └──────────────────────────────────────────────────┘
```

Three pieces, one feedback loop. No vendor in the path. No telemetry leaving disk.

## Try it

```sh
# 1. Install
brew install momentmaker/tap/jutsu       # or: curl -fsSL https://kaijutsu.dev/install.sh | sh

# 2. Wire your project
cd your-repo
jutsu init                               # detects active agents, writes .kaijutsu/ + AGENTS.md fragment
jutsu install pr-review                  # writes to .claude/skills/ AND .agents/skills/ — one author, every agent

# 3. Run multi-agent review on the current branch
jutsu swarm pr-review --diff-from-branch main --post-comment
```

Want the agent to do the setup for you instead? Paste this into Claude / Codex / Antigravity:

> "Set up kaijutsu from https://kaijutsu.dev in this repo. Run `jutsu init`, install `pr-review`, and tell me which API keys I still need."

After `jutsu init`, future agent sessions read the AGENTS.md fragment and already know the tool.

## What's in the box

### A skill registry (28+ skills, MIT)

Browse at **[kaijutsu.dev](https://kaijutsu.dev)**. Three categories:

- **Workflow** — `pr-review`, `doc-review`, `polish`, `dream`, `spec-driven-development`, `planning-and-task-breakdown`, `autopilot`, `editorial-review`, `journal`, `readme-update`, `session-retro`, `scope-check`
- **Primitives** — `blunder-hunt`, `convergence-detect`, `dispatch-parallel`, `deslop`, `verification-before-completion`, `lie-to-them`, `multi-model-synth`, `project-memory`
- **Domain** — `security-and-hardening`, `incremental-implementation`, `code-simplification`, `refactor-plan`, `security-audit`, `dcg`, `unstuck`

```sh
jutsu search "review"          # discovery
jutsu suggest "I want to find security holes in this PR"
jutsu info pr-review           # full metadata + agent compat + signed status
jutsu install pr-review        # writes .claude/skills/pr-review + .agents/skills/pr-review
```

### A multi-agent swarm (10 presets)

```sh
jutsu swarm pr-review --pr 42                          # adversarial code review
jutsu swarm doc-review SPEC.md                         # spec / plan / RFC review
jutsu swarm brainstorm "how should we ship X?"         # orthogonal-angle ideation
jutsu swarm refactor-plan src/auth/*.go --goal "..."   # risk-ranked refactor steps
jutsu swarm security-audit --pr 42                     # CVSS-aligned threat model
jutsu swarm dream "should we build X?"                 # 8-lens pre-implementation interrogation
jutsu swarm reverse --spec SPEC.md --diff main..HEAD   # spec-vs-impl drift detector
jutsu swarm bug-repro "intermittent 500 on POST /foo"  # ranked repro hypotheses + minimal repro
jutsu swarm code-archaeology path/to/legacy.go         # competing historical-context theories
jutsu swarm test-gap path/to/module.go                 # failure scenarios missing from tests
```

Every preset dispatches your enabled providers in parallel, each with a tailored lens, then a synthesizer emits a single markdown report with a disagreement table. Authoring your own preset is a `swarm.yaml` file (examples in [`docs/examples/swarm/`](./docs/examples/swarm/), spec in [`docs/specs/2026-05-08-v0.13.0-preset-sdk.md`](./docs/specs/2026-05-08-v0.13.0-preset-sdk.md)).

### Quality fingerprinting (`findings.db`)

The piece nothing else has. Every swarm run writes its findings to a local SQLite store:

```sh
jutsu finding stats                              # frequency table by preset / severity
jutsu finding precision                          # actioned-rate per (provider, persona)
jutsu finding precision --recommend              # → "use --personas claude,antigravity for this repo"
jutsu finding export --format jsonl > db.jsonl   # your data, portable
```

Cold-start safe (insufficient data → falls back to defaults). Codebase-scoped (one codebase's routing doesn't leak to another). Bootstrap-aware (low-confidence tuples explicitly excluded from recommendations).

### `agents.yaml` — bring any LLM

One file declares your provider catalog. Four driver kinds covered:

```yaml
# ~/.kaijutsu/agents.yaml
version: 1
providers:
  # Native CLIs — use the vendor's own login flow
  claude:    { driver: cli, cmd: claude }
  codex:     { driver: cli, cmd: codex }
  antigravity: { driver: cli, cmd: agy }

  # HTTP — direct OpenAI-compat or Anthropic-compat, no harness in the path
  deepseek:
    driver: http
    protocol: openai-compat
    base_url: https://api.deepseek.com/v1
    model: deepseek-v4-flash
    api_key_env: DEEPSEEK_API_KEY
    cost: { input_per_mtok: 0.27, output_per_mtok: 1.10 }

  qwen:
    driver: http
    protocol: openai-compat
    base_url: https://dashscope.aliyuncs.com/compatible-mode/v1
    model: qwen3-coder-plus
    api_key_env: DASHSCOPE_API_KEY

  # Local — Ollama / vLLM / llama.cpp, anything OpenAI-compat
  local-qwen:
    driver: http
    protocol: openai-compat
    base_url: http://localhost:11434/v1
    model: qwen2.5-coder:32b

  # MCP — any MCP tool server can be a swarm peer (e.g. semgrep-mcp)
  semgrep:
    driver: mcp
    transport: stdio
    command: npx
    args: ["semgrep-mcp"]
    tool_name: analyze
```

Per-project overrides live in `<repo>/.kaijutsu/agents.yaml`:

```yaml
version: 1
enabled: [claude, antigravity, deepseek]    # which providers swarm dispatches to in this repo
overrides:
  deepseek: { model: deepseek-v4-pro } # use the heavier reasoning model for this project
```

Personas (`(provider, model, system_prompt, tags)` tuples) author themselves alongside:

```yaml
personas:
  paranoid-claude:
    provider: claude
    system_prompt: "Lean conservative. Flag any change touching auth or session state."
    tags: [security]
```

Then: `jutsu swarm pr-review --personas paranoid-claude,architecture-purist-antigravity --pr 42`.

Full driver reference + cost model + MCP examples in [`docs/multi-agent.md`](./docs/multi-agent.md).

## Compose your own

A skill is one directory:

```
skills/community/my-skill/
  skill.yaml         # name, version, license, permissions, agents[], deps
  SKILL.md           # the standard — works for every agent
  scripts/           # optional shared executables
  README.md
```

```yaml
# skill.yaml
name: my-skill
version: 0.1.0
license: MIT
layout: flat
description: "What this skill does, when to invoke it."
agents: [claude, codex, antigravity]
permissions:
  bash: false
  network: false
  fs-write: false
deps:
  skills:
    - deslop@^0.2     # compose existing primitives
```

Then `jutsu publish` opens a PR against `skills/community/` (or `registry/index.json` for third-party-repo pointers). [`SCHEMA.md`](./SCHEMA.md) covers the full reference; [`CONTRIBUTING.md`](./CONTRIBUTING.md) covers the submission flow.

## Trust model

- **Signed core.** `skills/core/` is solo-maintained + signed at release time with [Sigstore](https://www.sigstore.dev/) keyless signing. The CLI verifies the signature and GitHub identity of the signer before installing.
- **Permission manifests.** Every skill declares `bash` / `network` / `fs-write` in `skill.yaml`. The CLI prompts on install when sensitive permissions are declared. Use `--yes` to opt-in non-interactively.
- **Local SQLite.** `findings.db` lives in `.kaijutsu/findings.db` (per project) or `~/.kaijutsu/findings.db` (global). Never leaves disk.
- **Local-only telemetry.** `~/.kaijutsu/usage.jsonl` logs `{ts, cmd, exit, ms}` per invocation. No flag values, no args, no content. Opt-out via `KAIJUTSU_USAGE_LOG=0`.
- **No SaaS in the path.** Resolution + install + signature verification + cost estimation all run client-side. The only network calls are to GitHub (skill download, tag resolution) and to your declared provider endpoints.
- **MIT throughout.** Skill licenses gated to MIT-compatible (BSD-2/3, ISC, Apache-2.0 with attribution).

Security disclosure: [`SECURITY.md`](./SECURITY.md).

## Install

```sh
# macOS / Linux — Homebrew
brew install momentmaker/tap/jutsu

# Any POSIX — shell installer
curl -fsSL https://kaijutsu.dev/install.sh | sh

# Authenticate GitHub API for higher rate limits (optional but recommended)
export GITHUB_TOKEN=$(gh auth token)

# Uninstall
brew uninstall jutsu
# or
curl -fsSL https://kaijutsu.dev/uninstall.sh | sh
```

Pre-built binaries for `darwin/{amd64,arm64}` and `linux/{amd64,arm64}` on every release: [Releases](https://github.com/momentmaker/kaijutsu/releases).

## Contributing

PRs welcome on three surfaces:

- **Skills** — submit to `skills/community/` or register a third-party repo in `registry/index.json`. See [`CONTRIBUTING.md`](./CONTRIBUTING.md).
- **Providers** — `agents.yaml` is open. If you're shipping a new OpenAI-compat or Anthropic-compat endpoint, all you need is a snippet in `docs/multi-agent.md` and (optionally) a cost-card.
- **CLI** — Go module under `cli/`. `go test ./...` from `cli/`; conventional commits.

All contributions MIT-licensed (or compatible: BSD-2/3, ISC, Apache-2.0). Skill quality enforced by `jutsu lint` in CI.

## Etymology

Three readings, layered:

1. **Open skills** *(primary)* — `kai` (開, "open") + `jutsu` (術, "skill / technique"). An open registry of agentic skills, free for any agent to consume.
2. **Beast of techniques** — `kaiju` (怪獣, "strange beast") + `jutsu`. A nod to the chibi monster mascot. Open-source agents are wild creatures of capability waiting to be tamed and shared.
3. **Kaizen-jutsu** — short for *kaizen-no-jutsu*, "techniques of continuous improvement." A system of practices toward optimum improvement, lived by a regret-minimization framework.

The primary meaning is *open skills*. The other readings are happy resonances.

## License

MIT. See [`LICENSE`](./LICENSE). Third-party attributions in [`NOTICE.md`](./NOTICE.md).
