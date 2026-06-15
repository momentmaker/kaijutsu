# Quality benchmark — loop output vs. a SOTA reference

This is the **loop-vs-reference** tier of the svg evals. It answers a different question than the calibration set (`../calibration.md`, which is agent-vs-human judgment):

> Is the skill's loop output competitive with a purpose-built SVG model's output for the same prompt?

The reference bar is [Quiver / Arrow 1.0](https://quiver.ai) (a SOTA text-to-SVG model). The skill does not claim to beat it — its edge is the loop + house-style system, not raw generation. This benchmark just keeps that claim honest: it shows how close the loop gets, and where it falls short.

## Why the references are not committed

Quiver is a commercial product. Its generated SVGs are its output, governed by its terms — **we do not redistribute them.** This repo is MIT-licensed, publicly distributed, and Sigstore-signed; bundling another product's output into it is a provenance problem.

So this directory commits only **text** (the prompts) and the **protocol**. The reference SVGs live in a local, git-ignored directory you populate yourself:

```
benchmark/
  README.md        # committed
  prompts.md       # committed — the prompt list (text)
  quiver-refs/     # git-IGNORED — you export quiver SVGs here, never committed
```

`quiver-refs/` is in the repo `.gitignore`. Nothing you put there is tracked or shipped.

## Setup (maintainer, local)

1. For each row in `prompts.md`, paste the **Quiver prompt** into quiver, generate, and save the SVG to `quiver-refs/<slug>.svg` (the slug is in the table).
2. That's it — the refs are yours, local, and never committed.

## Run protocol

For each prompt:

1. Run the `svg` skill with the row's **brief** (subject / weight / hue).
2. Render both your loop output and `quiver-refs/<slug>.svg` to PNG with `../../scripts/render.sh`.
3. Compare side by side. Score the loop output against the quiver reference on: subject fidelity, depth/dimensionality, palette cohesion, and overall "would I ship this."
4. Record the win/tie/loss. The aggregate feeds the kill-criterion `[THRESHOLD]` in `../../references/inspection-rubric.md`: if the loop loses badly across the set, ship style-system-only rather than the loop.

This is a human-in-the-loop eyeball comparison, not an automated metric — vector aesthetics don't reduce to a single number. Keep the scoring notes wherever you like (also local; don't commit reference-derived material).
