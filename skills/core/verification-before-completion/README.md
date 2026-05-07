# verification-before-completion

A primitive skill: discipline gate before claiming work is complete. Run the verification command fresh, capture output, only then emit the claim.

## Install

```bash
jutsu install verification-before-completion
```

## Compose

Invoke directly:

```
/verification-before-completion
```

Or have another skill compose it via `skill.yaml`:

```yaml
deps:
  skills:
    - verification-before-completion@^0.1
```

`polish`, `incremental-implementation`, and `pr-review` all compose this primitive.

## What it does

Forces you to run the actual verification command (test / lint / build / regression-test) BEFORE claiming "done" / "fixed" / "passing". Prevents the silent-fail tail where convergence ≠ correctness.

Not a finding-triage gate. Not human ceremony layered on top of agent output. Pure code-state verification, single source of truth across kaijutsu skills.

## Attribution

Adapted from [obra/superpowers/skills/verification-before-completion](https://github.com/obra/superpowers/blob/main/skills/verification-before-completion) under MIT. Copyright (c) Jesse Vincent. kaijutsu modifications: pared down for "trust the swarm" stance, reframed as a composable primitive, claim-table aligned with kaijutsu vocabulary.

See `SKILL.md` for the full discipline reference.
