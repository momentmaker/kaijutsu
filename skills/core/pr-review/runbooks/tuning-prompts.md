# Runbook: tuning per-agent prompts for your repo

The built-in prompts are calibrated for general code review. For specialized repos (security-critical, perf-sensitive, library APIs), per-repo overrides are usually worth the effort.

## When to override

Symptoms that suggest a per-repo override:
- One agent consistently produces noisy findings the team dismisses as low-value.
- A class of bug your team cares about (perf regressions, API stability, threading) doesn't surface even when present.
- Agents flag conventions that aren't conventions in your repo (e.g., "use Result<T,E>" when you use Go-style error returns).

## How to override

The CLI loads prompts in this order:

1. `<project>/.claude/skills/pr-review/prompts/<agent>.md`
2. `<project>/.agents/skills/pr-review/prompts/<agent>.md`
3. `~/.claude/skills/pr-review/prompts/<agent>.md`
4. `~/.agents/skills/pr-review/prompts/<agent>.md`
5. Built-in (compiled into jutsu)

So per-repo overrides go in `.claude/skills/pr-review/prompts/`.

```bash
mkdir -p .claude/skills/pr-review/prompts
cp ~/.claude/skills/pr-review/prompts/claude.md .claude/skills/pr-review/prompts/claude.md
$EDITOR .claude/skills/pr-review/prompts/claude.md
```

## What to add

Append a "Repo-specific rules" section to the agent's prompt. Keep the JSON schema instruction at the top intact — that's what makes the parse work.

Example for a Go API library:

```markdown
... (existing claude prompt) ...

## Repo-specific rules

- This is a public Go library. Any change to an exported symbol (capital
  initial letter on a func/type/var) is a potential semver-breaking
  change. Flag every such change as a `blocker` unless the diff also
  bumps go.mod's semver tag.
- Tests live in `*_test.go` next to the code they test. If a non-test
  file gains a new exported symbol with no corresponding test addition,
  flag it as `issue` severity with summary "missing test coverage on
  exported API".
- We use the standard library net/http; flag any new dependency on a
  third-party HTTP framework.
```

## Iterating with --replay

After each prompt change, re-run synthesis without re-fetching diffs or re-calling agents:

```bash
# First run records cache
jutsu swarm pr-review --pr 42 --full

# Edit the synthesizer prompt
$EDITOR .claude/skills/pr-review/prompts/synthesizer.md

# Re-run synthesis only (free, no model calls)
jutsu swarm pr-review --replay <sha>
```

The cached per-agent findings stay constant; only the synthesis prompt changes. Tune until the output is what you want, then keep the prompt overrides committed in the repo.

## What NOT to override

Don't change the JSON schema instruction at the top of each per-agent prompt. The orchestrator parses that schema. If you ask for a different shape, parse fails and you lose findings.

Don't shorten the prompts dramatically. The asymmetry is the point — claude.md should still emphasize "architecture+correctness" even with your additions, otherwise you collapse all three lenses into one.
