# Per-lens design

Why each agent gets a different prompt for doc-review.

## Principle: asymmetry beats redundancy

Same as `pr-review`: if we sent the same prompt to all three agents, we'd get three overlapping reviews — useful but not 3× as useful as one. The interesting signal is in the DISAGREEMENT, not the consensus. Each agent gets a tailored prompt that pushes its review into a region where the others are weaker.

## claude — completeness

Claude tends to do well at:
- Long-context reasoning across multi-section documents
- Spotting ambiguity and undefined terms
- Catching scope creep ("this section talks about X but the goal said Y")
- Calibrated tone — flags real gaps without crying wolf

What claude tends to MISS:
- Concrete implementability gaps ("how would you actually do this?")
- Cross-document references that don't resolve

claude.md leans into completeness + ambiguity + scope and explicitly de-prioritizes the implementability angle.

## codex — implementability

Codex (OpenAI codegen models broadly) is trained heavily on real-world implementation. It tends to do well at:
- "This says X but never specifies HOW"
- Acceptance criteria that don't define pass/fail
- API/CLI claims that contradict each other
- Numerical thresholds left unquantified

What codex tends to MISS:
- High-level scope and tone (it's "make this work" biased)
- Cross-section consistency at scale

codex.md cranks the implementer's-hat dial: "stake your reputation on it" framing pushes for concrete gaps, not vague concerns.

## gemini — consistency

Gemini's strength is breadth + cross-reference. It tends to do well at:
- Same term used differently across sections
- References to other docs/sections that don't resolve
- Heading hierarchy gaps
- Drift between locked-decisions and stage-detail

What gemini tends to MISS:
- Deep reasoning about a single complex section
- Subtle implementability gaps

gemini.md leans into pattern-hunting and explicitly OKs an empty review when the artifact is internally consistent.

## Synthesizer

Same shape as `pr-review`'s synthesizer: cluster, sort, surface disagreements. The disagreement table is rendered DETERMINISTICALLY by the orchestrator — synthesizer's job is the prose.

## Debate (Pass-2 critique, --full mode)

Each agent sees its own + peers' findings, marks agreement/disagreement, may add new findings the peers surfaced. The synthesizer then reads the post-debate findings, so 1/N findings that survive critique get visibility.

## Override per-repo

Each prompt is a single .md file in this directory. To override locally:

```bash
mkdir -p .claude/skills/doc-review/prompts
cp ~/.claude/skills/doc-review/prompts/claude.md .claude/skills/doc-review/prompts/claude.md
$EDITOR .claude/skills/doc-review/prompts/claude.md
```

The CLI checks project paths first, then home, then falls back to built-in. Repo-specific calibration without re-installing the skill.
