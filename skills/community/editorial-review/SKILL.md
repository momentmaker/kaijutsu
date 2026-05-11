---
name: editorial-review
description: Multi-pass editorial review for long-form essays + articles. Run after drafting is complete, before publish. Four passes — structural / line-level / AI-tells & voice / resonance. Voice-configurable (default literary-intelligent-general). Composes deslop for final user-facing prose pass. Use when the user says "review this essay", "polish this article", "edit this draft", "check this for AI-tells", or invokes /editorial-review.
---

# Editorial Review

Multi-pass editorial review for long-form essays and articles. Runs after drafting is complete, before publish.

## Input

The user provides a file path to the essay. If not provided, look for the most recently modified `.md` file in the working directory or in `content/`, `essays/`, `posts/`, or `drafts/`.

The user can also pass a **voice config** to tune thresholds — see [Voice configs](#voice-configs) below. Default: `literary-intelligent-general`.

## Step 0: Load Context

1. Read the essay file completely
2. Read any `CLAUDE.md` / `AGENTS.md` for project-specific editorial guidelines (if present)
3. Note the word count
4. Note the active voice config (default `literary-intelligent-general` unless user specified otherwise)

## Step 1: Structural Pass

Review the essay's architecture:

- **Arc**: Does the essay move through distinct registers? Is there a turn or complication?
- **Transitions**: Read the last paragraph of each section and the first paragraph of the next. Does the reader feel carried forward or dropped and reset?
- **Pacing**: Are there stretches of 2,000+ words at the same analytical register without relief? Flag them.
- **Sections**: Does each section earn its length? Could any be cut or merged without loss?
- **Opening**: Does it drop the reader into something specific and sensory within the first paragraph?
- **Closing**: Does it open rather than close? Does it echo or transform the opening?

Report findings as:

```
## Structural Pass
- [STRONG] description
- [FIX] description — suggested change
```

Fix all `[FIX]` items before proceeding.

## Step 2: Line-Level Pass

Read every paragraph checking for:

- **Paragraph density**: Any paragraph over 6 lines on a mobile screen (roughly 4 sentences of medium length)? Split it.
- **Jargon**: Any technical or academic term that isn't earned and explained gracefully? Replace with plain English.
- **Repetition**: Same word, phrase, or construction used in two different sections unintentionally? Vary one instance.
- **Rhythm**: Three or more consecutive paragraphs opening with the same sentence pattern? Vary.
- **Wellness-speak**: Any use of "healing," "journey," "transform," "authentic self" that isn't precise and necessary? Cut or replace.
- **Inspirational generalities**: Any sentence that could appear on a bumper sticker without the surrounding context? Earn it or cut it.
- **Telling vs. showing**: Any sentence that summarizes an experience instead of rendering it? ("It was profound" → show the specific moment.)
- **Proselytizing**: Any sentence that tells the reader what to conclude? Rewrite to walk them to the edge and let them look.
- **Dead phrases**: "One might argue," "it could be said," "in a sense" — cut.

Report findings with line numbers:

```
## Line-Level Pass
- [FIX] line:XX — description
- [MINOR] line:XX — description
```

Fix all `[FIX]` items. Fix `[MINOR]` items unless doing so would harm rhythm.

## Step 2.5: AI-Tells & Voice Pass

Whole-document pattern audit. These are tells that AI-generated prose leaks into long-form work. The fix is restraint, not elimination — measure first, then judge.

### Pattern counts

Run these against the full file (use `grep` + `wc` against the saved file). Thresholds default to the `literary-intelligent-general` voice; see [Voice configs](#voice-configs) to adjust.

- **Em dashes** — count `—` characters per 1,000 words.
  - Threshold: **≤ 5 per 1,000 words** for literary-contemplative register.
  - Above that, propose colon, period, or comma replacement for the lowest-impact instances. Em dashes earn their place when interrupting a live thought. They do not earn it as default sentence connectors.
- **Negation rhetoric** — count instances of *"is not X. It is Y"* / *"Not X. Y"* / *"is not X — it is Y"* / *"It is not X; it is Y."*
  - Threshold: **≤ 3 per essay**.
  - The pattern works once or twice as inversion of expectation. Beyond that it becomes a tic. Flag the weakest instances and rewrite as positive assertion.
- **Triplet fragments** — three consecutive short sentences (e.g., *"The same energy. The same substance. Two opposite shapes."*).
  - Threshold: **≤ 3 per 3,000 words**.
  - Use sparingly for emphasis; overuse signals a generative tic.
- **Adverb / hedge cluster** — *really, very, quite, actually, perhaps, almost, in some way, somehow, simply, just, basically.*
  - Flag any paragraph containing **2+** of these. Cut or sharpen.
- **Cliché bigrams** — phrases that commonly leak from LLM training data into prose:
  - *delve into, tapestry, navigate the complexities, in the realm of, robust, comprehensive, underscore, pivotal, multifaceted, let me know if, feel free to, in conclusion, furthermore, landscape, intricacies, journey, transform*
  - Flag any instance. Replace with concrete language.
- **Sentence-starter repetition** — 3+ consecutive sentences starting with the same word (especially *"The"*, *"It"*, *"This"*, *"There"*).
  - Flag and vary one or more openers.

### Tense consistency

Within a single scene, flag unmarked shifts between past and present.

- Editorial frame in present tense (the author addressing the reader directly — *"I want to ask you a question"*) is fine and expected.
- Narrative scene in past tense is fine.
- Mid-paragraph drift between the two, without a section break or structural cue, is the issue.

Specifically check:

- Scenes recounting a remembered or historical event — should hold one tense unless the slip is marked.
- Author-address paragraphs — present tense throughout.
- Borrowed scenes from sources — pick one tense and hold.

### Contemporary cultural touchstones (optional pass — voice-dependent)

A specific contemporary reference often beats an abstract description. Look for sentences where an abstract phrase could be replaced with a more vivid contemporary touchstone — an app interface, a meme term, a brand name, a current cultural artifact, a recognizable internet behavior. Examples:

- *"the influencer optimizes one trait at the expense of others"* → *"the influencer is **min-maxing** for visibility"*
- *"the algorithmic feed rewards constant performance"* → *"the timeline is a casino floor with no clocks"*
- *"the user keeps refreshing their notifications"* → *"the user is checking the slot machine on their lock screen"*

This pass is voice-dependent — only suggest touchstones if the active voice config allows them. The `literary-intelligent-general` default permits at most 1–2 per essay, italicized on first use, never explained. The `terse-technical` voice rejects them entirely. The `conversational` voice welcomes them more freely.

Hard rule for any voice: if a touchstone needs explaining, it doesn't belong.

### Report

```
## AI-Tells & Voice Pass
- Em dashes: X (Y per 1000 words) — [PASS / OVER]
- Negation pattern: X instances — [PASS / OVER, list line numbers]
- Triplet fragments: X — [PASS / OVER, list line numbers]
- Adverb/hedge clusters: [PASS / list paragraphs]
- Cliché bigrams: [PASS / list line numbers + replacement suggestions]
- Sentence-starter repetition: [PASS / list passages]
- Tense consistency: [PASS / list scenes with unmarked drift]
- Touchstone candidates: [optional list of swap suggestions]
```

Fix `[OVER]` items before proceeding to Resonance Check.

> **Signal not verdict.** These heuristics flag patterns common in AI-generated prose. High scores don't prove AI authorship — they identify passages worth rewriting for voice and variance. Hemingway and McCarthy would both fail several of these checks; that's a feature of the heuristics, not a defect of those writers. Use the signal to inform edits; never use it as a gotcha.

## Step 3: Resonance Check

This is the most important pass. Read the full essay one more time asking the central question at each paragraph:

> Would the right reader feel that this essay has said what they've been carrying but could not yet put into words?

### 3a: Naming the Unnamed

- **The core recognition**: Identify the essay's central unnamed thing — the feeling, condition, or experience it is giving language to. State it in one sentence. If you cannot, the essay may be informing without resonating. Flag this.
- **Naming moments**: List every passage where the essay gives precise language to something the reader likely feels but has never seen articulated. There should be at least 3–5 of these. If there are fewer, identify paragraphs doing intellectual work without emotional work and suggest where the unnamed thing is hiding inside the argument.

### 3b: Lines Worth Stealing

- **Quotable moments**: Are there at least 8–10 lines across the essay that a reader would screenshot, underline, or text to someone? List them. These should not be aphorisms dropped in — they should be moments where the essay's argument crystallizes into a single, precise, inevitable sentence.
- If there are fewer than 8, identify paragraphs carrying the most pressure and suggest where a cleaner release — a shorter sentence after a long build, a sharper image, a more direct statement — would create the moment.

### 3c: The Reader's Self-Recognition

- Are there at least 3 moments where the essay describes a condition the reader will recognize as their own? Not abstract conditions ("we all feel lost") but specific ones rendered with enough bodily detail that the reader thinks "yes, exactly"?
- If not, identify where a hyper-specific image or second-person address would pull the reader from observer to participant.

### 3d: The Shareability Test

- **The friend test**: If the right reader encountered this essay — someone in the middle of the experience the essay is about — would they send it to the one person who might understand? If the answer is not an immediate yes, identify what's missing: the essay may be interesting without being urgent.
- **Counter-arguments**: Does the essay engage its strongest objections? Or does it preach to the converted?
- **The ending test**: Read only the last 500 words. Does the reader feel seen and invited, not instructed? Does the essay point beyond itself?

Report:

```
## Resonance Check
- Core unnamed thing: [one sentence]
- Naming moments: X identified (list them)
- Lines worth stealing: X identified (list them)
- Reader self-recognition: X moments (list them)
- Shareability: would forward / interesting but not urgent / needs work
- Counter-arguments: engaged / not engaged
- Ending: opens / closes
- Overall: ready / needs work
```

Fix any deficiencies found. This pass is not optional polish — it is the difference between a good essay and one that travels.

## Step 4: Final Verification

- Run word count.
- Confirm formatting follows the publication's conventions (the user's CLAUDE.md / AGENTS.md or the project's house style — read both before this step).
- Read the first and last paragraphs back-to-back — do they rhyme?
- Run [`deslop`](https://kaijutsu.dev/skills/deslop) (composed dep) on the full essay as a final user-facing-prose pass. Stops fresh slop from sneaking past the editorial layer.

Report:

```
## Final Verification
- Word count: X
- Formatting matches project convention: pass/fail
- Opening/closing resonance: pass/fail
- Deslop pass: clean / N edits made
- Status: READY / X items remaining
```

## Voice configs

Editorial-review thresholds vary by voice. Pass the voice name via `--voice <name>` or set it in front-matter at the top of the essay file.

### `literary-intelligent-general` (default)

The Pilgrim Age / Paul Graham / Tim Urban / contemplative-but-not-precious register. Smart general reader.

| Pattern | Threshold |
|---|---|
| Em dashes | ≤ 5 per 1,000 words |
| Negation rhetoric | ≤ 3 per essay |
| Triplet fragments | ≤ 3 per 3,000 words |
| Cliché bigrams | hard reject |
| Touchstones | ≤ 1–2 per essay, italicized on first use |
| Sentence variance | medium-high |

### `terse-technical`

Engineering blog posts. Stripe / Vercel / Tailscale / Fly.io. Short sentences, plain words, examples-first.

| Pattern | Threshold |
|---|---|
| Em dashes | ≤ 2 per 1,000 words |
| Negation rhetoric | ≤ 1 per essay |
| Triplet fragments | rejected |
| Cliché bigrams | hard reject |
| Touchstones | rejected |
| Hedges | hard reject |
| Sentence variance | low (consistency over variety) |

### `conversational`

Personal blogs, casual newsletters, voice-heavy writing. Internet-native cadence.

| Pattern | Threshold |
|---|---|
| Em dashes | ≤ 8 per 1,000 words (conversation breaks more) |
| Negation rhetoric | ≤ 5 per essay |
| Triplet fragments | ≤ 5 per 3,000 words |
| Cliché bigrams | soft reject (only the most LLM-shaped) |
| Touchstones | welcomed |
| Hedges | tolerated |
| Sentence variance | high |

### `journalistic`

News / longform reporting. The New Yorker / Atlantic / Atavist. Clear, neutral, scene-driven.

| Pattern | Threshold |
|---|---|
| Em dashes | ≤ 3 per 1,000 words |
| Negation rhetoric | ≤ 2 per essay |
| Triplet fragments | rejected |
| Cliché bigrams | hard reject |
| Touchstones | rejected |
| Hedges | only when reporting uncertainty |
| Sentence variance | medium |

## Hard rules

- **Read the whole essay before flagging anything.** Patterns matter; isolated infractions don't.
- **Step 3 (Resonance) is the most important pass.** Skipping it because the prose is "clean" misses the point of the essay.
- **Heuristics are signal, not verdict.** A flagged passage might be the writer's intent; flag and explain, never strip.
- **Author voice wins over editor preference.** Suggest, don't impose.
- **Compose `deslop` at the end.** Catches slop the editorial layer waved through.

## Composes

- [`deslop`](https://kaijutsu.dev/skills/deslop) — final user-facing-prose pass at Step 4
- `jutsu swarm doc-review --personas claim-auditor-deepseek,architecture-purist-gemini` — optional multi-agent review on the same essay after this skill completes its passes

## When NOT to use

- Code-shaped artifacts (READMEs, SCHEMA docs, technical specs) — use `doc-review` instead
- Short-form (≤ 500 words) — overkill; the resonance pass needs length to work
- Fiction with strong stylistic choices — most "AI tells" are also legitimate prose techniques in fiction; skill thresholds will misfire
- Translation drafts where source-text fidelity matters more than voice
