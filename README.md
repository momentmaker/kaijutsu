<p align="center">
  <img src="./docs/assets/logo-256.png" alt="kaijutsu mascot" width="200" height="200">
</p>

<h1 align="center">kaijutsu</h1>

<p align="center">
  <strong>Open skills for AI coding agents.</strong><br>
  One source of truth — Claude, Codex, Gemini, all from the same registry.
</p>

<p align="center">
  <a href="https://kaijutsu.dev">kaijutsu.dev</a> ·
  <a href="./ROADMAP.md">Roadmap</a> ·
  <a href="./CONTRIBUTING.md">Contributing</a>
</p>

<p align="center">
  <a href="https://github.com/momentmaker/kaijutsu/releases/latest"><img src="https://img.shields.io/github/v/release/momentmaker/kaijutsu?display_name=tag&color=3DDC97" alt="Latest release"></a>
  <a href="./LICENSE"><img src="https://img.shields.io/badge/license-MIT-3DDC97.svg" alt="License: MIT"></a>
  <a href="https://github.com/momentmaker/kaijutsu/actions/workflows/lint-skills.yml"><img src="https://github.com/momentmaker/kaijutsu/actions/workflows/lint-skills.yml/badge.svg" alt="Lint skills"></a>
</p>

---

> **Status: alpha.** The `jutsu` CLI works end-to-end (init / install / list / remove / upgrade / lint / search / info / publish / agent / swarm / finding). 26 core skills currently ship across task-oriented (decide, journal, polish, unstuck, scope-check, session-retro, agent-doctor, pr-review, readme-update, doc-review, brainstorm, refactor-plan, security-audit, spec-driven-development, planning-and-task-breakdown, security-and-hardening, code-simplification, incremental-implementation), composable primitives (blunder-hunt, lie-to-them, convergence-detect, deslop, dispatch-parallel, multi-model-synth, project-memory), and a hooks bundle (dcg). v0.6 ships multi-provider swarm (driver abstraction + agents.yaml + personas); v0.7 adds quality fingerprinting (`jutsu finding *` + confidence-weighted synthesizer) — see below. Install paths, skill schema, and command surface are stable for v0.x. The public skill catalog at [kaijutsu.dev](https://kaijutsu.dev) is still maturing — see [`ROADMAP.md`](./ROADMAP.md). Security model: [`SECURITY.md`](./SECURITY.md).

## Why

Skills (markdown + scripts that extend AI coding agents) are exploding across Claude Code, Codex, and Gemini CLI. The good news: all three agents converge on the [Anthropic Agent Skills](https://agentskills.io) open standard, and Codex + Gemini both read from `.agents/skills/`. The bad news: nobody is curating a shared registry, and there is no easy way to keep the same toolkit in sync across all three agents.

**kaijutsu** is an MIT-licensed, agent-agnostic registry and CLI for AI agent skills. The CLI is named `jutsu`. One author manifest, two install paths (`.claude/skills/` for Claude, `.agents/skills/` for Codex + Gemini), three agents covered.

## Quickstart

```bash
# Install jutsu
brew install momentmaker/tap/jutsu
# or
curl -fsSL https://kaijutsu.dev/install.sh | sh

# Uninstall jutsu
brew uninstall jutsu
# or
curl -fsSL https://kaijutsu.dev/uninstall.sh | sh

# In any project
jutsu init                  # detects which agents you have installed
jutsu install pr-review     # writes to .claude/skills/ and/or .agents/skills/
jutsu list
jutsu upgrade

# Globally
jutsu install -g unstuck
```

A skill lives in this repo (or any third-party repo registered in `registry/index.json`) as a directory:

```
skills/core/pr-review/
  skill.yaml          # kaijutsu metadata (name, version, license, perms, agents[])
  SKILL.md            # Anthropic Agent Skills standard — works for all three agents
  scripts/            # optional shared executables
  README.md
```

Most skills only need a single `SKILL.md`. For the rare cases where Claude and Codex need different content, drop a per-agent override under `overrides/<agent>/SKILL.md`. See [`SCHEMA.md`](./SCHEMA.md) and [`docs/multi-agent.md`](./docs/multi-agent.md) for the full reference.

## Core skills

**Task-oriented** — `pr-review` · `readme-update` · `decide` · `journal` · `polish` · `unstuck` · `scope-check` · `session-retro` · `agent-doctor` · `doc-review`

**Primitives (composed by other skills via `deps.skills`)** — `blunder-hunt` · `lie-to-them` · `convergence-detect` · `deslop` · `dispatch-parallel` · `multi-model-synth` · `project-memory`

Run `jutsu search <query>` to find skills by name / description / tag, or `jutsu info <skill>` for full metadata.

## Multi-agent flywheel (`jutsu swarm`)

Seven swarm presets (`pr-review`, `doc-review`, `brainstorm`, `refactor-plan`, `security-audit`, `dream`, `reverse`) orchestrate multiple agents in parallel through `jutsu swarm <preset>`. Each agent reviews the same input through a different lens, then a synthesizer produces a single markdown report with a disagreement table. Used as the universal QA gate for kaijutsu artifacts:

- `jutsu swarm pr-review --pr 42` — adversarial multi-agent code review on a PR
- `jutsu swarm doc-review SPEC.md` — multi-agent review on a spec / plan / decision record / design doc
- `jutsu swarm brainstorm "<prompt>"` — orthogonal-angle ideation
- `jutsu swarm refactor-plan <files>... --goal "<goal>"` — ordered refactor steps with risk per step
- `jutsu swarm security-audit --pr 42` — CVSS-aligned threat model
- `jutsu swarm dream "<topic>"` — pre-implementation interrogation across 4-8 cognitive lenses (v0.8)
- `jutsu swarm reverse --spec <path> --diff <range>` — spec-vs-impl drift detector (v0.9)

Three artifact-producing skills (`spec-driven-development`, `planning-and-task-breakdown`, `decide`) call `jutsu swarm doc-review` as their final review pass instead of inventing their own — a single shared QA gate replaces three ad-hoc mechanisms. See [`IMPLEMENTATION_PLAN_PHASE2.md`](./IMPLEMENTATION_PLAN_PHASE2.md) for the design.

### v0.6 — Multi-provider agents

`jutsu swarm` is no longer locked to claude/codex/gemini CLIs. Four driver kinds:

- **`cli`** — native CLIs (claude/codex/gemini) — the v0.5 default
- **`http`** — direct OpenAI-compat / Anthropic-compat HTTP (DeepSeek, GLM, Kimi, local Ollama)
- **`cli-compat`** — wrap a native CLI with `BASE_URL` + `KEY` override (opt-in; emits a one-shot warning about telemetry leakage)
- **`mcp`** — MCP servers as deterministic peers (semgrep, eslint, custom analyzers); `Result.CostUSD = 0`

Two-layer config in `agents.yaml`:

- `~/.kaijutsu/agents.yaml` — global provider catalog
- `<repo>/.kaijutsu/agents.yaml` — project enabled list + per-repo overrides

7 built-in personas: 3 `default-*` (empty system prompt for v0.5 cache compat) + 4 reference flavored (`paranoid-security-claude`, `pragmatic-codex`, `architecture-purist-gemini`, `brainstorm-creative-claude`). Skills can require persona tags; auto-synthesis of `default-<provider>` for any enabled provider.

Quick start:

```bash
jutsu agent add deepseek                         # writes catalog default to .kaijutsu/agents.yaml
export DEEPSEEK_API_KEY=sk-...
jutsu agent enable deepseek
jutsu agent test deepseek                        # /models GET (or 1-token completion fallback)
jutsu swarm pr-review --personas paranoid-security-claude,pragmatic-codex,deepseek
jutsu swarm pr-review --estimate                 # dry-run cost projection (±20% accuracy)
```

`jutsu agent` ships 8 subcommands (list, doctor, add, enable, disable, remove, test, migrate). See [`docs/multi-agent.md`](./docs/multi-agent.md) for the full driver reference + persona authoring guide.

### v0.7 — Quality fingerprinting + confidence-weighted synthesizer

After ~10 swarm runs you've built up an implicit sense of which agent's findings consistently land vs which get dismissed. v0.7 captures that signal: every finding goes into a local SQLite store at `~/.kaijutsu/findings.db`, and the synthesizer weights each agent's vote by its observed precision per `(provider, persona, preset, codebase)` tuple. Cold-start behavior is byte-identical to v0.6.2 — no DB → no change.

```bash
jutsu swarm pr-review --personas paranoid-security-claude,default-gemini
jutsu finding list --pending          # what's actionable
jutsu finding accept 5 --reason "real auth bug"
jutsu finding dismiss 12 --reason "stylistic nit"
jutsu finding stats                   # per-(provider, persona, preset) precision
jutsu swarm pr-review --personas ... --show-weights   # opt-in: see weights in disagreement table
```

Three-state weight algorithm: cold-start (1.0) → bootstrap (0.7 after first action) → mature (clamped precision after 10 actioned). Sliding window of 200 actioned findings catches model drift after ~50 actions.

**Privacy**: no network calls. `cli/internal/cli/finding.go` is forbidden from importing any networking package — enforced by an import-list test. The DB lives in `$HOME` at mode `0600`, never in any repo. Telemetry / multi-machine sync deferred to v0.8 with separate spec + ADR.

See [`docs/multi-agent.md`](./docs/multi-agent.md#v07--quality-fingerprinting--confidence-weighted-synthesizer) for the full algorithm + CLI reference.

### v0.9 — Feedback-loop hardening

Five-item bundle closing v0.7+v0.8 gaps. Spec: [`docs/specs/2026-05-07-v0.9.0-feedback-loops.md`](./docs/specs/2026-05-07-v0.9.0-feedback-loops.md).

```bash
jutsu swarm reverse --spec docs/specs/foo.md --diff origin/main...HEAD   # spec-vs-impl drift detector
jutsu swarm dream "Should we ship X?" --mode full --yes                  # Pass-2 lens debate (v0.9 lifts cobra reject)
jutsu finding sync-pr 42 --apply                                         # ingest accept/dismiss replies into findings DB
KAIJUTSU_DISABLE_AGENTS=gemini jutsu swarm pr-review                      # force a provider subset for testing
KAIJUTSU_DREAM_ADAPTIVE_LENS=off jutsu swarm dream "..."                 # disable per-lens precision weighting (v0.8.3-byte-identical)
```

- **`jutsu swarm reverse`** — spec-vs-impl drift detector. Categorizes findings as ADDED / OMITTED / CHANGED / AMBIGUOUS. Distinct marker prefix coexists with pr-review on the same PR.
- **Dream Pass-2** — `--mode full` runs a critique round; agents emit `[new] / [disputes] / [revised] / [agreed]` revision tags. Cost prompt + non-TTY hard-fail prevents silent CI dispatch.
- **Lens-rotation rule** — repeat dreams within 7d shift the LEAD lens through the canonical 8-lens cycle (`honest → fit → gaps → wild → adversary → inverse → status-quo → time → honest`).
- **Adaptive lens-weighting** — synthesizer reads per-lens precision from the findings DB to tier load-bearing dream insights (≥0.7 → high-confidence badge, <0.4 → demoted to "consider" section).
- **`jutsu finding sync-pr <pr>`** — ingest human accept/dismiss decisions from PR reply comments. Grammar: `accept: <run_id>:<pos>` / `dismiss: <run_id>:<pos>`. Dry-run by default; `--apply` writes.
- **Per-skill provider routing** — `skill.yaml` `routing:` block declares per-persona preferred providers. 3-phase resolver picks first available.
- **Defaults bumped** for big-PR ergonomics: per-agent timeout 3min → 10min, diff/files cap 200KB → 500KB.

See [CHANGELOG.md](./CHANGELOG.md#090--2026-05-07) for the full feature list and the v0.9.x deferral list.

### v0.10 — `jutsu eval` runner

Test framework for skills. Port of [agent-skills-eval](https://github.com/darkrishabh/agent-skills-eval) (Anthropic's agentskills.io eval format) to Go + 3 kaijutsu-native swarm-shape extensions. Spec: [`docs/specs/2026-05-08-v0.10.0-eval-runner.md`](./docs/specs/2026-05-08-v0.10.0-eval-runner.md).

```bash
# Single-skill eval — agent-skills-eval upstream parity
jutsu eval skill skills/core/dream

# Per-persona head-to-head (kaijutsu-native)
jutsu eval persona --evals skills/core/dream/evals/evals.json

# Per-preset mode comparison (e.g. dream quick vs full)
jutsu eval preset --evals skills/core/dream/evals/evals.json

# Skill loaded into swarm pipeline vs not
jutsu eval swarm-skill --evals skills/core/dream/evals/evals.json

# CI gate: stateful --strict against prior tag's baseline
jutsu eval skill skills/core/dream --strict --baseline-from v0.9.1
```

- **Compat**: reads agent-skills-eval upstream `evals.json` files unchanged. Skill authors targeting both ecosystems write one schema.
- **4 eval shapes** beyond single-target eval — only kaijutsu measures multi-agent + per-persona + per-preset lift.
- **Workspace lock + cost guard + path-traversal sanitization + XSS-safe HTML report** — production-shaped from day one.
- **CI workflow** `.github/workflows/eval-skills.yml` validates the eval harness (Go tests + cobra wiring + `evals.json` parse-check) on PRs, pushes to main, and `v*` tags. Live judge dispatch deferred to v0.10.x once secrets-injected `ANTHROPIC_API_KEY` is wired.
- **3 core skills eval-covered at v0.10**: `dream`, `pr-review`, `scope-check`. Coverage gate prevents drift.

See [CHANGELOG.md](./CHANGELOG.md#0100--2026-05-08) for the full feature list, Stage 2 stub limitations, and the v0.10.x deferral list.

### v0.10.1 — polish

- **`--dry-run` on `install`/`upgrade`/`remove`/`publish`**. Each command resolves + validates as normal but skips every filesystem, lockfile, and network mutation. Per the agent-native CLI [principle #4](https://trevinsays.com/p/10-principles-for-agent-native-clis).
- **Error messages enumerate valid options** when rejecting an enum value (preset name, persona name, agent name, hook event). Self-correct in one retry instead of trial-and-erroring against `--help`.

### v0.12.0 — Tier A swarm presets

Three new presets that share a structural signature: hypothesis generation + cross-agent ranking by evidence. Multi-agent disagreement IS the differentiator.

```bash
# Surface failure scenarios MISSING from existing tests
jutsu swarm test-gap --code src/auth/login.go --tests src/auth/login_test.go

# Vague bug → ranked repro hypotheses + minimal-repro steps
jutsu swarm bug-repro "intermittent login failure on mobile" --files src/auth/

# Explain WHY legacy code looks the way it does
jutsu swarm code-archaeology --code cli/internal/swarm/preset.go --git-log 1y
```

- **`test-gap`** — Pair with `/polish`: polish ensures tests pass, test-gap ensures they cover.
- **`bug-repro`** — Differs from brainstorm (generates SOLUTIONS) and pr-review (hunts BUGS in a diff). Reasons from a bug REPORT — symptoms only.
- **`code-archaeology`** — Use BEFORE refactoring legacy code. Surfaces "why is this weird" answers faster than reading commit history manually. Theories marked corroborated / single-source / contested.

Picks survived a `jutsu swarm dream` adversarial pass on 7 candidates. Killed: `dep-review` (deterministic tools win), `api-review` (commodity, IDE-native soon). Deferred: `migrate` (too high-stakes), `postmortem` (harm-vector).

See [CHANGELOG.md](./CHANGELOG.md#0120--2026-05-08) for the full feature list, dream-survival framing, and v0.13+ deferral list.

### v0.11.0 — `swarm.RunPipeline` extraction + autopilot v2

Two converging features. The `runSwarmPipeline` extraction (470 LOC inline → 50 LOC cobra adapter + reusable public API) closes the v0.10.1-deferred Stage 1 work; `eval preset` and `eval swarm-skill` ship real implementations. Autopilot v2 lands as a distributed core skill, replacing the user-scope v1.

```bash
# autopilot v2 — distributed via kaijutsu
jutsu install autopilot                     # archives any pre-existing v1 to .archived/
jutsu autopilot init                        # writes .kaijutsu/autopilot.yaml from defaults
# Inside an agent CLI session:
/autopilot add a feature flag system        # 6-phase pipeline → opens PR
```

- **6-phase pipeline**: brainstorm (anti-sycophancy via dream) → spec → doc-review → plan → doc-review → build/polish per stage → final swarm pr-review → reverse-drift gate → PR.
- **2 gates total**: post-brainstorm + PR review on GitHub. Adversarial multi-agent review at every artifact replaces single-reviewer model.
- **Cost-capped**: $20 soft default, $100 hard ceiling in skill code. Env-var override per-shell only — yaml cannot raise the ceiling.
- **Reverse-drift gate**: spec drift between approved spec and pre-PR branch diff surfaces in PR description; PR labeled `autopilot-drift`. Informational, not blocking.
- **3 new built-in personas** ship with kaijutsu (CLI-backed, no API keys): `claim-auditor-claude`, `cross-file-gemini`, `perf-purist-codex`.

See [CHANGELOG.md](./CHANGELOG.md#0110--2026-05-08) for the full feature list, doc-review fixes, dream-design pass, and v0.12+ deferral list.

## Trust model

- **Core skills** (this monorepo, `skills/core/`) are signed at release time using [Sigstore](https://www.sigstore.dev/) keyless signing. The CLI verifies the signature and the GitHub identity of the signer before installing.
- **Community skills** (third-party repos registered in `registry/index.json`) are unsigned by default. The CLI prompts the user when a skill declares sensitive permissions (shell, network, filesystem write).
- Every skill ships a permission manifest in `skill.yaml`.

## Troubleshooting

### `GitHub API rate limit exceeded`

`jutsu install` and `jutsu upgrade` hit the GitHub API to resolve tags and download tarballs. Anonymous requests are capped at **60/hr per IP**; authenticated requests get **5000/hr**.

Set `GITHUB_TOKEN` to a personal access token to authenticate:

```sh
# Use the token gh CLI is already holding
export GITHUB_TOKEN=$(gh auth token)

# Or set explicitly in your shell rc (~/.zshrc, ~/.bashrc, etc.)
export GITHUB_TOKEN="ghp_..."
```

A fine-grained PAT with **public-repository read access** is sufficient. No write or admin scopes needed.

The same env var works for `install.sh`:

```sh
curl -fsSL https://kaijutsu.dev/install.sh | GITHUB_TOKEN=$(gh auth token) sh
```

### `dep <X> of <Y>: skill.yaml not found at skills/core/<X> in momentmaker/kaijutsu@<sha>`

You're on `v0.2.0`, which shipped broken transitive dep constraints. Upgrade to `v0.2.1` or later (`brew upgrade jutsu` or re-run `install.sh`).

## Recommended collections

kaijutsu is intentionally not the *only* skill source. We bundle a tight set under `skills/core/` (meta-process, primitives, safety) and adapt a handful of high-leverage skills from peer collections under MIT attribution:

- **[`addyosmani/agent-skills`](https://github.com/addyosmani/agent-skills)** — Addy Osmani's 20-skill SDLC framework (Define / Plan / Build / Verify / Review / Ship / Simplify). 5 of these are ported into our `skills/core/` with attribution: `spec-driven-development`, `planning-and-task-breakdown`, `security-and-hardening`, `code-simplification`, `incremental-implementation`. The remaining 15 are referenced in `registry/index.json` and installable via `jutsu install addyosmani/<skill>` (v0.3 vanilla-SKILL.md compat mode).
- **[Anthropic Agent Skills](https://agentskills.io)** — the open standard our schema extends. Any skill that follows the SKILL.md frontmatter convention is installable via kaijutsu, agent-agnostic.

Full attribution + license accounting in [`NOTICE.md`](./NOTICE.md).

## Contributing

See [`CONTRIBUTING.md`](./CONTRIBUTING.md). Skill authors can submit a PR to `skills/community/` or register a third-party repo in `registry/index.json`. All contributions must be MIT-licensed (or compatible: BSD-2/3, ISC, Apache-2.0).

## What "kaijutsu" means

Three readings, layered:

1. **Open skills** *(primary)* — `kai` (開, "open") + `jutsu` (術, "skill / technique"). An open registry of agentic skills, free for any AI agent to consume.
2. **Beast of techniques** — `kaiju` (怪獣, "strange beast") + `jutsu`. A nod to the chibi monster mascot. Open-source agents are wild creatures of capability waiting to be tamed and shared.
3. **Kaizen-jutsu** — short for *kaizen-no-jutsu*, "techniques of continuous improvement." A system of techniques varying in intensity and effort to make oneself better and stronger toward the state of **optimum improvement**, lived by a regret-minimization framework.

The primary meaning is *open skills*. The other readings are happy resonances.

## License

MIT. See [`LICENSE`](./LICENSE).
