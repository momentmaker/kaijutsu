# Brief schema — the loop's input contract

A brief is how the human (or calling agent) expresses intent. Resolve it into a single token source *before* generating, so the loop and the set share one style.

## Single-icon brief

| Field | Required | Default | Notes |
|---|---|---|---|
| `subject` | yes | — | What the icon depicts, in plain words ("a bell", "a layered cake", "a shipping box"). |
| `weight` | no | `dimensional` | `flat` \| `soft` \| `dimensional`. Controls only the depth-stack (see `house-style.md` §5). |
| `base_hue` | no | token default lane | A hue (HSL degrees) or a named semantic (`brand-primary`, `success`, `danger`, …). Feeds the palette lane (`house-style.md` §2). |
| `size` | no | 24 | Target render size in px; the `viewBox` stays `0 0 24 24` regardless. |
| `background` | no | `light` | Intended background lightness (`light` \| `dark`) — affects shadow/contrast choices. |
| `intent` | no | `semantic` | `semantic` \| `decorative`. Drives accessibility output (see SKILL.md "Finalize output"). |

**Missing `base_hue`:** apply the documented default lane from `assets/tokens.json` (`palette.base_hue`). Never invent a different hue per icon — that breaks set cohesion. *(AE7)*

## Set brief

A set is **a list of subjects sharing one `base_hue` and one `weight`, delivered in a single invocation.** Resolve the shared token source once, then generate each subject against it.

```
weight: dimensional
base_hue: brand-primary
subjects: [bell, gear, shield, bolt]
```

Because the token source is derived **before** the first icon is generated, all N icons share the same grid, stroke, lane, and depth recipe — they read as one family, not N independent styles. *(R10, AE4)*

For a status set where hues differ per semantic (error/warning/success), vary only the hue; keep lane construction, angle, geometry, and weight identical (`house-style.md` §2).
