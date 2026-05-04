# Skill Evals (skeleton)

Per-skill eval harnesses verify a skill still produces the right behavior. They run in CI and catch regressions when a skill's SKILL.md drifts in quality or when a refactor breaks composability.

This document is a v0 skeleton. The full harness is a v0.2 deliverable.

## Goals

- Detect when a skill stops triggering on its documented trigger phrases
- Detect when a skill's structured output (JSON sidecar) drifts from its declared schema
- Detect when a skill's hard rules get silently dropped
- Surface skill-quality regressions in CI before they reach users

## File layout

A skill that ships evals adds an `evals/` subdirectory under its skill root:

```
skills/core/<name>/
  skill.yaml
  SKILL.md
  README.md
  evals/
    cases.yaml          # the eval cases for this skill
    fixtures/           # any sample inputs the cases reference
```

## `evals/cases.yaml` schema

```yaml
version: 1
skill: <name>
cases:
  - name: triggers-on-explicit-invocation
    description: Skill should activate when the user invokes /<name>
    input:
      conversation: |
        user: /<name>
    expectations:
      - skill_invoked: true
      - tools_used_at_least_one_of: [Read, Grep, Bash]

  - name: produces-required-output-fields
    description: When invoked, the skill emits the documented output sections
    input:
      conversation: |
        user: <the trigger that should produce the canonical flow>
    expectations:
      - output_contains_section: "Findings"
      - output_contains_section: "Verdict"

  - name: respects-hard-rule-N
    description: <a specific hard rule the SKILL.md states>
    input:
      conversation: |
        user: <a scenario that would tempt the skill to break the rule>
    expectations:
      - output_does_not_contain: "<the forbidden phrase or behavior>"
```

## Eval runner (deferred)

The runner — likely `jutsu eval [--skill <name>]` — is not implemented in v0.1. The proposed shape:

1. Read `evals/cases.yaml`
2. For each case, run the skill against the input scenario via the agent's API
3. Parse the agent's response, check expectations
4. Emit a per-skill PASS/FAIL summary

Until then, this document defines the format so authors can write eval cases speculatively. They become live once the runner ships.

## What to write evals for

Start with a skill's most important guarantees:

- **Trigger fidelity** — does the skill activate when the user uses one of the documented trigger phrases?
- **Output structure** — does the skill produce the sections the SKILL.md promises?
- **Hard rules** — does the skill respect the explicit "don't" rules?
- **Composition** — for primitive-consumer skills, does the skill correctly invoke its declared `deps.skills`?

Skip evals for:

- Subjective quality (tone, prose flow) — humans review those
- Wide-ranging "is this right?" questions — too brittle to encode
- Anything dependent on private API keys or paid services in CI

## CI integration (deferred)

Once the runner ships, the `lint-skills.yml` workflow grows an eval step that runs after schema validation. Failures block merge; warnings surface as PR comments.

## Schema versioning

`cases.yaml` carries a `version` field. v0.2 is the first stable schema. Breaking changes bump the major and require existing cases to migrate.
