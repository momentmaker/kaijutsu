# Inspection rubric — what "satisfies the brief" means

This is the loop's stop signal for branch (a). Without it, the agent grades its own render with no standard and converges on whatever it produced. Apply it against the **rendered PNG** (from `scripts/render.sh`), not the SVG source — the whole point is to see what actually rasterized.

## The checklist

An icon **satisfies the brief** only when ALL of these hold:

**Always (every weight):**
- [ ] The subject is **recognizable** as what the brief asked for.
- [ ] It sits on the **keyline grid** and is **optically centered** (not bounding-box centered).
- [ ] The subject **fills the live area** (~4→20 on both axes, optically sized) — it does not float small in the center of the frame.
- [ ] Stroke width and corner radius match the set's tokens (consistent with siblings).
- [ ] Palette is from the resolved lane — no off-lane or arbitrary hues.
- [ ] No rasterization artifacts: no clipped edges, no banding in gradients, no stray paths.

**At `dimensional` weight, additionally:**
- [ ] A **body gradient** is present — not a flat solid fill.
- [ ] The **drop shadow** has a visible, non-zero offset (soft, short — not hard clip-art).
- [ ] At least one **bevel or highlight edge** is visible (the off-center light core reads).

**At `soft` weight:**
- [ ] Body gradient + soft drop shadow present; **no** hard bevel; highlight subtle or absent.

**At `flat` weight:**
- [ ] Clean fill on the grid with the correct palette; **no** depth-stack artifacts (no shadow, bevel, or gloss).

**Anti-slop (every weight):**
- [ ] The composition is not a generic centered-symmetric stock glyph — it carries the set's signature element / deliberate asymmetry (`house-style.md` §6).

If any box fails, the render does **not** satisfy the brief — patch and re-render. Only when every applicable box passes does branch (a) fire.

## Kill-criterion (is the loop worth running at all?)

The whole skill bets that the loop beats blind ad-hoc generation. Make that falsifiable:

- In a **blind comparison**, human raters prefer loop output over the agent's single-shot ad-hoc output at a rate of **≥ `[THRESHOLD]`** (set during eval; see Calibration).
- If the loop does **not** measurably beat ad-hoc, it is not worth its cost — ship the **style-system-only** path (the degrade output) and drop the loop.

## Calibration gate (can the agent see well enough to run the loop?)

The loop's value rests on the agent reliably judging its own render. Prove it before trusting it:

- Run the agent's pass/fail judgment against the **pre-labeled set in `evals/`** (known-good and known-flawed icons).
- The agent's verdicts must agree with the human labels at **≥ `[AGREEMENT_RATE]`** (set during eval).
- If agreement is below that bar, the "agent sees its own render" premise is unproven — hold the loop's value claim and fall back to style-system-only output until calibration passes.

> `[THRESHOLD]` and `[AGREEMENT_RATE]` are intentionally unset here — they are filled when the eval harness first runs (deferred-to-implementation per the plan). Do not ship the loop as "verified" with these unfilled; until then the loop runs but its quality claim is provisional.
