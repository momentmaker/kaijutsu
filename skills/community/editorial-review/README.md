# editorial-review

Multi-pass editorial review for long-form essays + articles. Four passes (structural / line-level / AI-tells & voice / resonance). Voice-configurable. Composes [`deslop`](https://kaijutsu.dev/skills/deslop) as the final pass.

## Install

```sh
jutsu install editorial-review
```

## Use

```sh
# Default voice (literary-intelligent-general)
"editorial-review on content/my-essay.md"

# Pick a voice
"editorial-review on content/my-essay.md with voice=terse-technical"

# Layer multi-agent on top (optional)
jutsu swarm doc-review content/my-essay.md \
  --personas claim-auditor-deepseek,architecture-purist-antigravity
```

## Voice configs

- **`literary-intelligent-general`** (default) — Pilgrim Age / Paul Graham / Tim Urban register. Smart general reader.
- **`terse-technical`** — engineering blog posts. Stripe / Vercel / Tailscale shape.
- **`conversational`** — personal blogs, casual newsletters, voice-heavy writing.
- **`journalistic`** — longform reporting. The Atlantic / Atavist shape.

Each voice tunes thresholds for em-dash density, hedge tolerance, cliché rejection, touchstone permission, etc. See [SKILL.md](./SKILL.md#voice-configs) for the full matrix.

## The four passes

1. **Structural** — arc, transitions, pacing, opening, closing
2. **Line-level** — paragraph density, jargon, repetition, wellness-speak, telling-vs-showing
3. **AI-tells & voice** — pattern audit against the active voice config (em-dashes, triplet fragments, hedges, cliché bigrams, sentence-starter repetition, tense drift)
4. **Resonance** — naming the unnamed, lines worth stealing, reader self-recognition, shareability

## Composes

- [`deslop`](https://kaijutsu.dev/skills/deslop) — final user-facing-prose pass (runs at Step 4)
- `jutsu swarm doc-review` — optional multi-agent prose review layered on top

## Signal not verdict

The AI-tells pass is a **signal**, not proof. Hemingway, McCarthy, and Strunk-and-White-disciplined writers all flag against several of these heuristics. Use the signal to inform edits; never use it as a gotcha.

## License

MIT.
