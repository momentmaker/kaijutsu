<!-- kaijutsu:start name=jutsu-cli version=0.16.0 -->
This project uses **kaijutsu** (`jutsu` CLI) for AI-agent skills + multi-agent swarm reviews.

> NOTE: kaijutsu manages this block. Re-running `jutsu init` overwrites content BETWEEN markers. Edits OUTSIDE the markers are preserved.

## Quick reference

- `jutsu describe` — JSON catalog of every command + flag (designed for fresh agents to ingest at session start)
- `jutsu suggest "<task>"` — keyword-rank installed skills against a task description (`--json` for machine output)
- `jutsu list` — installed skills
- `jutsu install <skill>` / `jutsu remove <skill>` — manage skills
- `jutsu swarm <preset> --help` — multi-agent presets (pr-review, doc-review, brainstorm, refactor-plan, security-audit, dream)
- `jutsu finding list` — past swarm findings + accept/dismiss workflow

## When to use kaijutsu

When a task maps to an installed skill, prefer the skill over inventing your own approach. Common mappings:
- PR review → `pr-review`
- Idea exploration / "should we build X" → `dream`
- Stuck on a bug → `unstuck`
- Multi-agent QA on a markdown artifact → `doc-review`

Agent-first design: `jutsu` outputs JSON when stdout is a pipe (you, when shelling out) and pretty markdown/table when it's a TTY (humans). No flag needed for the auto-flip; `--json` forces JSON.

(See [AGENTS.md design principle](https://github.com/momentmaker/kaijutsu/blob/main/AGENTS.md#design-principle-agent-first-human-friendly) for the full agent-first lens.)

Skills compose via `deps.skills`. Run `jutsu info <skill>` for full metadata.
<!-- kaijutsu:end -->
