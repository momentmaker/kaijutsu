# Landing page content + visual spec — kaijutsu.dev v0.14.0

**Status:** Draft (pre-doc-review)
**Date:** 2026-05-09
**Driving ADR:** [`docs/decisions/2026-05-09-landing-page-direction.md`](../decisions/2026-05-09-landing-page-direction.md)
**Spec parent:** [`docs/specs/2026-05-09-v0.14.0-persona-browse-and-preset-usage.md`](./2026-05-09-v0.14.0-persona-browse-and-preset-usage.md) (§ Stage 3)

This spec carries the 14-point locked direction from the ADR into concrete copy, layout, and visual specs ready for `cli/cmd/sitegen/main.go` extension + `docs/index.html` rewrite. Doc-review pass on this file (claim-auditor-deepseek + architecture-purist-gemini) gates Stage 3c implementation.

## Page outline (locked section order)

1. **Hero** (above-the-fold) — pitch + install one-liner + 3 quick-link icons
2. **Single-agent failure-mode demo** — sycophancy vs disagreement-table side-by-side
3. **Quickstart** — `curl install` + one `jutsu swarm pr-review` invocation + partial output
4. **Cross-vendor portability** — same `swarm.yaml` runs on claude / codex / gemini / deepseek; agents.yaml provider catalog
5. **Cost per bug found** — concrete numeric example; addresses 3x-API-cost objection inline
6. **Trust model** — sigstore signing, install-time perm gates, findings.db privacy boundary, cynical-read defense
7. **Skill catalog** — preserved table, demoted to here from current top placement; pulls from `docs/skills.json`
8. **Contributing** — link to GitHub + CONTRIBUTING.md + how to author a skill
9. **Footer** — copyright, license, mascot one-liner, JSON-catalog link

Sticky-on-desktop nav with anchor links to sections 2–8 (hero excluded; clicking the logo returns to top).

## Section 1: Hero (above-the-fold)

### Copy (≤ 30 words)

> **Cross-vendor agentic CI for codebases.**
> Compose review pipelines once. Run them across Claude, Codex, Gemini, and DeepSeek from one config.

(20 words — well under 30. Contains "cross-vendor". Drops "AI-powered", "transform", etc.)

### Sub-copy under headline (1-line, optional warmth)

> One CLI. One `swarm.yaml`. Multi-model adversarial reviews that disagree on purpose.

### Install one-liner (existing pattern)

```
curl -fsSL https://kaijutsu.dev/install.sh | sh
```

Click-to-copy box, identical visual treatment to current page.

### Quick-link strip

GitHub · README · Schema · Roadmap · Security · skills.json (existing strip; unchanged).

### Visual

- Mascot at **120px** square (down from 160px), top-center.
- Headline below mascot, charcoal `#1A1A1A` on paper `#F5EDE0` background.
- Sub-copy in muted ink `#6B6B6B`.
- Install box: dark ink background, mint `#3DDC97` for code text, mint button accent.
- Asymmetric ink-brush stroke decoration to one side of headline (SVG, ≤ 4KB).
- Kanji 術 visible as a watermark element in the brush stroke OR as a small accent at the corner of the mascot.

### Hero acceptance check (against ADR checklist)

- ✅ Pitch ≤ 30 words (20).
- ✅ Contains "cross-vendor".
- ✅ No banned words (no "AI-powered" / "transform" / "revolutionize" / "powerful" / "unleash" / "magic" / "delight" / "blazing fast" / "supercharge" / "elevate" / "reimagine").
- ✅ Mascot ≤ 120px.
- ✅ Kanji 術 + asymmetric ink-brush element prominent.
- ✅ Ink-brush SVG ≤ 4KB (gate before merge: `wc -c docs/ink-brush.svg`).

## Section 2: Single-agent failure-mode demo

### Copy

> ## One agent says yes. Three say no.
>
> Single-agent review is sycophancy by default. Same prompt, same model, run twice — you usually get the same "looks good." Adversarial review needs structural disagreement, not just rerolls.
>
> kaijutsu fans the same diff out across multiple models, surfaces where they agree, and **flags exactly where they don't**. The disagreement table is the artifact you actually read.

