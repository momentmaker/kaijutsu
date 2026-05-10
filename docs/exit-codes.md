# jutsu exit codes

Public contract from v0.15.0 onwards. Future minor versions may **add** codes; they will **never renumber or repurpose** an existing code.

| Code | Meaning | Examples |
|---|---|---|
| `0` | Success | normal completion |
| `1` | Generic error | I/O failure, network timeout, internal panic recovery, errors not yet mapped to a typed code (long tail) |
| `2` | Usage error | bad flag value (`--by bogus`), missing required arg, mutually-exclusive flag combo (`--json --yaml`), malformed `--since` duration, unknown `--format` |
| `3` | Not found | preset / persona / skill / agent name not in the registry. `jutsu install no-such-skill`, `jutsu swarm pr-review --personas ghost`, etc. |
| `4` | Auth / credential failure | required `<PROVIDER>_API_KEY` env var unset; provider rejected creds at agent-doctor / preflight |

## What's wired

v0.15.1 closes the gap from v0.15.0's typed-subset-only surface. Codes 2, 3, 4 now carry their documented meaning across the CLI; the only paths that stay at exit 1 are the genuine "long tail" (file-not-found errors not directly wrapped, network failures, internal panics, third-party tool failures).

**Code 2 — usage**:
- All cobra-emitted parser errors (`unknown flag`, `required flag`, `flag needs an argument`, `invalid argument`, `unknown command`, `MarkFlagsMutuallyExclusive`) — caught globally via pattern match in `cli.ExitCode`.
- Validation errors across: `jutsu agent add|migrate|test`, `jutsu autopilot init|run`, `jutsu eval skill|persona|preset|swarm-skill`, `jutsu finding accept|dismiss|clear|seed|stats|sync-pr|export`, `jutsu init`, `jutsu install`, `jutsu suggest`, `jutsu swarm bug-repro|code-archaeology|doc-review|brainstorm|dream|refactor-plan|reverse|security-audit|test-gap`.

**Code 3 — not-found**:
- `jutsu agent test <name>` — provider not in vendored catalog
- `jutsu eval persona` — unknown driver / persona registry miss
- `jutsu finding accept|dismiss <id>` — finding-id-not-found
- `jutsu finding *` — no findings store at expected path (run swarm first)
- `jutsu install` — no `kaijutsu.json` in cwd, no lockfile to sync, skill not in registry
- `jutsu lint` — no `skills/core` or `skills/community` in cwd
- `jutsu swarm code-archaeology|reverse` — `--code`/`--spec` path doesn't exist
- `jutsu swarm validate <path>` — file-not-found

**Code 4 — auth / credential failure**:
- `jutsu agent test` HTTP driver: `<PROVIDER>_API_KEY` not set in environment

**Stays at exit 1** (long tail):
- File system / I/O failures not directly wrapped
- Network failures (connect refused, timeout, DNS)
- Provider-side errors after the auth preflight (5xx, rate-limit at runtime)
- Internal panic recovery
- "Multi-model consent not granted" — see note in "What stays at code 1 in v0.15" below
- Anything that hasn't surfaced concrete user friction yet — caller can wrap incrementally

## Why typed codes

Agents that wrap `jutsu` shouldn't have to parse stderr to decide whether to **retry**, **ask the user for credentials**, or **abort**. Branching on `$?` is faster, more reliable, and language-agnostic.

## Why a small set

Earlier drafts of v0.15 spec'd nine codes (`5`/`6`/`7`/`8`/`9` for api/network, conflict, rate-limit, cost-cap, consent). Two parallel `jutsu swarm dream` passes flagged the full set as contract-lock-in risk: once shipped, codes are forever, and several of them mapped to friction we haven't actually hit. The minimal subset (4 codes) defangs the lock-in risk by deferring the contested codes until concrete user friction surfaces.

If you find yourself wishing `jutsu` distinguished a specific failure mode (rate-limit retry-after vs auth, say), open an issue with the call site + the desired branch behavior.

## Example agent retry loop

```bash
#!/usr/bin/env bash
# Wrap jutsu in a retry-with-context loop. Cap the catch-all retries
# so persistent code-1 failures (e.g. multi-model consent not granted,
# disk full, internal panic) don't loop forever.

attempts=0
max_attempts=5
while :; do
  jutsu swarm pr-review --diff-from-branch main --post-comment
  case $? in
    0) echo "review posted"; break ;;
    2) echo "caller misuse — fix args + re-run"; exit 1 ;;
    3) echo "preset/persona/skill not found — install or rename"; exit 1 ;;
    4) echo "missing API key — please export the relevant *_API_KEY"; exit 1 ;;
    *) attempts=$((attempts + 1))
       if [ "$attempts" -ge "$max_attempts" ]; then
         echo "giving up after $max_attempts attempts; check stderr for the underlying error"
         exit 1
       fi
       echo "transient failure (attempt $attempts/$max_attempts) — retry in 30s"
       sleep 30 ;;
  esac
done
```

## What stays at code 1

After the v0.15.1 sweep + global cobra-pattern catch:

- Long-tail call sites not yet wired with `cli.UsageError` / `NotFoundError` / `AuthError` — kept at exit 1 by design until concrete user friction earns the conversion.
- File / I/O failures not directly wrapped; including OS-level `EINVAL` (which would otherwise collide with cobra's "invalid argument" pattern — guarded explicitly).
- Network failures (connect refused, timeout, DNS) — runtime, not caller misuse.
- Provider-side errors AFTER auth preflight (5xx, runtime rate-limit) — runtime, not auth.
- Internal panic recovery.
- "Multi-model consent not granted" (run `jutsu swarm dream --grant-consent` first) — caller-side, but we keep code 4 tight to provider-credential failures only. May promote to code 2 in a future minor.

## What's NOT shipping

- Structured error JSON on stderr (`{code, message, suggestion, retry-after, doc-url}`) — wild-lens 10x candidate. Real candidate for v0.16+ once typed exit codes earn their keep.
- Codes 5/6/7/8/9 — see "Why a small set" above.
