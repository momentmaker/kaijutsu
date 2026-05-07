# dream

Pre-implementation interrogation primitive. Walks any topic through 8 cognitive lenses to surface hidden assumptions, weak premises, orthogonal angles, temporal decay, and adversarial misuse BEFORE the topic becomes a spec / plan / commit.

## Install

```bash
jutsu install dream
```

## Use

Standalone in a Claude session:

```
/dream should we ship a public eval harness in v0.8?
/dream --lenses all should we deprecate the cli-compat driver?
```

Multi-agent matrix mode (Stage 2 — `jutsu swarm dream` preset, lands in v0.8.0):

```bash
jutsu swarm dream "should we add a Tauri desktop wrapper to jutsu?"
jutsu swarm dream --lenses all "should we add a Tauri desktop wrapper to jutsu?"
jutsu swarm dream --lenses honest,gaps,inverse "topic"
```

## The 8 lenses

**Base (always run):** honest · fit · gaps · wild

**Extras (`--lenses all` or comma-list selection):** adversary · inverse · status-quo · time

Coverage: 4 critical, 2 generative, 1 contextual, 1 temporal.

## What dream is NOT

- NOT `brainstorm`. brainstorm gives you 5 OPTIONS to solve a problem; dream interrogates whether the problem is the right shape in the first place.
- NOT `polish` / `convergence-detect`. Those are POST-implementation. dream is PRE.
- NOT a checklist. Anti-pattern: invoking dream on every micro-decision produces template-shaped filler. Use judgment — see SKILL.md "Anti-pattern" section.

## Privacy

No network calls. Composes existing swarm primitives (which carry their own v0.7 privacy boundaries); dream itself adds no new network surface. Dream graveyard at `~/.kaijutsu/dreams/` — directory mode `0700`, files `0600`. Same boundary as findings.db.

## Future (v0.8.x)

- Per-lens precision tracking (dedicated `lens TEXT` schema column)
- Adaptive lens selection by topic embedding
- Lens-emphasis weighting in synthesizer
- `dream-as-eval-corpus` (did wild's predictions come true 6 months later?)

See `SKILL.md` for the full process + anti-sycophancy gates + load-bearing definition + lens prompts.

Spec: [`docs/specs/2026-05-07-v0.8.0-dream-skill-and-preset.md`](https://github.com/momentmaker/kaijutsu/blob/main/docs/specs/2026-05-07-v0.8.0-dream-skill-and-preset.md)
