# ADR: `svg` skill — premium SVG icons via a render-see-fix loop

**Date:** 2026-06-15
**Status:** Accepted; implemented on `feat/svg-skill`, pending release
**Origin:** [`docs/brainstorms/2026-06-14-svg-icon-visual-convergence-primitive-requirements.md`](../brainstorms/2026-06-14-svg-icon-visual-convergence-primitive-requirements.md)
**Plan:** [`docs/plans/2026-06-14-001-feat-svg-icon-visual-convergence-plan.md`](../plans/2026-06-14-001-feat-svg-icon-visual-convergence-plan.md)
**Author:** rubberduck

## Context

A generic coding agent generates SVG *blind* — it never sees what it drew — so icons come out flat and generic and take several re-rolls. The quality bar people admire (Quiver / Arrow 1.0) is a purpose-trained, multimodal SVG model; a prompted generic agent cannot match its raw generation.

But the two pains are not generation-horsepower problems. "Flat/generic" is a missing **style system**; "several tries" is a missing **feedback loop**. Both are mechanical and reproducible without a specialized model. A `dream → deep-research → brainstorm → doc-review` pass established this framing and surfaced three premise challenges (recorded as deferred questions below).

`svg` is the registry's **first design-asset skill** — every prior core skill is a coding-*workflow* primitive. That identity stretch is real and is the reason the decisions below lean conservative.

## Decision

1. **Concrete, factorable icon loop now — defer the generic primitive interface.** The origin framed a generic "visual-convergence primitive." Three reviewers (scope-guardian, product-lens, adversarial) and AGENTS.md's "don't build beyond the stage" agreed: build a concrete SVG-icon loop that is *cleanly factorable* (self-contained loop section + scripts), and promote it to a shared primitive only when a second consumer (charts/diagrams) constrains the interface. This resolves the origin's [P1] premature-abstraction flag.

2. **Render-see-fix loop with two stop branches.** Branch (a) — the render *satisfies the brief* against a concrete rubric — can fire on round 1. Branch (b) — *no further improvement* — composes `convergence-detect` on the **patch-set** only, honoring its 3-round floor. `convergence-detect` is a text-drift detector; it cannot judge a render, so the visual-satisfaction signal is new in-skill logic, not reuse. A hard iteration cap (6) bounds the loop.

3. **Data-driven house-style system.** Geometry / palette / gradient / depth as DTCG-leaning tokens (`assets/tokens.json`) plus an application recipe (`references/house-style.md`), with a **flat→soft→dimensional weight dial** that scales only the depth-stack. Premium depth is achieved by **layered compositing** (shadow + gradient + off-center radial highlight + bevel) — explicitly **not** `feSpecularLighting` alone (a popular claim our research adversarially refuted).

4. **Graceful degrade, never hard-fail.** The visual loop needs a rasterizer (`scripts/render.sh` probes resvg → librsvg → chromium → cairosvg, engine-matched to the shipping target). Absent → emit style-system output and report the loop was skipped (exit 3 is a signal, not an error). Optimization (`scripts/optimize.sh`, svgo, after the loop only) falls back to the unoptimized SVG and refuses to ship one whose `viewBox`/`<title>` it stripped.

5. **Falsifiable quality bar.** "Premium enough" is a concrete per-weight checklist (`references/inspection-rubric.md`), backed by a kill-criterion (loop must beat single-shot ad-hoc in blind comparison, else ship style-only) and a calibration gate (the agent's render-judgment must agree with human labels on the `evals/` set before the loop is trusted). Thresholds are filled when the eval first runs.

6. **Mechanics.** Rich layout (`references/`, `assets/`, `scripts/`); permissions `{bash: true, network: false, fs-write: scoped}` (same triple as `convergence-detect`); composes `convergence-detect@^0.2`; `trust.expected-signer: kaijutsu-core@github` (CI signs on tag). Single agent-agnostic `SKILL.md`, no swarm.

## Why these together

- **Concrete-over-generic** keeps v1 honest to the stated goal (kill flat/generic + several tries) and avoids designing an interface from one example. Factorability preserves the upside without paying the abstraction cost now.
- **Two-branch stop** is the load-bearing correction from review: a single "converged" signal over text would terminate before the *image* is good (or never, on cosmetic churn). Separating "is it good?" (visual, round-1 ok) from "is it still improving?" (text patch-set, 3-round floor) is what makes the loop both fast and correct.
- **Rubric + kill-criterion + calibration** answer the sharpest adversarial finding: the whole skill rests on the agent reliably seeing its own render. Rather than assume it, the skill makes that premise measurable and falls back to style-only if it fails.

## Deferred questions (carried for follow-up)

- **Identity fit:** reconfirm `skills/core` vs community/personal placement before tagging — this is the registry's first asset generator.
- **Arrow-inversion:** if Quiver/Arrow ships as an agent-callable tool, an agent could call it *inside* this loop, making the generation half redundant while the loop (the claimed edge) is trivially copyable. Revisit whether the loop is a defensible edge or a commodity wrapper.
- **Primitive promotion:** extract a shared `visual-convergence` primitive when charts/diagrams land.
- **Repo-wide gap (out of scope here):** CI does not validate any `skill.yaml` against `skills/skill.schema.json`, and the Go loader skips `shared`/`permissions`/`deps`/`trust` — a maintainer may want a repo-level `ajv-validate-skills` step.
