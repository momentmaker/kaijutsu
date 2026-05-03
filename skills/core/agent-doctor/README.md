# agent-doctor

Multi-agent diagnostic + cleanup tool for `~/.claude`, `~/.codex`, `~/.gemini`, `~/.agents`. Audits disk usage, surfaces stale sessions and orphaned project dirs, reclaims space using a trash-then-purge model with 30-day recovery.

Read-only by default; never touches `MEMORY.md`, `CLAUDE.md`, `AGENTS.md`, `GEMINI.md`, `skills/`, `agents/`, `hooks/`, `settings*.json`.

Install:
```bash
jutsu install agent-doctor
```

Trigger phrases: "agent doctor", "clean up claude", "audit codex", "old sessions", `/agent-doctor`.

This is a v0 flat port of the original Claude-only `claude-doctor`. The full rich layout (scripts/references/runbooks) returns in v0.1.
