# Benchmark prompts

Text only (safe to commit). For each row: paste the **Quiver prompt** into quiver and save the result to `quiver-refs/<slug>.svg` (git-ignored); run the `svg` skill with the **Brief**; compare per `README.md`.

The set spans all three weights and a range of subjects (simple glyph, mechanical, organic, container, symbol) so the comparison isn't biased toward one shape family.

| slug | Quiver prompt | Brief (subject / weight / hue) |
|---|---|---|
| `bell-dimensional` | "a notification bell icon, glossy and dimensional, purple gradient" | bell / dimensional / 250 |
| `gear-flat` | "a settings gear icon, flat, single color, clean" | gear / flat / 250 |
| `shield-soft` | "a security shield icon, subtle depth, purple" | shield / soft / 250 |
| `rocket-dimensional` | "a rocket launch icon, premium, dimensional, vibrant" | rocket / dimensional / 250 |
| `camera-dimensional` | "a camera icon, glossy, dimensional, indigo" | camera / dimensional / 250 |
| `bolt-dimensional` | "a lightning bolt icon, energetic, dimensional gradient" | bolt / dimensional / 40 |
| `folder-flat` | "a folder icon, flat, minimal, single color" | folder / flat / 210 |
| `heart-soft` | "a heart icon, soft depth, warm gradient" | heart / soft / 8 |

## Set briefs (cohesion check)

For testing set cohesion (R10 / AE4), use one multi-subject brief and compare the family against a quiver-generated set of the same subjects:

| slug | Quiver prompt | Brief |
|---|---|---|
| `status-set` | "a matching set of status icons: success, warning, error, info — flat, consistent style" | subjects: [check, warning-triangle, x-circle, info-circle] / flat / per-semantic hue (success, warning, danger, info) |

Add rows as needed; keep the prompt text natural-language (as a user would type into quiver) and the brief in the skill's structured form.
