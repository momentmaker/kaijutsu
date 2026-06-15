---
date: 2026-06-14
topic: svg-icon-visual-convergence-primitive
---

# SVG Icon Generation via a Visual-Convergence Primitive

## Summary

A reusable **visual-convergence primitive** for kaijutsu — *generate → render → look → fix → converge* — with a **premium SVG icon house-style system as its first consumer**. The style system encodes geometry, palette, gradients, and a dimensional depth-stack as data-driven tokens with a flat→dimensional weight dial, so a generic agent produces cohesive, premium icon *sets* without a specialized model.

---

## Problem Frame

When an AI agent makes an icon or graphic for a web page today, it generates SVG ad-hoc and *blind* — it never sees what it drew. Two costs follow. First, output is **flat and generic**: no depth, no gradient richness, nothing that makes a page feel designed. Second, it **takes several tries**: the human re-rolls and corrects because the agent can't tell its own output missed.

The quality bar that makes this worth solving is set by purpose-built tools like Quiver / Arrow 1.0 — a model *trained* to write structured, layered, gradient-rich SVG. A generic coding agent cannot out-generate a specialized model by prompting. But the two pains above are not generation-horsepower problems: "flat/generic" is a missing *style system*, and "several tries" is a missing *feedback loop*. Both are mechanical, and both are reusable beyond icons.

---

## Actors

- A1. Invoking agent: the AI coding agent (Claude / Codex / Antigravity) that reads the skill and runs the loop.
- A2. Human author: the developer who wants premium icons/graphics for a page and judges the result.
- A3. Rasterizer: external tool (resvg / headless browser) that renders SVG→PNG so A1 can visually inspect its own output.

---

## Key Flows

- F1. Single icon, render-see-fix
  - **Trigger:** A2 asks for an icon from a brief (subject + intended weight).
  - **Actors:** A1, A3
  - **Steps:** A1 generates a candidate SVG from the house-style tokens → A3 rasterizes it → A1 inspects the render against the brief → A1 patches the SVG → re-render → repeat until converged or the iteration cap is hit.
  - **Outcome:** A converged, on-style SVG that matched the brief on a render the agent actually saw.
  - **Covered by:** R1, R3, R5, R8, R11

- F2. Cohesive set
  - **Trigger:** A2 asks for multiple icons that belong together.
  - **Actors:** A1, A3
  - **Steps:** A1 derives one token source (grid, stroke, palette lane, weight) → generates each icon against that shared source → runs F1's loop per icon.
  - **Outcome:** N icons that read as one designed family.
  - **Covered by:** R6, R7, R9, R10

- F3. Degraded run (no rasterizer)
  - **Trigger:** F1/F2 starts but no rasterizer is available.
  - **Actors:** A1
  - **Steps:** A1 produces output from the style system without the visual-verification pass → reports that the visual loop was skipped.
  - **Outcome:** Style-consistent SVG, no hard failure, the human knows verification didn't run.
  - **Covered by:** R4

---

## Requirements

**Visual-convergence primitive (the reusable core)**
- R1. Provide a render-see-fix loop: generate candidate → rasterize to PNG → visually inspect the render against the brief → patch → re-render, iterating until converged.
- R2. Expose a generic, artifact-agnostic interface (brief in, converged artifact out) so future consumers (charts, diagrams) reuse the loop without modifying it. Only the SVG-icon consumer is implemented in v1.
- R3. Stop on the first of: (a) the render satisfies the brief — a render-vs-brief check that can terminate at round 1 — or (b) no meaningful improvement occurs across a round; always bounded by a hard iteration cap that is a safety boundary, not a target. `convergence-detect` is a text-drift primitive (token deltas, item similarity, 3-round minimum); it can gate branch (b) on the *patch-set* (are successive SVG edits restating each other) but cannot judge render-vs-brief satisfaction. The visual-satisfaction signal in branch (a) is new work this skill provides, not reuse, and is not subject to the 3-round floor.
- R4. When no rasterizer is available, degrade gracefully: produce output from the style system without the visual-verification pass and surface that the loop was skipped — never hard-fail.

