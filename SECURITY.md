# Security Policy

## Scope

kaijutsu is a registry + CLI for AI-coding-agent skills. The primary security surfaces are:

1. **Skill content** — `SKILL.md` files are loaded into an agent's context window. A malicious skill can prompt-inject the agent to perform actions against the user's interest, including shell execution and filesystem writes via the agent's tool calls.
2. **Skill scripts** — when a skill ships executable scripts (`scripts/`), they may be invoked by the agent in contexts that execute shell commands on the user's machine.
3. **Tarball extraction** — the CLI fetches and extracts archives from GitHub. Path-traversal and zip-bomb guards are in place but warrant ongoing review.

## Threat model

We assume:

- Users trust the kaijutsu maintainers and skills shipped under `skills/core/` of the canonical `momentmaker/kaijutsu` repository.
- Users do NOT inherently trust third-party skills installed via `--registry <other-repo>` or via a future `registry/index.json` entry.
- Sigstore-signed core skills will carry strong publisher provenance once enforcement ships (v0.3 target).

## Reporting a vulnerability

Email **gh@fz.ax** with subject `kaijutsu security`. Include:

- Reproduction steps
- Affected version (`jutsu --version`)
- Severity assessment
- Whether you have discussed the issue with anyone else

We aim to acknowledge within 7 days. Coordinated disclosure preferred — please do not open public issues for unfixed vulnerabilities.

## Current safeguards

- Tarball downloads capped at 100 MiB; extracted size capped at 200 MiB (zip-bomb guard) — see `cli/internal/fetch/`.
- Path-traversal guard rejects archive entries with absolute paths or `..` components.
- Symlink entries in tarballs are skipped during extraction.
- PAX metadata entries are skipped before top-level-dir detection.
- License allowlist (MIT + BSD-2/3 + ISC + Apache-2.0) enforced at `jutsu lint` time.
- Skill name regex `^[a-z][a-z0-9-]*[a-z0-9]$` — prevents path traversal via skill name.
- `permissions` manifest declared in every `skill.yaml` — `bash`, `network`, `fs-write` (false / scoped / full).

## Sigstore signature verification — enforced (v0.3.1+)

Skills that declare `trust.expected-signer` in `skill.yaml` are sigstore-verified at install time. The CLI:

1. Fetches the signature bundle (`<skill>-<tag>.tar.gz.sig`) from the source release
2. Shells out to `cosign verify-blob` against the canonical kaijutsu-core OIDC identity (or the future per-signer mapping in v0.4)
3. Hard-fails on any mismatch — bundle missing, identity wrong, signature invalid

If `cosign` is not on PATH, install hard-fails with an actionable error. Override with `--no-verify` (logs a warning to stderr) when you accept the risk — for example, when `cosign` is genuinely unavailable on a constrained system.

The `dcg` (destructive command guard) skill ships with `expected-signer: kaijutsu-core@github` from v0.3.1 onward; other core skills will adopt it incrementally as we observe real-world install patterns.

## Stubs not yet enforced (v0.3)

- **Install-time permission prompts**: declared `permissions` in `skill.yaml` are visible via `jutsu info` but not enforced via runtime prompt at install time. Planned for v0.4.

## Intentional unsafe behavior — be aware

- A skill with `permissions.bash: true` can effect arbitrary shell on the user's machine through agent tool calls. This is the inherent trade-off of agent-managed skills; users should review the skill's `SKILL.md` and any `scripts/` content before installing.
- Third-party (non-core) skills are out of the kaijutsu signing tier. Treat them like running any other unverified code from the internet — install only what you can audit.
- Prompt injection is the dominant attack vector. The text inside a `SKILL.md` becomes part of the agent's instructions. A malicious skill can attempt to instruct the agent to read secrets, modify files, or exfiltrate data. Mitigations are mostly social (signing, reputation, review) rather than technical.

## What kaijutsu does NOT do

- We do not sandbox skill execution. Skills run in the same trust boundary as the calling agent.
- We do not proxy or filter network calls made by skills.
- We do not vet third-party skills for malice. The community tier is opt-in trust.

## Recommended hardening for users

1. Install only `core` tier skills until v0.3 enforces signing.
2. Inspect `jutsu info <skill>` before installing; review the `permissions` field.
3. Pin skills via a checked-in `kaijutsu.lock.json`. The integrity hash will change if a tarball is tampered with mid-flight.
4. Run agents under a non-privileged user account where possible.
5. Keep secrets out of any directory the agent has access to.
