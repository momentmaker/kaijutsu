---
name: security-and-hardening
description: OWASP-Top-10-aware security review of a diff or target file set. Use when the user says "security review", "OWASP check", "audit for vulns", "harden this", "security audit", or invokes /security. Composes blunder-hunt's hostile-input lens. Surfaces injection, auth gaps, broken access control, sensitive data exposure, and supply-chain risks.
---

# Security + Hardening

> Adapted from [`addyosmani/agent-skills`](https://github.com/addyosmani/agent-skills/blob/main/skills/security-and-hardening) under the MIT License. Copyright (c) Addy Osmani. Modifications by kaijutsu maintainers — composed with `blunder-hunt`.

A focused review pass that walks the OWASP Top 10 against a code change set. The deliverable is a list of findings, each with severity, evidence, and remediation.

## When to invoke

- Before merging changes that touch auth, sessions, payment, PII, or external network calls
- After taking on a new dependency
- After a security-relevant CVE in the project's stack
- User says "security review" / "audit" / "harden"

## Process

### Step 1: Identify the target

- PR diff: `gh pr diff <n>`
- Branch diff: `git diff main...HEAD`
- File set: explicit list from user

### Step 2: Walk the OWASP Top 10

For each category, scan the target. Tick yes / no per finding.

| # | Category | Look for |
|---|---|---|
| A01 | Broken Access Control | missing authz checks, IDOR, path traversal, allowed-by-default routes |
| A02 | Cryptographic Failures | hardcoded secrets, weak ciphers (MD5/SHA1), missing TLS, plaintext PII |
| A03 | Injection | SQL/NoSQL/LDAP/cmd/template injection, unsanitized concat into queries / shells / templates |
| A04 | Insecure Design | trust boundaries crossed without validation, ambient authority, no rate-limit on expensive ops |
| A05 | Security Misconfiguration | default creds, debug endpoints exposed, verbose error pages, permissive CORS |
| A06 | Vulnerable / Outdated Components | dep versions with known CVEs, deprecated transitive deps |
| A07 | Identification + Auth Failures | weak session mgmt, predictable session IDs, missing rate-limit on login, missing MFA gate |
| A08 | Software / Data Integrity | unsigned artifacts, supply-chain trust gaps, missing SRI for external scripts |
| A09 | Logging + Monitoring Failures | no audit log on auth events, no alerting on lockouts, secrets in logs |
| A10 | Server-Side Request Forgery (SSRF) | user-controlled URLs in fetch/curl, allow-list missing, internal IPs not blocked |

### Step 3: Compose blunder-hunt with hostile-input lens

After the OWASP sweep, invoke `blunder-hunt` with the hostile-input lens specifically:

> "Find every place where untrusted data flows into a trust boundary."

This catches subclasses of injection that don't fit neatly into the table above.

### Step 4: Format findings

For each finding:

```
[severity] <file>:<line>
Category: <OWASP A0X>
Issue: <one-line>
Evidence: <quote / git blame / repro steps>
Remediation: <concrete fix>
```

Severities: `critical` (immediate fix), `high` (fix this PR), `medium` (file an issue, fix this sprint), `low` (FYI).

### Step 5: Output

Write to a markdown file (`docs/security-reviews/YYYY-MM-DD-<target>.md`) AND post inline as PR comments if a PR is in scope (compose `pr-review` if installed).

## Anti-rationalization table

| Excuse | Rebuttal |
|---|---|
| "We're not exposed to the internet" | Internal threats matter too. Compromised employee laptop / supply-chain attack surface remains. |
| "The framework handles auth" | Frameworks handle SOME auth. Application-layer authz (object-level) is your job. |
| "We use prepared statements everywhere" | Then prove it. One missed concat in one query is the whole compromise. |
| "Our deps are pinned" | Pinning ≠ secure. Run `npm audit` / `cargo audit` / `pip-audit` and report. |
| "It's just a small change" | Auth bypass via 3-line change is the classic story. Size doesn't predict risk. |

## Hard rules

- **Never approve a security review without evidence per finding.** Vague "looks fine" is forbidden.
- **Critical findings block merge.** No "we'll fix it next PR" for critical.
- **Test the proposed fix.** A remediation that doesn't actually mitigate is worse than nothing — the user thinks they're protected.
- **Document supply-chain decisions.** When a dep gets a CVE, the decision to upgrade-now vs. defer is a `decide`-worthy moment.

## Composes

- `blunder-hunt` — hostile-input lens layered onto the OWASP sweep
- `pr-review` (optional) — post findings as inline PR comments if a PR is in scope
- `decide` (optional) — record any architectural security decisions surfaced