**House-style system (the icon consumer)**
- R5. Encode the house style as data-driven tokens covering geometry, palette, gradient, and depth, so a set shares one style source. DTCG composite `gradient` + `shadow` token types are the leading candidate format (research-grounded); the final token format is chosen in planning against what the agent loop actually needs to consume.
- R6. Geometry tokens: a fixed keyline grid with constant corner radius and stroke width applied across every icon in a set, with optical (not purely mathematical) alignment.
- R7. Palette tokens: procedural tonal lanes from a base hue; gradient pairs drawn from adjacent steps on a single lane at a consistent angle — no arbitrary multi-hue gradients.
- R8. Gradient + depth recipe: a linear body fill plus an off-center radial highlight, composited with a soft drop shadow and beveled edges (dark + light) to produce dimensionality — via layered compositing, not a single lighting filter. Because filter/gradient primitives render differently across engines, the in-loop rasterizer must be the same engine family the SVG targets (or the recipe must avoid primitives known to diverge), so a render the agent approves predicts what the user's browser ships.
- R9. Weight dial (flat → soft → dimensional) controls only the depth-stack; geometry, palette, and cohesion stay constant across weights. Dimensional is the default.
- R10. Set cohesion: when generating multiple icons, they share one token source so the set reads as one family.

**Output quality + portability**
- R11. Output is well-formed, optimized SVG — optimization runs *after* the loop, not mid-loop — with vector paths only and no embedded rasters. Accessibility follows the brief's `intent` field (R13): semantic icons carry a `<title>` derived from the subject plus an appropriate role; decorative icons set `aria-hidden="true"` and omit the title.
- R12. Agent-agnostic: the skill is a markdown document usable by any supported agent, with no single-agent assumptions and no swarm dependency.

**Brief / input contract**
- R13. Define a minimum brief schema the human (A2) supplies: required — subject description, weight (`flat` | `soft` | `dimensional`, default `dimensional`); optional — base hue or named semantic (e.g. `brand-primary`), icon size, intended background lightness, and an `intent` field (`decorative` | `semantic`) that drives accessibility output (R11). The base hue feeds R7's palette lane.
- R14. Define a set brief: a list of subjects sharing one base hue and one weight, delivered in a single invocation, so the shared token source (R10) is derived before generation starts rather than inferred across requests.
- R15. Anti-slop guardrail: the house-style system includes compositional guidance that differentiates output from default AI icon shapes — a characterized premium reference (what makes the target look premium beyond "dimensional") plus at least one structural rule (e.g. deliberate asymmetry, off-center balance, a signature element). Uniform shadow + gloss + bevel applied identically across a set is itself the AI-slop signature this guards against.

---

## Acceptance Examples

- AE1. **Covers R4.** Given no rasterizer on the system, when the skill runs, it emits SVG from the style system and reports the visual-verification pass was skipped — no hard failure.
- AE2. **Covers R9.** Given `weight = flat`, when an icon is generated, the output omits the depth-stack (no drop shadow, bevel, or gloss) yet keeps the same grid, stroke width, and palette as its dimensional counterpart.
- AE3. **Covers R1, R3.** Given a brief and an available rasterizer, when the first render misses the brief, the loop patches and re-renders until it converges or hits the iteration cap.
- AE4. **Covers R10.** Given a request for N icons, when generated, all share one token source and read as a cohesive family rather than N independent styles.
- AE5. **Covers R9.** Given `weight = soft`, when an icon is generated, the output includes a reduced depth-stack (e.g. soft drop shadow and body gradient present; bevel and radial highlight omitted or at reduced intensity) — visibly distinct from both flat (no depth-stack) and dimensional (full depth-stack), on the same grid, stroke, and palette.
- AE6. **Covers R12.** Given the published skill, when invoked by Claude, Codex, or Antigravity, the loop runs without agent-specific code branches or swarm dependencies.
- AE7. **Covers R13.** Given a brief missing an optional base hue, when an icon is generated, the skill applies a documented default lane rather than failing or inventing an inconsistent hue per icon.

---

## Success Criteria

