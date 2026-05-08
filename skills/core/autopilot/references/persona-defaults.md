# Persona defaults

Built-in personas (`cli/internal/agents/builtin_personas.go`) ship with kaijutsu. Zero-config autopilot uses these without requiring `agents.yaml`.

## Default per-phase persona sets

| Phase | Personas | Lens |
|---|---|---|
| Brainstorm | `brainstorm-creative-claude`, `architecture-purist-gemini`, `claim-auditor-claude` | Anti-sycophancy + load-bearing claim audit |
| Spec review | `architecture-purist-gemini`, `paranoid-security-claude`, `claim-auditor-claude` | Architecture + security + claims |
| Plan review | `perf-purist-codex`, `claim-auditor-claude`, `cross-file-gemini` | Perf + claims + cross-file consistency |
| Final pr-review | `paranoid-security-claude`, `architecture-purist-gemini`, `perf-purist-codex`, `claim-auditor-claude` | Full lens |

## v0.11.0 built-in personas

Pre-existing (v0.6+):
- `paranoid-security-claude` — security review lens
- `pragmatic-codex` — refactor-cautious lens
- `architecture-purist-gemini` — layer-boundary + dependency lens
- `brainstorm-creative-claude` — orthogonal-angles + counterargument lens

New in v0.11.0:
- `claim-auditor-claude` — load-bearing-claim audit (CLI-backed via claude provider)
- `cross-file-gemini` — cross-file-consistency lens
- `perf-purist-codex` — algorithmic-complexity lens

All built-ins are CLI-backed (claude/codex/gemini providers). No HTTP API key required. Power users can override via `agents.yaml` to add HTTP-backed lenses (deepseek personas, etc.).

## Override

`.kaijutsu/autopilot.yaml::*.personas` lists the personas for each phase. autopilot resolves names against `BuiltinPersonas()` first, then user's `agents.yaml`. Unknown personas error with the full available set named inline.

## Provider availability

Codex/gemini personas degrade gracefully when the underlying CLI isn't installed. autopilot prefers personas whose providers are available; if a phase's persona list resolves to fewer than 2 available, autopilot warns + proceeds with what's available.

If zero personas resolve for a phase → hard error: install at least one of `claude` / `codex` / `gemini`.
