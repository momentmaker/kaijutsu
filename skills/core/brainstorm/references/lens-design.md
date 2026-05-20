# Per-lens design

Why each agent gets a different prompt for brainstorm.

## Principle: orthogonal lenses, not redundant ones

Same as `pr-review` and `doc-review`: if all three agents got the same prompt, three reviewers produce three overlapping option lists. Useful, but not 3× useful.

For brainstorm specifically the asymmetry is even more important than for review presets — review converges on a small number of "real" findings, but ideation diverges by design. The interesting outcome is the BREADTH of options across angles, not consensus.

## claude — long-horizon framing

Claude tends to do well at:
- Stretch reasoning ("if money/time/people were unlimited, what's the BEST version?")
- Spotting the ambition gap ("user picked the safe option but would have wanted X if they'd thought about it")
- Calibrated bold-but-defensible takes — flags when something's a stretch vs reckless

What claude tends to MISS in brainstorm:
- Concrete library names + battle-tested patterns
- Cross-domain analogies that aren't widely cited

claude.md leans into ambition + ideal-end-state.

## codex — code-pattern grounding

Codex (OpenAI codegen models broadly) is trained heavily on real code. Best at:
- Cite-able library names ("use go-redis/redis" vs "use a Redis client")
- Battle-tested patterns the user can pick up tomorrow
- Failure modes from real-world code (DIY rate-limiter has X common bug)

What codex tends to MISS:
- Stretch / aspirational framing
- Cross-domain analogies

codex.md cranks the practitioner's-hat dial: real names, real failure modes.

## antigravity — cross-domain analogy

Antigravity's strength is breadth across domains. Best at:
- "How does ops/SRE / queueing theory / distrib-systems / biology solve this shape of problem?"
- Citing RFCs, papers, well-known products
- Counterintuitive options the inside-this-codebase view would miss

What antigravity tends to MISS:
- Tight code-grounded recommendations (it's biased toward concepts vs implementations)

antigravity.md leans into adjacent-fields + prior-art.

## Synthesizer

Different shape from pr-review/doc-review's synthesizer. Brainstorm's synthesizer:

1. Groups options that point at the same APPROACH across reviewers (cross-lens convergence is signal — if claude AND codex AND antigravity independently proposed Redis sliding-window, that's a strong signal).
2. Ranks by severity (recommended > alternative > risky > speculative) with reviewer-count as tiebreaker.
3. Surfaces "cross-cuts" — themes that appeared in multiple options. Often more useful than any single option.

The disagreement table is rendered DETERMINISTICALLY by the orchestrator — it just shows which agent proposed which option. For brainstorm, the table is mostly 1/3 rows by design (different lenses → different options). The synthesis is where cross-lens convergence shows up.

## Debate (Pass-2 critique, --full mode)

Each agent sees its own + peers' options, marks "strengthened by peer X" / "weakened by peer X", may add new options the peers' angles surfaced. Pass-2 is where 1/3 options that survive critique get meaningful upgrades (or get dropped for not surviving).

## Override per-repo

Each prompt is a single .md file in `prompts/`. To override locally:

```bash
mkdir -p .claude/skills/brainstorm/prompts
cp ~/.claude/skills/brainstorm/prompts/claude.md .claude/skills/brainstorm/prompts/claude.md
$EDITOR .claude/skills/brainstorm/prompts/claude.md
```

Useful when you want a domain-specific lens (e.g., "always cite our internal docs first" or "options must be implementable in our existing stack only").
