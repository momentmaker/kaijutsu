# Calibration set

A small, pre-labeled set for the calibration gate in `references/inspection-rubric.md`. Not a harness — a handful of icons with a known human verdict, used to measure whether the agent's render-judgment agrees with a human before the loop's quality claim is trusted.

This is the **agent-vs-human** tier (does the agent judge like a person?). The separate **loop-vs-quiver** quality benchmark lives in `benchmark/` — see `benchmark/README.md`.

## How to run the probe

For each case below: render the SVG with `scripts/render.sh`, have the agent judge the **rendered PNG** against the rubric for the stated brief, and compare the agent's PASS/FAIL to the expected verdict. Agreement across the set must clear the gate's `[AGREEMENT_RATE]` (set at eval time) before the loop is trusted; otherwise fall back to style-system-only output.

A good calibration set covers every weight as a PASS and at least two distinct FAIL modes (missing depth, and unrecognizable subject). Each verdict must be unambiguous — a case a human would call borderline teaches the gate nothing.

## Cases

| File | Brief | Expected verdict | Why |
|---|---|---|---|
| `cases/good-bell-dimensional.svg` | bell, `dimensional` | **PASS** | Body gradient, off-center radial highlight, soft drop shadow, light bevel edge — the full depth-stack reads. |
| `cases/good-shield-soft.svg` | shield, `soft` | **PASS** | Body gradient + soft drop shadow, no bevel/gloss — correct reduced depth-stack for `soft`. Confirms the rubric accepts soft as its own weight. |
| `cases/good-gear-flat.svg` | gear, `flat` | **PASS** | Recognizable toothed gear (ring + 8 teeth + center hole), clean flat fill on the grid, correct palette, no depth artifacts. Confirms the rubric is weight-aware (flat is not "failed dimensional"). |
| `cases/flawed-bell-flat.svg` | bell, `dimensional` | **FAIL** | Flat solid fill at dimensional weight — no gradient, no shadow, no highlight. The "flat/generic" failure mode. |
| `cases/flawed-gear-unrecognizable.svg` | gear, `flat` | **FAIL** | A 12-point sunburst with a center circle — reads as a star/sun, not a gear. Fails the **subject-recognizable** box even though it's clean and on-grid. (Authored blind as a "gear" and only caught on render — exactly why the loop exists.) |

Extend the set by adding a labeled SVG under `cases/` and a row here. Always render-and-look before labeling: an SVG that *reads* wrong to a human must be labeled FAIL regardless of how clean its source looks.
