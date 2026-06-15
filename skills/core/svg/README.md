# svg

Generate premium, dimensional SVG icons through a render-see-fix loop: the agent generates SVG from a house-style token system, rasterizes it, inspects the render against a concrete quality rubric, patches, and converges. Fixes the two failure modes of ad-hoc SVG generation — flat/generic output (a missing style system) and several re-rolls (a missing feedback loop).

Install:
```bash
jutsu install svg
```

Composes `convergence-detect` for the patch-set stop signal. Needs a rasterizer (resvg / headless browser) for the visual loop; degrades to style-system-only output when absent. Trigger phrases: "make an SVG icon", "generate an icon set", "icon for X", `/svg`.
