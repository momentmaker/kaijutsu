# Runbook: tuning doc-review prompts for your repo

The built-in prompts are calibrated for general-purpose markdown review. For specialized artifacts (security RFCs, architecture decisions, compliance specs), per-repo overrides are usually worth the effort.

## When to override

Symptoms that suggest a per-repo override:
- One lens consistently produces noisy findings the team dismisses.
- A class of issue your team cares about (compliance gaps, missing diagrams, undefined SLAs) doesn't surface even when present.
- Agents flag conventions that aren't conventions in your repo (e.g., they expect numbered sections but your repo uses bullet points).

## How to override

The CLI loads prompts in this order:

1. `<project>/.claude/skills/doc-review/prompts/<lens>.md`
2. `<project>/.agents/skills/doc-review/prompts/<lens>.md`
3. `~/.claude/skills/doc-review/prompts/<lens>.md`
4. `~/.agents/skills/doc-review/prompts/<lens>.md`
5. Built-in (compiled into jutsu)

Per-repo overrides go in `.claude/skills/doc-review/prompts/`.

```bash
mkdir -p .claude/skills/doc-review/prompts
cp ~/.claude/skills/doc-review/prompts/claude.md .claude/skills/doc-review/prompts/claude.md
$EDITOR .claude/skills/doc-review/prompts/claude.md
```

## What to add

Append a "Repo-specific rules" section to the lens. Keep the JSON schema instruction at the top intact — that's what makes the parse work.

Example for a compliance-heavy doc repo:

```markdown
... (existing claude prompt) ...

## Repo-specific rules

- Every spec MUST have a "Compliance" section listing GDPR / SOC2
  controls touched. Flag missing section as `blocker`.
- Acceptance criteria MUST cite a specific test type
  (unit / integration / e2e). Vague "tested" without type is `issue`.
- Threat model MUST include at least one "Out of scope" subsection.
  Missing = `minor` finding with summary "no out-of-scope statement".
```

## Iterating with --replay

```bash
# First run records cache
jutsu swarm doc-review SPEC.md --full

# Note cache key from stderr (e.g., abc123def456)

# Edit synth prompt
$EDITOR .claude/skills/doc-review/prompts/synthesizer.md

# Replay (free, no model calls)
jutsu swarm doc-review --replay abc123def456
```

The cached per-agent findings stay constant; only synthesis prompt changes. Tune until output is what you want, then commit the prompt overrides.

## What NOT to override

Don't change the JSON schema instruction at the top. Orchestrator parses that schema. If you ask for a different shape, parse fails and you lose findings.

Don't shorten the prompts dramatically. The asymmetry is the point — claude.md should still emphasize completeness even with your additions, otherwise you collapse all three lenses into one.

## Severity threshold tuning

Add a `severity_floor` to `.kaijutsu/doc-review.yaml`:

```yaml
allow-multi-model: true
severity_floor: minor   # suppress info-level findings
```

(Stage 4+ wires this; today the floor is unused.)
