# security-and-hardening

OWASP-Top-10-aware security review of a diff or target file set. Adapted from [`addyosmani/agent-skills`](https://github.com/addyosmani/agent-skills) under MIT.

Reviews for: injection (SQL/cmd/template/SSRF/XXE), broken auth, sensitive data exposure, broken access control, security misconfig, vulnerable / outdated deps, identification + auth failures, software / data integrity (supply-chain trust), security logging / monitoring failures, server-side request forgery.

```sh
jutsu install security-and-hardening
```

Trigger phrases: "security review", "OWASP check", "hardening", "audit for vulns".

Composes `blunder-hunt` with the hostile-input lens.
