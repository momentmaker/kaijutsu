# ADR: kaijutsu.dev landing page rebuild — locked direction

**Date:** 2026-05-09
**Status:** Accepted, implementing in v0.14.0 Stage 3
**Spec:** [`docs/specs/2026-05-09-v0.14.0-persona-browse-and-preset-usage.md`](../specs/2026-05-09-v0.14.0-persona-browse-and-preset-usage.md) (§ Stage 3)
**Dream archive:** `~/.kaijutsu/dreams/53dee208832bf970-kaijutsu-landing-page-ethos-design-load-bearing-qu-20260509-032204.md` (87 findings, 4 personas, `--mode full --lenses all`)
**Author:** rubberduck

## Context

`docs/index.html` was a sitegen-generated catalog table with a minimal pitch. v0.10–v0.13 shipped in fast succession (eval runner, autopilot v2, Tier A presets, Preset SDK) but the landing surface didn't keep pace. New users hit the page, saw a skill table, and bounced because the WHY-this-tool-exists framing wasn't there.

A `jutsu swarm dream --mode full --lenses all` pass on landing-page direction surfaced 87 findings across 4 personas (brainstorm-creative-claude / architecture-purist-gemini / claim-auditor-deepseek / paranoid-security-claude). Synthesis converged on a small number of locked decisions; this ADR carries them forward as an implementation-time checklist.

## Decision

Ship a hand-authored landing page that extends (does not big-bang replace) the current `docs/index.html`, with a hero-block hook added to `cli/cmd/sitegen/main.go` so the catalog continues to render unchanged underneath.

### Locked direction (14 points)

1. **Above-the-fold framing**: cross-vendor portable swarm.yaml primitive, NOT "multi-agent". (Time-lens 4-agent consensus: multi-agent commodifies by 2028 as vendor IDEs ship native review; cross-vendor portability is the durable moat.)
2. **Page section order**: hero → single-agent failure-mode demo → quickstart → cross-vendor portability → cost-per-bug-found → trust model → skill catalog (preserved, demoted) → contributing → footer. Sticky-on-desktop nav.
3. **Voice**: terse, technical, code-block-heavy, show-don't-tell. Some warmth allowed — discount the all-agents-agreed "no aspirational voice ever" warning as a lens-blindspot (Linear / Raycast / Arc broke this rule profitably).
4. **Cost framing**: "cost per bug found" (e.g. "$0.12 / 14s for an architecture flaw caught"), not "cost per call". Address the 3x-API-cost objection inline at the cost section, NOT as a top-line feature.
5. **Mascot + visual identity**: keep the chibi mascot at smaller hero placement (≤ 120px, down from 160px). Charcoal/paper/ink primary palette; mint-teal #3DDC97 as ACCENT only. Time-lens warning: avoid trend-saturated colors; if the brand needs a refresh later, easier to swap an accent than a hero.
6. **Receipt-driven trust**: every release page links to the actual swarm pr-review run that reviewed it. Embed ONE real disagreement-table screenshot from a v0.13 release pr-review. Receipts > rhetoric.
7. **Drop "pre-alpha" framing** anywhere user-visible. A page that says pre-alpha + ships v0.13 + autopilot + Preset SDK reads as abandoned. Internal contributor docs (CLAUDE.md / AGENTS.md) keep the framing; the landing page does not.
8. **NO "reviewed by N LLMs" badge**: invites mockery + falsifiability attack ("three stochastic parrots agree" tweet writes itself). Replace with a concrete artifact link/embed pointing at a specific PR-review run with a real disagreement-table.
9. **Sitegen regen as hard constraint**: extend `cli/cmd/sitegen/main.go` to add a hero-block template hook BEFORE the catalog table. Catalog continues to render; hero block is the new addition. CI's lint-skills.yml drift check stays valid because skills.json wiring unchanged.
10. **Audience = senior IC engineers** (both skill author + skill user). Team-lead/EM/buyer audience deferred to v0.20+ (mixed audiences = muddy CTA).
11. **Avoid "swarm" as headline marketing word** — vendor definitions of "agentic" may diverge by 2028. Use "multi-model" / "cross-vendor agentic CI" / "kaijutsu" itself for headline framing; "swarm" stays as the existing CLI command name (unchanged).
12. **Cynical-read defense**: explicitly address the "this is just a wrapper around OpenAI/Anthropic APIs" reading. Show what's NOT a wrapper: cross-vendor portability, findings.db quality fingerprinting (per-agent confidence weights from real precision data), cost transparency, Preset SDK, dream lens-blindspot warnings. One section, factual, no defensive tone.
13. **Inverse path explicitly considered + rejected**: "no landing page, just README + asciinema" is coherent (Anthropic MCP launched on docs alone) but rejected — kaijutsu's mascot brand + sumi-e identity are sunk-cost worth using AND the page provides discoverability anchors that README+asciinema can't.
14. **Success metric**: skills-installed-via-`jutsu install`-in-real-codebases (qualitative for v0.14; opt-in telemetry deferred to v0.20+). Tracked via GitHub stars + Homebrew install count + skill-import frequency in opt-in PR comments. NOT tracked: vanity metrics (page views, time on page).

