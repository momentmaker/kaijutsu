# Per-lens design

Why each agent gets a different prompt for security-audit.

## Principle: orthogonal angles cover the InfoSec attack surface

A single-lens security review misses cross-cutting vulnerabilities. An auth-only reviewer doesn't catch supply-chain risk. A dep-only reviewer doesn't catch authorization gaps. The three lenses below cover most of the OWASP Top-10 surface area between them, with explicit overlap on the highest-stakes categories (auth, injection, RCE).

## claude — auth + data flow

Best at:
- Reasoning about trust boundaries — where authenticated data becomes authoritative, where unauthenticated input enters privileged code paths
- Identity propagation — does the request's identity flow correctly through every layer?
- Authz vs authn — does the change conflate "who" with "what they can do"?
- Data flow from sources (user input) to sinks (queries, exec, file paths)

What claude tends to MISS in security:
- Concrete injection payloads (codex's strength)
- Dep CVE knowledge (gemini's strength)

## codex — injection + privilege escalation

Best at:
- Concrete attack vectors — "this regex allows ReDoS via input X", "this query is vulnerable to SQLi via input Y"
- Crypto misuse — wrong algorithm, wrong mode, weak random, hardcoded key, missing IV/MAC
- Race conditions / TOCTOU
- Privilege boundary violations

What codex tends to MISS:
- High-level trust boundary reasoning (claude's strength)
- Dep CVE knowledge (gemini's strength)

## gemini — dependency + supply-chain

Best at:
- Known CVE awareness for popular libraries
- Version-pin analysis — exact version vs range vs tag, supply-chain risk
- Transitive trust — does this dep pull in deps with post-install scripts or native code?
- License compatibility (security-adjacent: a GPL dep in an MIT project is a legal risk that often surfaces alongside security findings)

What gemini tends to MISS:
- In-code injection vectors (codex's strength)
- Auth-flow reasoning (claude's strength)

## Synthesizer

Different shape from pr-review/doc-review's synthesizer. Security-audit's synthesizer:

1. Clusters findings that map to the same vulnerability (cross-lens convergence on the same CVE or attack path is the strongest signal)
2. Produces a threat-model entry per cluster: vulnerability + attack vector + impact + severity + mitigation
3. Sorts by CVSS severity (critical → informational), ties broken by reviewer count

The disagreement section is highest-leverage — single-lens findings (1/N) are often where one specialist caught what the generalists missed. The synthesizer surfaces these prominently and recommends explicit triage.

## Debate (Pass-2 critique, --full default)

For security-audit specifically, --full defaults ON. Pass-2 critique catches:
- False-positive severity inflation (CVSS-critical assigned to a defense-in-depth gap)
- Wrong mitigations (fixing symptom not cause)
- Cascading findings (peer A's auth-bypass implies peer B's unrelated code path is now reachable)

The asymmetric cost of false-positives in security review (they erode user trust faster than missed issues) justifies the doubled cost of --full mode by default.

## Override per-repo

Each prompt is a single .md file in `prompts/`. To override locally:

```bash
mkdir -p .claude/skills/security-audit/prompts
cp ~/.claude/skills/security-audit/prompts/codex.md .claude/skills/security-audit/prompts/codex.md
$EDITOR .claude/skills/security-audit/prompts/codex.md
```

Useful for repo-specific threat model priors:
- "Our biggest risk is multi-tenant data leak — emphasize tenant-isolation checks in claude's auth review"
- "We use crypto/rand exclusively — flag any math/rand in security-relevant code as critical"
- "Our deps are auto-pinned by renovate — gemini can deprioritize version-pin findings"
