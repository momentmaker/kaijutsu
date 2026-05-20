# Per-lens design

Why each agent gets a different prompt for refactor-plan.

## Principle: orthogonal angles on the same refactor

Three agents, three different mental hats. Without the asymmetry, you'd get three overlapping step lists. With it, the synthesizer has more material to work with — claude's "right new shape" + codex's "what order minimizes risk" + antigravity's "what existing pattern to match" combine into a richer plan than any single voice produces.

## claude — architectural decomposition

Best at:
- Reasoning about abstractions and boundaries — "this should be a service, this should be a value object"
- Spotting test seams — "extracting X makes Y testable"
- Bold-but-defensible structural moves

What claude tends to MISS:
- Concrete regression risk per step
- Existing-pattern adherence (it's biased toward proposing the "right" shape, not the "matches-the-repo" shape)

## codex — stepwise risk

Best at:
- Real-world refactor sequencing — small reversible steps before invariant-changing ones
- Naming concrete risks ("this step breaks integration_test.go's setup")
- Migration ordering ("schema migration must precede code that reads new column")

What codex tends to MISS:
- High-level architectural critique
- Cross-file pattern matching

## antigravity — pattern consistency

Best at:
- Detecting when a proposed shape doesn't match how similar concerns are factored elsewhere in the repo
- Naming + idiom consistency
- Catching unnecessary novelty (the refactor invents a new abstraction when an existing one would work)

What antigravity tends to MISS:
- Stepwise risk
- Deep architectural reasoning on a single complex function

## Synthesizer

Different shape from pr-review/doc-review's synthesizer. Refactor-plan's synthesizer:

1. Clusters steps by what-changes-and-where (file + intent), merging across reviewers
2. Orders into a SINGLE executable plan — low-risk reversible steps first, big-invariant steps last
3. Surfaces ordering DISAGREEMENTS explicitly — when claude wants Step 4 before Step 3 but codex wants the reverse, the synthesizer picks one with a rationale and notes the alternative

The orderings disagreements section is the highest-leverage signal in this preset's output. Refactors that go sideways usually do so because the wrong step ran first.

## Debate (Pass-2 critique, --full mode)

Each agent sees its own + peers' steps + must critique BOTH order and completeness:
- Reorder if a step depends on a later one
- Add if a peer surfaced a gap
- Drop if a peer's step is wrong (over-engineering, missing constraint)

Pass-2 is where complex multi-file refactors converge — the first pass produces 3 independent plans, the debate pass merges them into one credible sequence.

## Override per-repo

Each prompt is a single .md file in `prompts/`. To override locally:

```bash
mkdir -p .claude/skills/refactor-plan/prompts
cp ~/.claude/skills/refactor-plan/prompts/codex.md .claude/skills/refactor-plan/prompts/codex.md
$EDITOR .claude/skills/refactor-plan/prompts/codex.md
```

Useful for repo-specific rules ("always extract into the existing services/ package, not a new one") or domain-specific risk priors ("our deployment can't roll back schema migrations, so all schema steps go LAST").
