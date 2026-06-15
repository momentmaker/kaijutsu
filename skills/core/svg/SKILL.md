---
name: svg
description: Generate premium, dimensional SVG icons via a render-see-fix loop — generate, rasterize, inspect the render against a quality rubric, patch, converge — driven by a data-driven house-style token system with a flat→dimensional weight dial. Use when asked to make an SVG icon, an icon set, or a logo mark, or when ad-hoc generated SVG comes out flat/generic. Degrades to style-system-only output when no rasterizer is present.
---

# svg

A generic agent generates SVG *blind* — it never sees what it drew — so icons come out flat and generic and take several re-rolls. This skill fixes both: a **house-style token system** (so output is premium and cohesive, not flat) and a **render-see-fix loop** (so the agent looks at its own render and converges instead of you correcting it). It will not out-generate a purpose-trained SVG model; its edge is the loop plus the system, not raw generation.

## When to invoke

- The user asks to make an SVG icon, an icon set, a logo mark, or a small vector graphic for a page.
- Ad-hoc generated SVG came out flat/generic and needs to look designed.
- Another skill says "use `svg` to produce the icon/mark."

## How it works

Three steps, in order:

1. **Read the brief** — resolve the subject, weight, and palette into a single token source (`references/brief-schema.md`, `assets/tokens.json`).
2. **Run the render-see-fix loop** — generate from the tokens, rasterize, inspect the render against the rubric, patch, converge (`references/inspection-rubric.md`).
3. **Finalize output** — optimize after the loop, add accessibility, validate.

The house-style system that makes output premium (geometry, palette, gradient, depth, the weight dial, and the anti-slop guardrail) lives in `references/house-style.md`; the token data it operates on lives in `assets/tokens.json`. Read those before generating.

## 1. Read the brief

Resolve the brief into one token source before generating. The full schema is in `references/brief-schema.md`; the minimum is:

- **`subject`** (required) — what to depict, in plain words.
- **`weight`** — `flat` | `soft` | `dimensional` (default `dimensional`). Scales only the depth-stack.
- **`base_hue`** — a hue or named semantic; missing → the default lane in `assets/tokens.json` (never invent a per-icon hue).
- **`intent`** — `semantic` (default) | `decorative`; drives accessibility output.

For a **set**, take the list of subjects sharing one `base_hue` and `weight`, resolve the shared token source **once**, then generate each subject against it. Deriving the source before the first icon is what makes the set cohere.

## 2. The render-see-fix loop

Generate, then *look*, then fix — never ship a render the agent hasn't seen. One icon per loop (a set runs this loop per subject against the shared token source).

1. **Generate** a candidate SVG by applying the resolved tokens (`assets/tokens.json`) through the recipe in `references/house-style.md` at the brief's weight. Use `viewBox="0 0 24 24"`.
2. **Rasterize** with `scripts/render.sh <candidate.svg> <out.png> [size]`.
   - It prints `engine=<family>` to stderr — confirm that family matches where the SVG will ship (filter/gradient rendering diverges across engines; an approved render only predicts the browser when the engines match).
   - **If it exits `3` (`RASTERIZER_ABSENT`):** take the degrade path — emit the current style-system SVG, tell the user the visual-verification loop was skipped, and go straight to step "Finalize output." Do not error.
3. **Inspect** the rendered PNG against `references/inspection-rubric.md` for the brief's weight.
4. **Branch (a) — satisfied?** If every applicable rubric box passes, the icon is done — go to "Finalize output." This can fire on round 1; it is not subject to any minimum-rounds rule.
5. **Branch (b) — not satisfied?** Patch the SVG to fix the specific failed boxes, then re-render (back to step 2). Keep each round's SVG so the patch-set can be compared.
   - Once **3+ rounds** have run without branch (a) firing, invoke `convergence-detect` on the **patch-set** (treat each round's SVG as the "item" — are successive patches just restating each other?). If it returns CONVERGED, stop: the loop is no longer improving the render, so emit the best round and note it converged-without-satisfying. `convergence-detect` governs only this branch and only its own 3-round floor — it cannot judge whether the render satisfies the brief (that is branch (a)).
6. **Iteration cap.** Never exceed a hard cap (default **6** rounds) regardless of branch. At the cap, emit the best render so far and report `capped`.

**Loop-state sidecar.** Emit loop telemetry as a JSON sidecar (markdown on a TTY, JSON when piped), `lower_snake_case` per `SCHEMA.md`:

```json
{"skill": "svg", "schema_version": "0.1.0", "round": 3, "verdict": "converged", "stop_branch": "a"}
```

`verdict` ∈ `converged` | `capped` | `skipped` (no rasterizer); `stop_branch` ∈ `a` (satisfied) | `b` (patch-set converged) | `none` (capped/skipped).

## 3. Finalize output

Run once, on the converged (or best/degraded) SVG:

1. **Optimize** — `scripts/optimize.sh <converged.svg> <final.svg>`. It runs svgo when present and falls back to the unoptimized SVG otherwise (and refuses to ship an SVG whose `viewBox` or `<title>` it stripped). Never run this mid-loop.
2. **Accessibility — driven by the brief's `intent`:**
   - `semantic` (default): include a `<title>` whose text is the subject, and set `role="img"` on the root `<svg>`.
   - `decorative`: set `aria-hidden="true"` on the root and **omit** the `<title>`.
3. **Validate** before handing back:
   - Vector **paths only** — no `<image>` elements and no base64-embedded rasters.
   - `viewBox="0 0 24 24"` present.
   - Well-formed XML.

Emit the final SVG (and, for a set, the family of SVGs). If the loop ran degraded (no rasterizer) or hit the cap, say so alongside the output so the user knows the visual gate did not fully fire.

## Hard rules

- **The agent must see its own output.** When a rasterizer is available, never declare an icon done without inspecting the rendered PNG against `references/inspection-rubric.md`. Generating-and-shipping blind is the failure mode this skill exists to remove.
- **Degrade, never hard-fail.** No rasterizer → emit style-system output, state that the visual loop was skipped, and stop. No optimizer → emit unoptimized output with a note. Never error out because a tool is missing.
- **One house style per set.** Every icon in a set resolves the same token source so the set reads as one family — do not re-derive geometry/palette per icon.
- **Optimize after the loop, never during.** SVGO-style optimization runs once, on the converged SVG. Optimizing mid-loop destroys structure the next patch needs.
- **Layered compositing, not a single filter.** Premium depth comes from stacking shadow + gradient + highlight + bevel — not from `feSpecularLighting` alone.
- **Agent-agnostic.** No agent-specific branches, no swarm dependency. The same `SKILL.md` runs on every supported agent.

## Provenance

The render-see-fix loop applies the convergence discipline from `convergence-detect` to a *visual* artifact: the agent rasterizes its own output and critiques the render, rather than trusting blind generation. The house-style system is grounded in published icon-design practice (Material/Apple/Microsoft grids, DTCG composite tokens, layered SVG filter recipes) — see `references/house-style.md` for sources.