- **Human outcome:** icons land premium/dimensional enough to use on the first or near-first converged output — directly killing the "flat/generic" and "takes several tries" pains that motivate the work.
- **Visual quality bar (the loop's inspection rubric):** "premium/dimensional enough" is made concrete as an observable checklist the inspection step (F1) checks against. At dimensional weight: a body gradient is present (not a flat solid fill), the drop shadow has a non-zero offset, at least one bevel/highlight edge is visible, and the icon sits on the keyline grid. This checklist is what R1/R3 mean by "satisfies the brief" for the depth dimension — without it, the agent grades its own render with no standard.
- **Loop-vs-ad-hoc kill-criterion (falsifiable):** in a blind comparison, human raters prefer loop output over the agent's single-shot ad-hoc output at a rate set in planning. If the loop does not measurably beat ad-hoc, the render-loop is not worth its cost and the icon consumer ships as a style-system-only skill.
- **Inspection-reliability gate:** before the full kit is trusted, a calibration probe confirms the agent's render-critique agrees with human judgment on pre-labeled good/flawed icons above a planning-set rate. If it fails, the agent-sees-its-own-render premise is unproven and the loop's value claim is on hold.
- **Downstream handoff:** `ce-plan` (or kaijutsu's spec stage) can build the primitive + icon consumer without inventing the loop interface, the conceptual token structure, the weight-dial semantics, or the degrade behavior.

---

## Scope Boundaries

### Deferred for later

- Charts / diagram consumers of the primitive — the loop is built generic to accommodate them, but only the icon consumer ships in v1.
- Animation / "dynamic" motion (SMIL / CSS keyframes).

### Outside this product's identity

- Training or matching a specialized SVG model (Arrow 1.0). The edge is the loop + style system, not raw generation horsepower.
- Raster image → SVG vectorization / tracing.
- A swarm / multi-agent generation variant.
- A general third-party brand-theming engine beyond the weight dial.

---

## Key Decisions

- **Visual-convergence primitive over a plain SVG skill** — the loop gets reused for charts/diagrams and earns a clean kaijutsu slot as a *primitive*, not the registry's first asset generator.
- **Whole kit including the depth-stack in v1** — premium dimensionality is the core value, not a v2 add-on.
- **Weight dial with a premium default** — one system serves both hero graphics and functional UI icons without forcing gloss everywhere.
- **Depth via layered compositing, not `feSpecularLighting` alone** — adversarial research killed (0–3) the claim that the specular filter is the single lever for premium dimensionality; the look comes from stacking shadow + gradient + highlight + bevel.
- **Graceful-degrade when no rasterizer is present** — preserves agent-agnostic portability rather than making a heavy dependency mandatory.

---

## Dependencies / Assumptions

- The visual loop requires a rasterizer (resvg / headless browser). Assumed present for full value; degrades to style-system-only when absent (R4).
- The skill shells out (rasterizer, SVG optimizer), so it declares sensitive permissions and triggers kaijutsu's install-time permission prompt.
- Lands in kaijutsu; adding a skill triggers catalog regeneration (`docs/index.html` + `docs/skills.json`) per project convention, or the catalog-drift CI check fails.
- The generic loop interface is assumed able to serve charts/diagrams later, but is validated against icons only in v1.

---

## Outstanding Questions

### Deferred to Planning

- [Affects R4][Needs research] Which rasterizer to standardize on (resvg-js vs rsvg-convert vs headless chromium vs cairosvg) — fidelity vs install weight.
- [Affects R5][Technical] Exact token schema layout and how the agent consumes it inside the loop.
- [Affects R2][Needs research] The minimal generic loop interface that won't need reshaping when chart/diagram consumers land.
- [Affects R8][Needs research] Concrete filter/gradient recipe values tuned to render genuinely premium — the research gives the structure (off-center radial highlight, blur→offset→merge shadow, beveled edges); the exact numbers need visual tuning in the loop.
- [Affects R3][Technical] How `convergence-detect` composes as the stop condition vs a bespoke iteration cap. (Partly resolved by R3's reframe: it gates the patch-set branch only; planning confirms the exact composition.)

---

## Deferred / Open Questions

### From 2026-06-14 review

- **[P1] Primitive-first may be premature abstraction (R2 / Key Decisions).** Three reviewers (scope-guardian ×2, product-lens, adversarial) flag that building a generic, artifact-agnostic loop interface for chart/diagram consumers that are explicitly deferred contradicts AGENTS.md "don't build beyond the stage," and the interface is validated against icons only. The primitive-first direction was chosen deliberately in brainstorm Phase 2; revisit in planning whether v1 ships a concrete, factorable icon loop (not a frozen generic seam) with primitive promotion deferred until a second consumer constrains the interface.
- **[P1] Identity fit: registry's first asset generator (Problem Frame / Summary).** product-lens notes every existing core skill is a coding-workflow primitive; this is the first design-asset generator, and "primitive" is the load-bearing justification for the fit. Landing in kaijutsu was decided in the dream; reconfirm kaijutsu vs personal-collection placement before committing registry ceremony (catalog regen, signing, permission prompt).
- **[P1] "Loop beats Arrow" may invert if Arrow/quiver ships as a callable tool (Scope Boundaries).** product-lens: if Arrow becomes agent-callable, an agent could invoke it *inside* the same render-loop — making the skill's generation half redundant while the loop (the claimed edge) is trivially copyable. Add an inversion analysis establishing why the loop is a defensible edge rather than a commodity wrapper.
