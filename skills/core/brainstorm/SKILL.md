---
name: brainstorm
description: Multi-agent ideation against a free-form prompt. Wraps `jutsu swarm brainstorm "<prompt>"` to run claude / codex / antigravity in parallel with three different angles (claude=long-horizon framing, codex=code-pattern grounding, antigravity=cross-domain analogy), synthesizes into a ranked list of approaches with tradeoffs. Use when the user says "brainstorm", "ideas for", "how should we approach", "options for", or invokes /brainstorm. Designed to feed into `decide` (for ADR capture of the picked option) or `refactor-plan` (for execution of the picked option).
---

# brainstorm

Three reviewers walk into a question. Each comes back with a different list. The synthesizer ranks them.

This skill wraps `jutsu swarm brainstorm`. Free-form prompt input, ranked-list output. Use it before `decide` (you don't know what to record yet) or before `refactor-plan` (you don't know which approach to plan against yet).

## How agents collaborate

| Lens | Agent | Looks for |
|---|---|---|
| **Long-horizon framing** | claude | Ideal end-state, ambition gap, bold-but-defensible moves |
| **Code-pattern grounding** | codex | Existing patterns, concrete libraries, real-world failure modes |
| **Cross-domain analogy** | antigravity | Adjacent fields, prior art, RFC/paper precedents |

Severity vocab differs from pr-review/doc-review. Brainstorm options use:

- **recommended** — high-confidence "this is what I'd do"
- **alternative** — also good, different tradeoff profile
- **risky** — works but with caveats
- **speculative** — wild idea worth recording, not yet defensible

The synthesizer ranks options across reviewers, groups similar approaches, and surfaces "cross-cuts" — themes that appeared in multiple options.

## Layout (rich)

```
brainstorm/
├── SKILL.md
├── skill.yaml
├── scripts/
│   └── run.sh
├── prompts/
│   ├── claude.md       long-horizon lens
│   ├── codex.md        code-pattern lens
│   ├── antigravity.md       cross-domain lens
│   ├── synthesizer.md  ranked-list assembler
│   └── debate.md       Pass-2 critique template
└── references/
    ├── lens-design.md       why each lens is what it is
    └── chaining.md          how brainstorm composes with decide / refactor-plan
```

## First-run consent

Brainstorm sends the user prompt to remote model providers. Per-repo consent required before the first run:

```bash
jutsu swarm brainstorm --grant-consent
```

This writes `allow-multi-model: true` to `.kaijutsu/brainstorm.yaml`. Subsequent runs work non-interactively.

## When to invoke

```bash
# Direct
jutsu swarm brainstorm "how do I rate-limit my API endpoints?"

# Via skill wrapper
~/.claude/skills/brainstorm/scripts/run.sh "what's the right cache invalidation strategy?"

# High-stakes question — use --full for a debate round
jutsu swarm brainstorm "should we migrate from Postgres to CockroachDB?" --full --strict

# Iterate cheap on the synth prompt
jutsu swarm brainstorm --replay <key>
```

## Reading the output

Same structure as pr-review/doc-review but the disagreement table shows each reviewer's options as 1/N rows by design (different lenses produce different options — that's the point). The synthesis sections matter most:

```markdown
## Recommended options
- **Redis sliding-window** (proposed by claude, codex) — proven pattern, ...
- **Token bucket per-API-key** (codex) — battle-tested, library exists ...

## Alternatives worth considering
- **CRDT-based limiter** (antigravity, speculative→alternative after debate) — ...

## Speculative
- **Probabilistic admission control** (antigravity) — paper-tier idea, ...

## Cross-cuts
- All three reviewers mentioned graceful degradation under burst.
- Both claude + codex emphasized the importance of per-key limits.
```

The "Cross-cuts" section is the highest-leverage signal — themes that emerged from multiple lenses are usually worth designing for, even if no single option won.

## Composes

This skill chains naturally:

1. `brainstorm` → pick a recommended option
2. `decide` → record the choice as an ADR (Step 5 multi-agent review on the decision record uses doc-review, not brainstorm)
3. `refactor-plan` (Stage 6) OR `spec-driven-development` → execute the chosen option

See `references/chaining.md` for example workflows.

## Hard rules

- **Never confuse with doc-review.** brainstorm produces NEW options. doc-review reviews EXISTING markdown. Different inputs, different outputs.
- **Don't run brainstorm to "review" something.** That's doc-review's job.
- **Cite reviewer in synthesis.** When picking an option, note which lens produced it — useful when you later want to re-examine the assumption.

## When NOT to use

- **You already know the answer** — skip to `decide` or implementation.
- **Question requires tools** (running queries, reading specific files) — brainstorm is prompt-only.
- **Question is "review my X"** — that's doc-review or pr-review.

## Cost

Default `--max-cost $1.00`. With 3 agents in `--quick` mode, typical brainstorm runs ~$0.05–0.20. `--full` (with debate) doubles that. `--strict` adds another synthesis call.
