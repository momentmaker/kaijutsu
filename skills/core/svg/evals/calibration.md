# Calibration set

A small, pre-labeled set for the calibration gate in `references/inspection-rubric.md`. Not a harness — a handful of icons with a known human verdict, used to measure whether the agent's render-judgment agrees with a human before the loop's quality claim is trusted.

## How to run the probe

For each case below: render the SVG with `scripts/render.sh`, have the agent judge the **rendered PNG** against the rubric for the stated brief, and compare the agent's PASS/FAIL to the expected verdict. Agreement across the set must clear the gate's `[AGREEMENT_RATE]` (set at eval time) before the loop is trusted; otherwise fall back to style-system-only output.

## Cases

| File | Brief | Expected verdict | Why |
|---|---|---|---|
| `cases/good-bell-dimensional.svg` | bell, `dimensional` | **PASS** | Body gradient, off-center radial highlight, soft drop shadow, light bevel edge — the full depth-stack reads. |
| `cases/flawed-bell-flat.svg` | bell, `dimensional` | **FAIL** | Flat solid fill at dimensional weight — no gradient, no shadow, no highlight. The exact "flat/generic" failure the skill exists to catch. |
| `cases/good-gear-flat.svg` | gear, `flat` | **PASS** | Correctly flat for `flat` weight: clean on-grid fill, correct palette, no depth artifacts. Confirms the rubric is weight-aware (flat is not "failed dimensional"). |

Extend the set by adding a labeled SVG under `cases/` and a row here. Keep verdicts unambiguous — a case a human would call borderline teaches the gate nothing.