## Implementation checklist (objective acceptance criteria)

These are the checkable bits the dream pass synthesis distilled. Stage 3c implementation is not "done" until each of these passes:

### Copy / content

- [ ] Above-the-fold pitch ≤ 30 words.
- [ ] Above-the-fold pitch contains the words "cross-vendor" or "portable" (durable-moat framing per #1).
- [ ] Above-the-fold pitch passes the "would a senior IC say 'oh, that's different'?" gut-test.
- [ ] Banned words absent from page body: "AI-powered", "transform", "revolutionize", "powerful", "unleash", "magic", "delight", "blazing fast", "supercharge", "elevate", "reimagine".
- [ ] "pre-alpha" string absent from any user-visible body copy. CLAUDE.md / AGENTS.md / contributor-only docs may keep it.
- [ ] No "reviewed by N LLMs" badge. Replaced with a concrete artifact link to a real swarm pr-review run (per #8).
- [ ] Cynical-read defense paragraph present in trust-model or near-it section, factual tone (per #12).
- [ ] Cost-per-bug-found framing present with at least one concrete numeric example (per #4).

### Section order + structure

- [ ] Sections appear in the locked order: hero → failure-mode demo → quickstart → cross-vendor portability → cost-per-bug-found → trust model → skill catalog → contributing → footer.
- [ ] Sticky-on-desktop nav OR top-of-page anchor list links to each section.
- [ ] Skill catalog section consumes `docs/skills.json` (sitegen drift check still passes).
- [ ] Single-agent failure-mode demo embeds OR links to a real artifact (sycophancy-prone single-agent output side-by-side with kaijutsu's disagreement-table; lifted from `findings.db` or recent `pr-review.log`-style capture).
- [ ] Quickstart shows `curl install` line + ONE `jutsu swarm pr-review` invocation + a partial output snippet.

### Visual / brand

- [ ] Mascot present at ≤ 120px hero placement (smaller than current 160px).
- [ ] Charcoal/paper/ink primary palette; #3DDC97 used only on accent surfaces (button hover, active links).
- [ ] Kanji 術 + asymmetric ink-brush element prominent in hero.
- [ ] No "swarm" used as a headline marketing word in body copy (CLI command-name references like `jutsu swarm pr-review` are fine).

### Mobile + a11y + perf

- [ ] Layout intact at 375px viewport (iPhone SE baseline) — manual visual check.
- [ ] Layout intact at 1280px viewport — manual visual check.
- [ ] Lighthouse Performance ≥ 90 (local run before merge).
- [ ] Lighthouse Accessibility ≥ 90 (local run before merge).
- [ ] Hero pitch fits in viewport without scroll on 1280×720 laptop window.

### Sitegen + deploy

- [ ] `cli/cmd/sitegen/main.go` extended with a hero-block template hook BEFORE the catalog.
- [ ] `docs/skills.json` rendering path unchanged; CI catalog-drift check still passes.
- [ ] `kaijutsu.org` 301 → `kaijutsu.dev` (existing redirect; verify still works after deploy).
- [ ] `curl https://kaijutsu.dev | grep "<title>"` returns the new title (catches a deploy regression).
- [ ] `https://kaijutsu.dev` reflects the new page within ~minutes of merge to main (existing GitHub Pages workflow).

## Rejected alternatives

| Alternative | Why rejected |
|---|---|
| Keep current sitegen-only catalog page | New users bounce because the WHY framing isn't there. Catalog without context = directory listing. |
| README + asciinema only (no landing page) | Coherent (Anthropic MCP launched on docs alone) but throws away mascot brand + sumi-e identity that are sunk-cost worth using; loses discoverability anchors that the page provides. |
| Big-bang redesign (rip out current page entirely) | 3-agent fit/status-quo lens consensus: extend, don't replace. Current page has a working catalog wiring + GitHub Pages deploy; risk-adjusted return is higher for additive hero block than for full rewrite. |
| Lead with "multi-agent" as the headline framing | Time-lens 4-agent consensus: multi-agent commodifies by 2028 (vendor IDEs ship native review). Cross-vendor portability is the durable moat — that's the headline. |
| "Reviewed by N LLMs" badge | 2-agent adversary lens consensus 0.85: invites "three stochastic parrots agree" mockery + falsifiability attack. Replace with concrete artifact embed. |
| Mint-teal as primary palette | 3-agent honest lens consensus that current mint-dominant palette risks "cute toy" framing. Demote to accent; primary = charcoal/paper/ink. (Lens-blindspot warning preserved: don't over-correct toward serious-aesthetic-only.) |
| Strip all warmth from voice | Lens-blindspot warning surfaced + discounted: LLMs over-index on "no aspirational voice ever" but Linear / Raycast / Arc broke this rule profitably. Some warmth allowed; banned-words list catches the actual marketing-speak failure mode. |
| Mixed audience (IC + EM/buyer) | Gaps-lens consensus: muddy CTA. v0.14 audience = senior IC; team-lead/buyer deferred to v0.20+. |

## Lens-blindspot warnings preserved as guardrails

These warnings flagged the dream synthesis itself; they're carried into the implementation phase as anti-over-correction guardrails:

- **No over-correction toward "stripped voice"** — counter-evidence exists (Linear, Raycast, Arc). Some warmth is allowed; the banned-words list is the actual constraint.
- **No over-correction toward "serious-aesthetic only"** — chibi mascot stays; kanji 術 stays; ink-brush primitive stays. Just at smaller hero placement.
- **The 2028-multi-agent-becomes-commodity prediction may not hold** — vendor coordination could fail, in which case kaijutsu's multi-agent surface is also durable. Locked direction works either way because cross-vendor portability is durable in BOTH futures.

## Why a separate ADR + content spec + implementation step

The dream pass surfaced 87 findings across 4 personas. Direct-to-implementation would lose the synthesis. Separate ADR (this file, locked direction + checklist) → content spec (concrete copy + visuals derived from this ADR) → doc-review on the content spec → implementation that holds itself accountable to the ADR's checklist.

Doc-review the content spec, not this ADR — this ADR is the synthesis already; doc-review on synthesis would over-iterate. The content spec is where new claims appear (specific copy, specific layout choices) and where doc-review catches drift from the locked direction.

## Status

Stage 3a (this ADR): **landed 2026-05-09 with v0.14.0 Stage 3 implementation.**
Stage 3b (content spec): in progress.
Stage 3c (implementation): pending content spec + doc-review on content spec.
