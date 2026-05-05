# Per-agent prompt design

Why each agent gets a different prompt + what each lens optimizes for.

## Principle: asymmetry beats redundancy

If we sent the same prompt to all three agents, we'd get three overlapping reviews — useful but not 3x as useful as one. The interesting signal is in the DISAGREEMENT, not the consensus. So each agent gets a tailored prompt that pushes its review into a region where the others are weaker.

## claude.md — high-level architecture + correctness

Claude tends to do well at:
- Reasoning over long contexts (whole-file or whole-PR understanding)
- Cross-cutting consistency (does this diff fit the rest of the codebase?)
- Calibrated tone — flags real issues without crying wolf
- Security-boundary reasoning (auth, input validation)

What claude tends to MISS:
- Subtle off-by-one and integer-overflow bugs
- Race conditions in concurrent code
- Encoding/locale gotchas

So claude.md leans into architecture + correctness + invariants and explicitly de-prioritizes nit-level findings.

## codex.md — brutal edge cases

Codex (and OpenAI codegen models broadly) is trained heavily on real-world bug-fix data. It tends to do well at:
- Off-by-one, nil/null handling, integer overflow
- Race conditions and ordering bugs
- Retry/idempotency semantics
- Inputs that look fine but break under load

What codex tends to MISS:
- High-level architectural critique (it's bias is "make this code work" not "should this code exist")
- Cross-file pattern matching at scale

So codex.md cranks the skepticism dial: "stake your reputation on it" framing pushes for concrete failure modes, not vague speculation.

## gemini.md — cross-file patterns + consistency

Gemini's strength is breadth + speed. It tends to do well at:
- Style/idiom drift across many files
- Naming consistency
- Detecting "this pattern conflicts with the existing pattern in X"
- Coverage gaps (which tests exist, which are missing)

What gemini tends to MISS:
- Deep reasoning about a single complex function
- Subtle invariant violations

So gemini.md leans into pattern-hunting and explicitly OKs an empty review when the diff is a one-off tactical fix.

## synthesizer.md — the actual product

Synthesis isn't just "combine the lists." It's:
1. Cluster findings that point at the same root cause across reviewers.
2. Surface DISAGREEMENT explicitly. A 1/3 finding is high-signal — the lone wolf might be the only one who saw the bug, OR might be the one who's wrong.
3. Sort by severity, break ties by consensus.

The disagreement table is rendered DETERMINISTICALLY by the orchestrator — we don't trust the synthesizer model to format it correctly. The synthesizer's job is the prose.

## debate.md — the Pass-2 critique

The Pass-2 prompt is asymmetric: it gives each agent its OWN findings + every peer's findings, and asks the agent to:
- Mark agreements (boost confidence)
- Mark disagreements (with reason)
- Add new findings the peers' reviews surfaced

The reasoning field carries `[agreed with peer X]` / `[disputes peer X: <why>]` markers so the synthesizer can tell which findings survived peer scrutiny.

## Override per-repo

Each prompt is a single .md file in this directory. To override locally:

```bash
mkdir -p .claude/skills/pr-review/prompts
cp ~/.claude/skills/pr-review/prompts/claude.md .claude/skills/pr-review/prompts/claude.md
# edit and add domain-specific rules to the bottom
```

The CLI checks project paths first, then home, then falls back to built-in. So a per-repo override doesn't require re-installing the skill.
