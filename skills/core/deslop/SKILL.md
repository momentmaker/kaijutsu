---
name: deslop
description: Strip AI-generated tells from a draft without changing its substance. Use when the user says "deslop", "remove AI tells", "make this sound less like ChatGPT", or invokes /deslop. Composed by polish, journal, readme-update, and any skill that produces user-facing prose.
---

# deslop

Models leak stylistic tells: emdash overuse, formulaic contrasts ("It's not X, it's Y"), pseudo-profound openers ("At its core..."), bullet-list explosions, and hollow connector words. Deslop removes those without changing what the text says.

## When to invoke

- Another skill produces prose for human consumption (commit messages, READMEs, journal entries)
- The user explicitly asks: "deslop this", "remove AI tells", "make it sound less like ChatGPT"
- A draft is going public (PR description, blog, docs)

## What to remove

### 1. Emdash overuse

Models use emdashes (—) as a default structural break. Replace with periods, parentheses, or semicolons depending on weight.

| Original | Deslopped |
|---|---|
| "It's a great tool — fast, lightweight, and cross-platform." | "It's a great tool. Fast, lightweight, cross-platform." |
| "We considered Postgres — but ultimately chose SQLite." | "We considered Postgres but ultimately chose SQLite." |
| "The key insight — that caching invalidates predictably — drove the design." | "The key insight (caching invalidates predictably) drove the design." |

Don't remove every emdash. Keep the rare one where it carries real weight.

### 2. Formulaic contrasts

The "It's not X, it's Y" pattern. Or "Not just X but also Y." Replace with the direct positive claim.

| Original | Deslopped |
|---|---|
| "It's not just a CLI; it's a workflow." | "It's a workflow CLI." |
| "Not merely fast — it's instant." | "It's instant." |

### 3. Pseudo-profound openers

"At its core...", "Fundamentally...", "In essence...", "Simply put...", "The thing about X is...". Cut and start with the actual claim.

| Original | Deslopped |
|---|---|
| "At its core, kaijutsu is a registry." | "kaijutsu is a registry." |
| "Fundamentally, this changes how we think about agents." | "This changes how we think about agents." |

### 4. Hollow connectors

"It's worth noting that...", "It should be mentioned that...", "It's important to recognize...". If the next sentence is worth saying, just say it.

### 5. Bullet-list explosion

If a paragraph is being converted to a bullet list with 6+ items where 2 sentences would do, revert. Lists should reduce cognitive load, not perform structure.

### 6. Trailing summaries that restate the obvious

"In summary, kaijutsu provides...", "Overall, this approach...". Drop unless the doc is over 1000 words and a real summary would help.

### 7. The phrase "kaijutsu's whole pitch", "the whole game", "the magic is", and other gestures-at-importance

Just say what's important.

## What to keep

- **The substance.** Deslop changes voice, not claims. If the original says "we use SQLite," the deslopped version still says "we use SQLite."
- **Domain-specific terms.** If the draft uses "tarball," keep it. Don't sanitize jargon.
- **The author's voice.** If the original has a quirk that's intentional (a recurring metaphor, a specific tone), preserve it.
- **Markdown structure** when load-bearing — code blocks, headings, links.

## Modes

### `--strip` (default)
Edit the draft in place. Apply every category of fix. Save and report the edits.

### `--detect`
Don't edit. Scan the draft and report flagged passages with category + suggested replacement. Let the user accept/reject per item. Useful for high-stakes prose where you want human approval on each change (PR descriptions, blog posts going public, official docs).

### Project-aware mode (always on)
Before applying any edits, read the project's `CLAUDE.md` / `AGENTS.md` / contributor docs for project-specific style guidance. Honor:
- Project-specific term preferences ("tarball" not "archive"; "skill" not "plugin")
- Voice / tone constraints stated in the doc
- Forbidden phrases the maintainer has called out
- Any explicit "preserve emdash usage in X context" overrides

If the project has a `.deslop.yaml` (or similar) configuration file at the repo root, that takes precedence over generic rules.

## Workflow

1. Read the draft.
2. List the slop categories you spot (emdash, formulaic contrast, opener, connector, bullet explosion, trailing summary).
3. Apply edits per category in one pass.
4. Re-read. The text should sound like a person who knows the subject and respects the reader.
5. If you stripped something the user might want back, mark it: "I removed X — restore if intentional."

## Hard rules

- **Don't add new content while deslopping.** No new examples, no new caveats. Pure subtraction (or word-level substitution).
- **Don't reduce technical precision.** "We pin to a SHA" is precise. "We anchor to a fixed point in history" is slop. Preserve the former.
- **Don't strip emdashes that are doing real work.** Where an emdash carries the weight of a thought-pivot, leave it.
- **Don't deslop other people's published writing.** This skill operates on drafts. Edited published material has its own choices.

## Provenance

Pattern catalog from Jeffrey Emanuel's "Agentic Coding Flywheel" de-slopification guidance. kaijutsu adopts it as a primitive so the catalog lives in one place.
