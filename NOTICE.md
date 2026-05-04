# NOTICE

kaijutsu is licensed under the MIT License (see [`LICENSE`](./LICENSE)). This file lists external works incorporated into or referenced by this repository, with their respective licenses and attribution.

## Bundled adaptations under `skills/core/`

The following skills under `skills/core/` are adapted from upstream MIT-licensed sources. Each adapted skill carries a preamble in its `SKILL.md` linking to the upstream and a `homepage` / `upstream` field in its `skill.yaml`.

| kaijutsu skill | Upstream | License | Author |
|---|---|---|---|
| `spec-driven-development` | [`addyosmani/agent-skills`](https://github.com/addyosmani/agent-skills/blob/main/skills/spec-driven-development) | MIT | Addy Osmani |
| `planning-and-task-breakdown` | [`addyosmani/agent-skills`](https://github.com/addyosmani/agent-skills/blob/main/skills/planning-and-task-breakdown) | MIT | Addy Osmani |
| `security-and-hardening` | [`addyosmani/agent-skills`](https://github.com/addyosmani/agent-skills/blob/main/skills/security-and-hardening) | MIT | Addy Osmani |
| `code-simplification` | [`addyosmani/agent-skills`](https://github.com/addyosmani/agent-skills/blob/main/skills/code-simplification) | MIT | Addy Osmani |
| `incremental-implementation` | [`addyosmani/agent-skills`](https://github.com/addyosmani/agent-skills/blob/main/skills/incremental-implementation) | MIT | Addy Osmani |

Modifications: each port is wrapped in a kaijutsu `skill.yaml` (license, agents, permissions, deps, hooks), composed with kaijutsu primitives (`polish`, `blunder-hunt`, `convergence-detect`, `scope-check`, `decide`) where appropriate, and adapted to the kaijutsu schema (single `SKILL.md` per skill, agent-agnostic, install via `jutsu`).

## Referenced — not bundled

The full [`addyosmani/agent-skills`](https://github.com/addyosmani/agent-skills) collection (20 skills covering Define / Plan / Build / Verify / Review / Ship / Simplify) is also accessible via `registry/index.json` entries. Users can install any skill in the upstream repo with `jutsu install addyosmani/<skill-name>` once the v0.3 vanilla-SKILL.md compat mode lands. The originals remain canonical; kaijutsu's index just provides the multi-agent install path.

## Patterns and ideas (no code carried)

- **Anti-rationalization tables** — pattern from `addyosmani/agent-skills` adopted in several kaijutsu core skills.
- **Process-over-prose framing** — same source.
- **Verification exit criteria** — same source.
- **Multi-pass critique with deliberately different lenses** — derived from Jeffrey Emanuel's [Agentic Coding Flywheel](https://agent-flywheel.com/complete-guide). See `blunder-hunt` and `polish`.
- **Lie-to-them prompt pressure** — same source. See `lie-to-them` skill.
- **Convergence detection (output-shrink + change-rate + similarity)** — same source. See `convergence-detect`.
- **Cross-agent skill standard** — [Anthropic Agent Skills](https://agentskills.io); the canonical SKILL.md schema and the `.agents/skills/` directory are also documented there.

## Mascot

The kaijutsu monster mascot is original work commissioned by the maintainer. Generic Japanese vocabulary (`kai` / `jutsu` / `kaiju` / `kaizen`) is in the public domain.

## License compatibility

All upstream sources adapted into kaijutsu are MIT or compatibly permissive (BSD-2/3, ISC, Apache-2.0). New skill submissions to `skills/core/` and `skills/community/` must also be MIT or compatible — see [`CONTRIBUTING.md`](./CONTRIBUTING.md) and [`LICENSE`](./LICENSE).