### Visual: side-by-side artifact

Two columns (collapses to stacked on mobile <768px):

- **Left column — "Single-agent (Claude only)"**: a verbatim Claude pr-review output excerpt where it green-lit the v0.13 missing field-drop bug. Source: `~/.kaijutsu/findings.db` row from v0.13 final pr-review pre-fix; redact only PII.
- **Right column — "kaijutsu swarm (claude + gemini + deepseek)"**: the disagreement table that caught the bug. Three columns, three rows, ✓/✗ marks; deepseek caught the missing-field drop where claude and gemini missed it.

Caption under the artifact:

> Real run from v0.13.0 final pr-review. DeepSeek (paranoid-security persona) caught a `validateUserPresetEntry` field-drop bug Claude and Gemini both green-lit. The bug was real. We fixed it.

Link below caption: "View the full run output → [link to a public gist or GitHub commit comment]" (TBD asset URL; Stage 3c verifies before merge).

### Acceptance check

- ✅ Real artifact, not synthesized.
- ✅ Disagreement-table rendered (not just claimed).
- ✅ No "reviewed by N LLMs" badge.
- ✅ Voice: terse, technical, show-don't-tell.

## Section 3: Quickstart

### Copy

> ## Quickstart
>
> Install:
>
> ```sh
> curl -fsSL https://kaijutsu.dev/install.sh | sh
> jutsu init                    # detects active agents, writes .kaijutsu/
> ```
>
> Run a swarm pr-review on the current branch:
>
> ```sh
> jutsu swarm pr-review --diff-from-branch main
> ```
>
> Partial output:
>
> ```
> claude     │ ✓ │ ✗ │ ✓
> gemini     │ ✓ │ ✓ │ ✗
> deepseek   │ ✗ │ ✓ │ ✓
>
>            severity   file:line                summary
>            issue      auth.go:42               token expiry uses < not <=
>            issue      handlers/user.go:118     missing rate-limit on /verify
>            minor      schema.sql:9             index name shadows reserved word
> ```
>
> [Full quickstart guide →](https://github.com/momentmaker/kaijutsu/blob/main/README.md#quickstart)

### Acceptance check

- ✅ Curl install line.
- ✅ One `jutsu swarm pr-review` invocation.
- ✅ Partial output snippet (real disagreement-table format from `cli/internal/swarm/synth.go`).

## Section 4: Cross-vendor portability

### Copy

> ## One config. Every vendor.
>
> The `swarm.yaml` you write today is consumed by `jutsu`, which dispatches across Claude Code, Codex CLI, Gemini CLI, DeepSeek-via-HTTP, and whatever ships next. v0.6's driver abstraction handles the wire format per vendor; v0.13's Preset SDK lets you compose your own pipelines. Vendor lock-in stays a vendor problem.
>
> ```yaml
> # .kaijutsu/swarm.yaml — composable, portable, version-controlled
> version: 1
> presets:
>   - name: tight-pr-review
>     base: pr-review
>     personas:
>       - default-claude
>       - paranoid-security-claude
>       - default-gemini
>     mode: full
>     confidence_floor: 0.7
> ```
>
> ```sh
> jutsu swarm tight-pr-review --diff-from-branch main
> ```

### Visual

Side note callout (separate visual block, paper background, charcoal border):

> **Why this matters:** Multi-agent review is becoming a vendor commodity — Cursor, Zed, GitHub, every IDE will ship native versions by 2027. Cross-vendor portability is the part that doesn't commodify. Your pipeline is yours, regardless of which vendor wins.

### Acceptance check

- ✅ Code block shows real `swarm.yaml` format.
- ✅ "Cross-vendor" framing reinforced.
- ✅ No "swarm" used as headline word; "swarm" appears only as the existing CLI command name.

## Section 5: Cost per bug found

### Copy

> ## Cost per bug found
>
> Multi-agent review costs more per call than a single-agent run. The framing the spreadsheet wants is **cost per bug actually caught and confirmed real**, not cost per call.
>
> Real numbers from the v0.14.0 stage-2 doc-review run on the landing-page content spec (this very document):
>
> | Metric | Value |
> |---|---|
> | Models in pool | deepseek + gemini (2 personas) |
> | Total API cost | **$0.05** |
> | Wall time | **4m31s** |
> | Findings emitted | 10 |
> | Issue + minor (after triage) | 6 fixed in this spec, 4 skipped with documented rationale |
> | Cost per fixed issue | **~$0.008** |
>
> (Numbers updated post-merge with the v0.14.0 final swarm pr-review of the merged branch — kept as a live receipt, not aspirational.)
>
> A senior IC's billable hour catches more, but at a different price. Both numbers are useful; "cost per call" alone is not.
>
> "But isn't this 3x more expensive than a single-agent run?" Yes — and that's the point. Three models with different priors disagree about different things. The cost difference is what buys you the disagreement.

### Acceptance check

- ✅ Concrete numeric example.
- ✅ Addresses 3x-API-cost objection inline (not as standalone top-line feature).
- ✅ "cost per bug found" framing, not "cost per call".

## Section 6: Trust model

### Copy

> ## Trust model
>
> kaijutsu's trust surface has four parts:
>
> **Skill provenance.** Core skills (`skills/core/*`) are sigstore-signed at release. `jutsu install` verifies the signature before unpacking. Community skills are pinned to a tagged commit + SHA in `kaijutsu.lock.json`; tampering breaks the lockfile.
>
> **Permission gates.** `jutsu install` parses each skill's `permissions:` manifest before write. Bash, network, fs-write are explicit; `jutsu install` prompts on first use of a sensitive permission and records the decision.
>
> **Local data stays local.** `~/.kaijutsu/findings.db` (the quality fingerprinting store) is mode 0600, never leaves disk, never syncs anywhere. The `cli/internal/cli/finding.go` package is import-list-restricted from `net`, `net/http`, `net/url` — enforced as a build-time test.
>
> **Receipts, not rhetoric.** Every kaijutsu release links to the actual `jutsu swarm pr-review` run that reviewed itself. The v0.14.0 release page links to its own pre-merge review run. (Self-review is necessary, not sufficient — but it's a falsifiable receipt, not a marketing badge.)
>
> ### "Isn't this just a wrapper around OpenAI/Anthropic APIs?"
>
> The honest answer: it has wrapper parts and non-wrapper parts.
>
> **Wrapper parts:** the per-vendor driver shims (cli/http/cli-compat/mcp). These are intentionally thin — vendor APIs change, and we want the change surface to be small.
>
> **Non-wrapper parts:**
> - **Cross-vendor portability layer** — same swarm.yaml runs across vendors with different wire formats and auth.
> - **`findings.db` quality fingerprinting** — per-(provider, persona, preset, codebase) precision math derived from your accept/dismiss history. The synthesizer weights each agent's vote by observed precision. No vendor offers this.
> - **Preset SDK** — compose your own swarm shapes; project + home overlay; built-ins shadowed at registration.
> - **Dream lens-blindspot warnings** — `jutsu swarm dream --mode full --lenses all` runs the 8-lens matrix and flags where lens consensus may be RLHF-convergence rather than actual signal.
> - **`agents.yaml` driver abstraction** — provider catalog with overlay + override semantics; vendor switch is a config edit.

### Acceptance check

- ✅ Cynical-read defense paragraph present, factual tone.
- ✅ Lists specific non-wrapper parts (per ADR #12).
- ✅ Receipts framing (per ADR #6).

## Section 7: Skill catalog (preserved, demoted)

### Copy

> ## Skill catalog
>
> Browseable list of every skill kaijutsu ships, plus vendored third-party catalogs. Filter by source, search by name or tag.

### Visual

The current catalog table (search box + filter pills + grid). Pulls from `docs/skills.json` (existing wiring; unchanged). Visual treatment slightly demoted: smaller heading, less padding, no longer the page's centerpiece.

### Acceptance check

- ✅ Catalog renders from `docs/skills.json`.
- ✅ CI sitegen drift check still passes (skills.json wiring intact).

## Section 8: Contributing

### Copy

> ## Contributing
>
> Skills + tools live at [github.com/momentmaker/kaijutsu](https://github.com/momentmaker/kaijutsu). Read [CONTRIBUTING.md](https://github.com/momentmaker/kaijutsu/blob/main/CONTRIBUTING.md) for skill authoring + PR conventions.
>
> Want to add a skill? `jutsu skill new <name>` scaffolds a flat-layout skill directory. `jutsu lint <path>` validates schema + license + permission manifest. Run `jutsu publish` to open a PR against `skills/community/`.

## Section 9: Footer

### Copy (mostly preserved)

> kaijutsu is MIT-licensed. The mascot eats unrecoverable shell commands.
>
> Generated from `skills/core/` + `registry/index.json`. [JSON catalog](/skills.json).

## Visual / brand spec

### Color palette

| Token | Hex | Usage |
|---|---|---|
| `--ink` | `#1A1A1A` | Body text, primary buttons, hero headline |
| `--paper` | `#F5EDE0` | Page background, soft section backgrounds |
| `--bg` | `#FFFFFF` | Card backgrounds, default page background |
| `--mint` | `#3DDC97` | **Accent only**: button hover, active links, code highlights inside ink-bg install box |
| `--mint-dark` | `#2BB37C` | Link color (regular state) |
| `--rust` | `#C18450` | Reserved for community/third-party badges only |
| `--muted` | `#6B6B6B` | Sub-copy, captions, tertiary text |
| `--border` | `#E5E5E5` | Card borders, separators |

Mint must NEVER be a section background; only accents. Charcoal/paper/ink dominates.

### Typography

- Body: `ui-sans-serif, system-ui, -apple-system, "Helvetica Neue", sans-serif` (existing).
- Code: `ui-monospace, SFMono-Regular, "SF Mono", Menlo, monospace` (existing).
- Hero headline: 32–56px clamp (existing pattern).
- Section h2: 28–32px.
- Body: 15–16px, line-height 1.55.

### Mascot + ink-brush

- Mascot SVG/PNG at 120px square in hero. Existing `web-app-manifest-192x192.png` resized via CSS, OR ship a dedicated `docs/mascot-120.png` if the resize artifact looks bad.
- Ink-brush stroke: hand-drawn-style asymmetric SVG path, charcoal stroke, ≤ 4KB. Decorates one side of the hero headline.
- Kanji 術: rendered as a small accent (24–32px) near the mascot OR woven into the ink-brush element.

### Layout

- Max content width: 1080px (existing).
- Section padding: 48px top/bottom on desktop, 32px on mobile.
- Mobile breakpoint: 768px (existing).
- iPhone SE (375px) tested manually before merge.

### Sticky-on-desktop nav

- Desktop ≥ 1024px: thin top nav bar with anchor links to sections 2–8. Sticky position.
- Mobile / tablet: anchor list at top of page (linear, not sticky). Saves vertical real estate.

### Section anchor IDs

Pin the slug → DOM id map so implementation, sticky nav, and any inbound link from README / blog all use the same identifiers:

| Section | Anchor ID |
|---|---|
| Hero (top) | (none — clicking logo returns to top via `#top` on `<body>`) |
| Single-agent failure-mode demo | `id="failure-demo"` |
| Quickstart | `id="quickstart"` |
| Cross-vendor portability | `id="portability"` |
| Cost per bug found | `id="cost"` |
| Trust model | `id="trust"` |
| Skill catalog | `id="catalog"` |
| Contributing | `id="contributing"` |

## Sitegen extension

### Data-driven content separation

The doc-review surfaced a real architectural concern: hero copy hardcoded as Go string constants in `cli/cmd/sitegen/main.go` couples textual content to binary releases (a copy edit becomes a CLI release). Pattern from elsewhere in the project: `skills.json` decouples catalog content from generator code. Apply the same pattern here.

**Content lives in a new `docs/landing-content.json`** (mirroring the `skills.json` pattern). Schema:

```json
{
  "version": 1,
  "hero": {
    "headline": "Cross-vendor agentic CI for codebases.",
    "sub_copy": "Compose review pipelines once. Run them across Claude, Codex, Gemini, and DeepSeek from one config.",
    "tagline": "One CLI. One swarm.yaml. Multi-model adversarial reviews that disagree on purpose.",
    "install_oneliner": "curl -fsSL https://kaijutsu.dev/install.sh | sh"
  },
  "failure_demo": {
    "left_caption": "Single-agent (Claude only)",
    "right_caption": "kaijutsu swarm (claude + gemini + deepseek)",
    "artifact_link": "(filled in pre-merge with the v0.14.0 self-review URL)"
  },
  "cost_example": {
    "models": "deepseek + gemini",
    "cost_usd": "0.05",
    "wall_time": "4m31s",
    "findings_emitted": 10,
    "issues_fixed": 6,
    "cost_per_fix": "~$0.008",
    "source": "v0.14.0 stage-2 doc-review on this spec"
  }
}
```

`docs/landing-content.json` is hand-edited (unlike `docs/skills.json` which is sitegen-regenerated). Editing copy = no Go rebuild. Hands the architecture-purist concern (#2 + #3 from doc-review) the right shape: data in JSON, layout in template, generator just glues them.

### Template structure

Use Go's `html/template` package, NOT raw `io.Writer` calls. The existing `docs/index.html` template gets split into named sub-templates (still one file, just `{{define}}` blocks):

```
{{define "head"}}     ...  (existing head, unchanged)            {{end}}
{{define "hero"}}     ...  (new; consumes .Hero data)            {{end}}
{{define "failure"}}  ...  (new; consumes .FailureDemo data)     {{end}}
{{define "quickstart"}}...                                       {{end}}
{{define "portability"}}...                                      {{end}}
{{define "cost"}}     ...  (new; consumes .CostExample data)     {{end}}
{{define "trust"}}    ...                                        {{end}}
{{define "catalog"}}  ...  (existing catalog table; unchanged)   {{end}}
{{define "contributing"}}...                                     {{end}}
{{define "footer"}}   ...  (existing footer, near-unchanged)     {{end}}

{{define "page"}}
  <!DOCTYPE html><html>{{template "head" .}}<body>
  {{template "hero" .}}
  {{template "failure" .}}
  ...
  {{template "footer" .}}
  </body></html>
{{end}}
```

Generator: load `docs/landing-content.json` into a struct, merge with the `[]Skill` catalog data, execute `page` template. No raw HTML strings escape from `main.go`.

### Hook contract (revised)

`cli/cmd/sitegen/main.go` adds:

- A `LandingContent` struct mirroring the JSON schema above.
- A `loadLandingContent(path string) (LandingContent, error)` reader.
- Existing `renderCatalog(...)` unchanged; the catalog template block reads `.Skills` exactly as today.
- Top-level `RenderSite()` composes both: load skills + load landing content + execute the unified template.

Errors from `loadLandingContent` halt the build (CI fails fast); a missing JSON would otherwise silently render an empty hero.

### Sub-template scope

A copy edit ≤ ~10 lines (e.g., headline rewording) touches `docs/landing-content.json` only — no Go change, no regenerated `docs/skills.json`, no template diff. CI sitegen drift check still gates skills.json wiring.

A layout/structure edit (e.g., adding a section) touches the template file + the JSON schema if new content fields are needed; Go struct gets the new field. Three coordinated changes is the price of a new section — same as adding a new column to the catalog today.

## Doc-review triage section

### Doc-review pass — 2026-05-09

Personas: `claim-auditor-deepseek` + `architecture-purist-gemini`. Run command:

```sh
jutsu swarm doc-review docs/specs/2026-05-09-landing-page-content.md \
  --personas claim-auditor-deepseek,architecture-purist-gemini
```

Run-id: `20260509T155131Z`. Total: 10 findings (4 issue, 4 minor, 2 info). Cost: $0.05. Wall time: 4m31s.

| # | Severity | Source | Summary | Disposition | Rationale |
|---|---|---|---|---|---|
| 1 | issue | architecture-purist-gemini | "swarm.yaml runs on CLIs" misstates orchestration relationship | **Fixed** | Reworded §4: jutsu is the orchestrator; vendor CLIs are dispatch targets. |
| 2 | issue | architecture-purist-gemini | Hardcoded hero copy in Go binary breaks data-driven pattern | **Fixed** | Rewritten §"Sitegen extension": copy lives in `docs/landing-content.json`, mirroring the `skills.json` pattern. |
| 3 | issue | architecture-purist-gemini | `renderHeroBlock` raw `io.Writer` bypasses template engine | **Fixed** | Rewritten §"Sitegen extension": use `html/template` named sub-templates; no raw HTML escapes `main.go`. |
| 4 | issue | claim-auditor-deepseek | Cost numbers from unfinished v0.14.0 run unverifiable | **Fixed** | §5 Cost: switched to real v0.14.0 stage-2 doc-review numbers ($0.05, 4m31s, 10 findings, 6 fixed). Numbers updated post-merge with the live release pr-review run. |
| 5 | minor | claim-auditor-deepseek | "agentic CI" undefined term | **Skipped** | "agentic CI" is an emerging industry term; install line + sub-copy + section bodies clarify what the tool actually does. Adding a definition under the hero would bloat above-the-fold pitch beyond the 30-word constraint. |
| 6 | minor | claim-auditor-deepseek | SVG 4KB constraint missing from acceptance checklist | **Fixed** | Added to §1 Hero acceptance check. |
| 7 | minor | claim-auditor-deepseek | Subjective acceptance criteria ("voice: terse") not testable | **Skipped** | ADR explicitly addresses this in its "Subjective acceptance criteria" section: banned-words list is the mechanical surrogate; subjective items are intentionally human-judgment-shaped per the ADR-acceptance-section. Already handled at the ADR layer. |
| 8 | minor | claim-auditor-deepseek | Section anchor IDs unspecified | **Fixed** | Added §"Section anchor IDs" mapping section → DOM id. |
| 9 | info | claim-auditor-deepseek | ADR's 14-point direction not enumerated in this spec | **Skipped** | ADR is the source of truth; duplicating its 14 points here creates drift risk (ADR change → spec stale). Spec cross-references ADR via the preamble link. Same pattern as every other spec → ADR pair in the project. |
| 10 | info | claim-auditor-deepseek | Self-review artifact URL is TBD | **Skipped** | Explicitly listed in §"Open questions" as a Stage 3c-resolved item. The TBD is the spec being honest about a known unresolved decision; promoting it to a blocker would over-rotate. |

**Apply rule (per IMPL plan)**: all 4 issues addressed. 2 of 4 minor findings addressed (cheap wins). 2 minor + 2 info skipped with one-line rationale each.

## Open questions (deferred to Stage 3c)

- Where does the v0.14.0 self-review run get hosted as a public artifact? Options: GitHub commit comment, a gist, a `docs/receipts/v0.14.0-pr-review.md` page. Lean: gist (simple, public, no extra repo wiring). Decide before merge.
- Dark mode via `prefers-color-scheme`? Defer if implementation balloons CSS by >50 lines.
- Open-graph image (`docs/og-image.png`, 1200×630): static export of the hero, OR a separate composition (mascot + headline + receipts hint). Decide during 3c implementation; if neither converges fast, ship without OG image and add in v0.14.x.

## Status

Stage 3a (ADR): ✅ landed.
Stage 3b (this spec): ✅ landed (v1 draft + doc-review triage applied).
Stage 3b' (doc-review): ✅ landed — see triage section above.
Stage 3c (implementation): in progress — implementer reads this spec + the ADR; gates Stage 3c against the ADR's checklist.
