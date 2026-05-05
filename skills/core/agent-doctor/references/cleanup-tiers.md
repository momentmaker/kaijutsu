# Cleanup Tiers

agent-doctor uses tiered rules so cleanup risk scales with user intent. Lower tier = higher confidence + easier reversibility. Tiers apply identically across every agent dir.

## Tier 1: safe (default)

Removable with high confidence. Reversible via trash for 30 days.

| Target | Cutoff | Why safe |
|---|---|---|
| `telemetry/*` | mtime >7d | Failed-event uploads. SDK retries for ~24h. After a week, dead. |
| `paste-cache/*` | mtime >30d | One-shot clipboard captures. After a month, never referenced again. |
| `image-cache/*` | mtime >30d | Image inputs to past sessions. Originals still on user's disk. |
| `file-history/*/` | mtime >90d (top-level dirs only) | Editor undo state per session. Undo window long passed. |
| `security_warnings_state_*.json` | mtime >14d (root only) | Per-session ephemeral approvals. Stale state, not load-bearing. |
| **Orphaned project dirs** | n/a | `projects/-Users-foo-bar/` where no candidate filesystem path resolves. The repo is gone; sessions are unrecoverable. Memory inside is still protected. |

## Tier 2: aggressive

Tier 1 + actions with longer recovery friction.

| Target | Action | Why deferred to aggressive |
|---|---|---|
| `projects/**/*.jsonl` | zstd-compress in place, >180d, >1MB | Loses fast `--resume`; needs decompression to read. Saves 5–10× on long-tail sessions. Reversible (`zstd -d`), not transparent. |
| `.trash/*` | hard delete, batch >30d | Two-stage delete completion. After 30d in trash, the user has accepted the loss. |

## Why these cutoffs

- **7d telemetry** — Anthropic SDK retries failed uploads for ~24h. After a week, the events are dead.
- **30d paste/image cache** — Most "I need that paste" workflows happen within a week. A month is ample buffer.
- **90d file-history** — Editor undo is session-scope; 90d is well past any practical recall.
- **180d sessions** — Half a year of inactivity = repo is dormant. Compression preserves data, drops cost.
- **30d trash TTL** — Long enough to notice "wait, where did that go," short enough that disk doesn't refill.

## Overriding

Edit constants near the top of `cleanup.sh` — search for `-mtime +N`. Or pass them on the command line by patching the script. Per-tier YAML config is a future enhancement.

## Per-agent scope

By default the scripts walk every agent dir. Limit with:

```bash
AGENT_DIR=~/.claude scripts/cleanup.sh --apply --tier safe
```

Or override the full list:

```bash
AGENT_DIRS=~/.claude:~/.gemini scripts/doctor.sh
```

## What NEVER gets touched, regardless of tier

See `protected-paths.md`. Memory dirs, skills, agents, hooks, plans, settings — all immune across every agent dir.
