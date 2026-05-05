---
name: security-audit
description: Multi-agent security audit. Runs claude / codex / gemini in parallel through three CVSS-aligned lenses (claude=auth+data flow, codex=injection+priv-esc, gemini=dep+supply-chain). Wraps `jutsu swarm security-audit --pr <n>` (diff mode) or `jutsu swarm security-audit <path>...` (files mode). Output is a threat model with attack vectors + prioritized mitigations. --full mode defaults ON because security findings are high-stakes; the Pass-2 debate filters false-positives that would erode trust. Use when the user says "audit this for security", "threat model", "check for vulns", "review the auth flow", or invokes /security-audit.
---

# security-audit

Adversarial multi-agent security review. Different from pr-review's broad code review — security-audit's three lenses are explicitly InfoSec-shaped, the severity vocab is CVSS-aligned, and the output is a threat model rather than a code-review findings list.

## How agents collaborate

| Lens | Agent | Looks for |
|---|---|---|
| **Auth + data flow** | claude | Trust boundaries, identity propagation, authorization vs authentication, data flow to sinks |
| **Injection + priv-esc** | codex | SQL/command/XSS injection, path traversal, SSRF, race conditions, crypto misuse |
| **Dependency + supply-chain** | gemini | Known CVEs, version drift, transitive trust, license compatibility |

Severity vocab (CVSS-aligned):
- **critical** — RCE, auth bypass, privilege escalation
- **high** — data breach, integrity violation
- **medium** — DoS, info disclosure, weak boundaries
- **low** — defense-in-depth gap, timing leak
- **informational** — worth noting but no exploitable path

## Two input modes

```bash
# Diff mode — audit a PR
jutsu swarm security-audit --pr 42

# Diff mode — audit a local branch (no PR yet)
jutsu swarm security-audit --diff-from-branch origin/main

# Files mode — audit specific paths
jutsu swarm security-audit cmd/server/auth.go

# Files mode — audit a directory (multi-file concatenated)
jutsu swarm security-audit cmd/server/

# --quick to override the default --full mode
jutsu swarm security-audit --pr 42 --mode quick

# Iterate cheap on synth prompt
jutsu swarm security-audit --replay <key>
```

The two modes are mutually exclusive — passing `--pr` AND positional file paths errors with a "pick one" hint.

## --full defaults ON

Unlike other presets, security-audit runs `--full` mode (with Pass-2 round-robin debate) by default. Security findings have asymmetric cost: false-positives erode user trust faster than missed issues. The debate round filters false-positives by having each agent critique its peers.

Override to `--mode quick` only when you're doing a fast first-pass scan.

## First-run consent

Before the first invocation in a repo:

```bash
jutsu swarm security-audit --grant-consent
```

Persists `allow-multi-model: true` to `.kaijutsu/security-audit.yaml`. Diffs/files go to remote model providers; explicit consent required.

**Important caveat for security-audit specifically:** the input may contain code that itself contains secrets, vulnerabilities, or proprietary IP. The pre-flight secrets-scan blocks any obvious credential leaks (.env files, AWS keys, etc.) — but it doesn't catch every form of sensitive data. If the audit target has compliance or NDA constraints, consider running on redacted excerpts or skip security-audit entirely in favor of a self-hosted setup (Phase 3+).

## Reading the output

```markdown
### Threat model

#### Auth bypass via missing token validation — critical (3/3 reviewers)
- **Where**: `cmd/server/auth.go:42-58`
- **Attack vector**: Anonymous request with no Authorization header reaches the protected handler because the middleware short-circuits on err != nil.
- **Impact**: Full account takeover for any user whose ID an attacker can guess.
- **Mitigation**: Treat ErrTokenMissing as 401 explicitly; don't let the err-handling fall through.

#### SQL injection in search query — high (2/3 reviewers)
- ...

#### Stale dependency with active CVE — medium (1/3 reviewers, gemini)
- ...

### Disagreements
- 1/N rows are usually the highest-leverage. The gemini-only finding
  on the stale dep would have been missed by single-lens review.
```

The disagreement table (rendered separately) shows which lens caught what. For security-audit, 1/N findings ARE the value — single-lens review misses cross-lens vulnerabilities.

## Cache + collision prevention

Cache key is salted with the preset name, so a `pr-review --pr 42` and `security-audit --pr 42` produce distinct cache dirs:

- `.kaijutsu/pr-review-runs/<sha>/` (pr-review of PR 42)
- `.kaijutsu/security-audit-runs/<sha>/` (security-audit of PR 42)

Same diff can be reviewed by both presets without overwriting either's cache.

## Composes

Chain naturally with:
- `pr-review` — ran first; security-audit is the security-specific deep dive when pr-review surfaced security-adjacent concerns
- `decide` — record any security tradeoffs surfaced (e.g., "we accepted medium-severity timing leak because mitigation cost > impact")
- `dcg` — already in the kaijutsu core; runtime guard for destructive shell commands. security-audit catches design-level issues; dcg catches execution-level ones.

## Hard rules

- **Never `--post-comment` for security-audit.** The `--post-comment` flag is NOT registered on this subcommand — posting vuln details to a public PR is a leak. If you need to share findings, route them through your team's private channels.
- **CVSS severities are not a substitute for triage.** A "high" finding from security-audit is multi-model output, not an actual CVSS-scored vulnerability. Use the output as the seed for a real triage process, not as the final verdict.
- **`--allow-secrets` is even more dangerous here than for pr-review.** The audited code likely contains credentials by definition (auth flows, key handling). Consider redacting before running OR running on a sandboxed snapshot.

## When NOT to use

- **General code review** — use pr-review.
- **Spec/design review** — use doc-review.
- **One-line auth fix** — overkill; manual review is fine.
- **Compliance-grade audit** — multi-model heuristic review is NOT a substitute for a real security audit by a human auditor. Use security-audit as a fast first-pass only.

## Cost

Default `--max-cost $1.00`. With 3 agents in `--full --strict` (the default), typical security-audit runs ~$0.30–1.00 depending on input size. The debate round adds ~50% to base cost; the strict-filter adds another synthesis call. Override `--max-cost` higher for big audits.
