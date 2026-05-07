# dcg — Destructive Command Guard

Pre-tool-use hook that blocks unrecoverable shell commands (`rm -rf /`, `git reset --hard`, `git clean -fd`, force-push to main, `rm` of `.env` / credentials, fork bombs, `shred` / `dd`, etc.) before the agent runs them.

Install registers the hook into all active agents (Claude Code, Codex CLI, Gemini CLI). Removal cleans it up surgically — user-authored hooks are preserved.

```sh
jutsu install -g dcg     # globally — covers every project on this machine
jutsu install dcg        # project-scoped
```

The first install prompts for `permissions.hooks` confirmation since hooks register into your agent settings file. Pass `--yes` to skip.

Trigger phrases: "install dcg", "destructive command guard", "guard rails", `/dcg`.

See [`SKILL.md`](./SKILL.md) for the full pattern list, customization recipes, and per-agent install paths.
