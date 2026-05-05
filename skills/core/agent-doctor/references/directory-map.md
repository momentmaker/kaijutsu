# Agent Directory Map

Inventory of what each subdirectory holds across the four agent dirs scanned by agent-doctor (`~/.claude`, `~/.codex`, `~/.gemini`, `~/.agents`). Not every agent creates every subdir — the scripts skip missing paths silently.

## Common to all four agents

| Path | Purpose | Cleanup status |
|---|---|---|
| `CLAUDE.md` / `AGENTS.md` / `GEMINI.md` | Per-agent global instructions | **Protected** |
| `**/MEMORY.md` (any depth) | Per-project memory index | **Protected** |
| `**/memory/` (any depth) | Per-project memory directory | **Protected** |
| `agents/` | Subagent definitions | **Protected** |
| `skills/` | Skills (this includes agent-doctor itself) | **Protected** |
| `rules/` | Custom rules referenced from the global instructions file | **Protected** |
| `hooks/` | Hook scripts | **Protected** |
| `identities/` | Identity configs | **Protected** |
| `plans/` | Saved implementation plans | **Protected** |
| `settings.json` / `settings.local.json` / `config.toml` | Runtime config | **Protected** |
| `keybindings.json` | Custom keybindings | **Protected** |
| `ecosystem.yaml` | Ecosystem config | **Protected** |
| `projects/<repo>/` | Per-repo session state | conditional (see below) |
| `projects/<repo>/*.jsonl` | Session transcripts | aggressive: zstd-compress >180d (>1MB) |
| `projects/<repo>/memory/`, `MEMORY.md` | Per-project memory | **Protected** |
| `projects/<orphan>/` | Repo no longer on disk | safe: trash entire dir (memory inside still protected) |
| `telemetry/` | Failed-event uploads | safe: >7d |
| `file-history/<session>/` | Editor undo state | safe: >90d |
| `paste-cache/` | Clipboard captures | safe: >30d |
| `image-cache/` | Image input cache | safe: >30d |
| `security_warnings_state_*.json` | Per-session ephemeral approvals | safe: >14d |
| `plugins/` | Installed plugins (managed externally) | manual review |
| `plugins/cache/` | Plugin asset cache | manual review (risky to auto-clean) |
| `backups/`, `cache/`, `debug/`, `downloads/`, `logs/` | Misc agent state | manual review |
| `history.jsonl` | Prompt history (arrow-up recall) | manual truncate if needed |
| `tasks/`, `todos/` | Task/todo state | manual (may be active) |
| `shell-snapshots/` | Shell environment snapshots | manual review |
| `stats-cache.json`, `statsig/` | Stats / feature flag cache | regenerable |
| `mcp-needs-auth-cache.json` | MCP auth state | regenerable on next auth |
| `.trash/` | agent-doctor staging area | aggressive: hard-purge batches >30d |

## Per-agent specifics

**Claude Code (`~/.claude`)**
- `outreach/`, `outreach-log.md` — outreach skill state. Protected.
- `settings.local.json` — machine-specific overrides. Protected.

**Codex CLI (`~/.codex`)**
- Uses `config.toml` instead of `settings.json` for runtime config. Both are recognized as protected by `lib.sh`.
- `AGENTS.md` is Codex's persistent instruction file.

**Gemini CLI (`~/.gemini`)**
- Uses `settings.json` like Claude.
- `GEMINI.md` is Gemini's persistent instruction file.

**Shared `~/.agents/`**
- Cross-tool interoperability path that Codex and Gemini both honor for skills.
- Contents are typically `~/.agents/skills/<name>/SKILL.md`.

## Sizing on a heavy user (reference, 2026-05-04)

- `~/.claude/projects/` typically the fattest dir (1–2 GB across 30+ repos)
- `~/.claude/file-history/` ~100M
- `~/.claude/telemetry/` 50–100M depending on offline streaks
- `~/.claude/plugins/` 20–30M
- Codex / Gemini dirs are usually 10–100× smaller — they're newer

Typical safe-tier reclaim across all four dirs: 100–400M. Aggressive-tier additional: 1G+ depending on session compression ratio.
