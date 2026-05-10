# jutsu exit codes

Public contract from v0.15.0 onwards. Future minor versions may **add** codes; they will **never renumber or repurpose** an existing code.

| Code | Meaning | Examples |
|---|---|---|
| `0` | Success | normal completion |
| `1` | Generic error | I/O failure, network timeout, internal panic recovery, errors not yet mapped to a typed code (long tail) |
| `2` | Usage error | bad flag value (`--by bogus`), missing required arg, mutually-exclusive flag combo (`--json --yaml`), malformed `--since` duration, unknown `--format` |
| `3` | Not found | preset / persona / skill / agent name not in the registry. `jutsu install no-such-skill`, `jutsu swarm pr-review --personas ghost`, etc. |
| `4` | Auth / credential failure | required `<PROVIDER>_API_KEY` env var unset; provider rejected creds at agent-doctor / preflight |

## Why typed codes

Agents that wrap `jutsu` shouldn't have to parse stderr to decide whether to **retry**, **ask the user for credentials**, or **abort**. Branching on `$?` is faster, more reliable, and language-agnostic.

## Why a small set

Earlier drafts of v0.15 spec'd nine codes (`5`/`6`/`7`/`8`/`9` for api/network, conflict, rate-limit, cost-cap, consent). Two parallel `jutsu swarm dream` passes flagged the full set as contract-lock-in risk: once shipped, codes are forever, and several of them mapped to friction we haven't actually hit. The minimal subset (4 codes) defangs the lock-in risk by deferring the contested codes until concrete user friction surfaces.

If you find yourself wishing `jutsu` distinguished a specific failure mode (rate-limit retry-after vs auth, say), open an issue with the call site + the desired branch behavior.

## Example agent retry loop

```bash
#!/usr/bin/env bash
# Wrap jutsu in a retry-with-context loop.

while :; do
  jutsu swarm pr-review --diff-from-branch main --post-comment
  case $? in
    0) echo "review posted"; break ;;
    2) echo "caller misuse — fix args + re-run"; exit 1 ;;
    3) echo "preset/persona/skill not found — install or rename"; exit 1 ;;
    4) echo "missing API key — please export the relevant *_API_KEY"; exit 1 ;;
    *) echo "transient failure — retry in 30s"; sleep 30 ;;
  esac
done
```

## What stays at code 1 in v0.15

- Anything that hasn't been explicitly converted via `cli.UsageError` / `NotFoundError` / `AuthError` wrappers — the long tail.
- Cobra-internal mutually-exclusive-flag enforcement (the `MarkFlagsMutuallyExclusive` machinery emits its error before our cli layer sees it; we may convert this in v0.15.x).
- "Multi-model consent not granted" (run `jutsu swarm dream --grant-consent` first) — caller-side issue but we keep code 4 tight to provider-credential failures only. May promote to code 2 in a future minor.

## What's NOT shipping

- Structured error JSON on stderr (`{code, message, suggestion, retry-after, doc-url}`) — wild-lens 10x candidate. Real candidate for v0.16+ once typed exit codes earn their keep.
- Codes 5/6/7/8/9 — see "Why a small set" above.
