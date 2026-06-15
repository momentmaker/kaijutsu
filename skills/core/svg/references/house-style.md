# House style — how to turn tokens into a premium icon

Read this with `assets/tokens.json`. The tokens are the *values*; this is the *recipe* for applying them. Every icon in a set uses the same resolved tokens — that shared source is what makes a set read as one designed family rather than ten one-offs.

The numbers below are research-grounded defaults (Material 3 / Apple HIG / Microsoft Fluent icon grids; layered SVG filter recipes from MDN/W3C; DTCG composite token types). Tune them per brief, but keep them **constant across a set**.

## 1. Geometry — the cohesion backbone

- Build on a **24-unit keyline grid** (`viewBox="0 0 24 24"`), inside a **20-unit live area** (2-unit margin).
- Snap primary forms to shared **keyline shapes** (square, circle, vertical/horizontal rectangle) so unrelated icons still feel related.
- **Corner radius 2** by default (0 only for intentionally sharp marks). **Stroke width 2** (drop to 1.5 for fine interior detail). Both constant across the set.
- Align **optically, not mathematically** — nudge by eye so the mark looks centered (a triangle centered by bounding box looks left-heavy).

## 2. Palette — designed, not random

- Pick one **base hue** (from the brief, a named semantic, or the token default). Generate a **lane** of three tones: light / base / dark.
- A gradient pair is **two adjacent steps on the same lane** (e.g. light→dark) at a **consistent angle (~120°)**. This single rule is what separates a premium gradient from AI-gradient-vomit: never pair arbitrary hues.
- For a multi-purpose set (e.g. status icons), vary the *hue* per semantic but keep the *lane construction and angle identical* — that's how a red error icon and a green success icon still look like siblings.

## 3. Gradients — richness without a mesh

SVG has no native gradient mesh; approximate it by **layering two gradients**:

1. **Body**: a `linearGradient`, vertical (`x1="0" y1="0" x2="0" y2="1"`), light at the top stop, dark at the bottom stop.
2. **Highlight**: a `radialGradient` with an **off-center focal point** (`fx≈0.32 fy≈0.28` against `cx=0.5 cy=0.5`), white at full opacity fading to transparent by ~60%. Layered over the body, this is the glossy "light core" that reads as dimensional.

Keep stops few (2–3) and opacity transitions smooth — banding is the tell of a cheap gradient.

## 4. Depth — the layered stack (this is the premium part)

> Premium dimensionality comes from **stacking layers**, NOT from `feSpecularLighting` alone. (Adversarially-verified: the single-specular-filter claim is a myth.)

Compose, bottom to top:

1. **Drop shadow** — `feGaussianBlur` on `SourceAlpha` (stdDeviation ≈ 1.25) → `feOffset` (dy ≈ 0.75) → `feMerge` the blurred shadow under the source. Soft, low-opacity, short offset. A hard or far shadow looks like clip-art.
2. **Body gradient** (§3.1).
3. **Radial highlight** (§3.2).
4. **Bevel edges** — a thin light edge on the top/left inner rim and a thin dark edge on the bottom/right inner rim. Opposing edges read as a raised, beveled surface.

## 5. The weight dial

The dial scales **only the depth-stack**. Geometry, palette, and cohesion never change with weight.

| Weight | Depth layers | Use |
|---|---|---|
| `flat` | none | minimal/functional UI icons (nav, settings) — solid or single-gradient fill on the grid |
| `soft` | body gradient + drop shadow (no bevel; highlight ≤ 0.25 opacity) | subtle depth without the full gloss |
| `dimensional` *(default)* | body gradient + highlight + drop shadow + bevel | hero/feature graphics — the full "wow" |

`flat` and `dimensional` must share the *same* grid, stroke, and palette — only the depth layers differ.

## 6. Anti-slop guardrail

A house style enforces consistency but does not, by itself, prevent every icon looking like generic stock AI output. Apply at least one structural rule so the set has a point of view:

- **Don't default to centered-symmetric single-path glyphs.** Use deliberate asymmetry or off-center balance.
- Give the family a **signature element** — a consistent perspective angle, a recurring negative-space cut, or a shared accent shape.
- **Uniform shadow + gloss + bevel applied identically to every icon is itself the slop signature.** Vary subject composition while holding the style tokens constant.
- Name a concrete *premium reference* for the brief (what specifically makes the target look premium beyond "dimensional" — restraint, optical correctness, tasteful depth) and steer the loop toward it, not toward maximal gloss.

## Sources

- Material Design 3 — system icon keyline grid, 2dp radius, stroke conventions.
- Apple HIG / Microsoft Fluent — icon construction grids, exterior/interior radii, restrained gradients.
- MDN / W3C SVG — `linearGradient`/`radialGradient` orientation and focal points; `feGaussianBlur`→`feOffset`→`feMerge` drop-shadow chain.
- DTCG draft — composite `gradient` and `shadow` token types (stored as data in `assets/tokens.json`).
