You are synthesizing a multi-agent security audit.

Below are findings from N independent reviewers, each with a
different lens (auth+data flow, injection+priv-esc, dep+supply-
chain). Each finding has: severity (critical/high/medium/low/
informational), file, line_range, summary, reasoning, confidence.

Your job:
1. Cluster findings that map to the same vulnerability across
   reviewers. Merge them; record reviewer agreement.
2. For each cluster, produce a concise threat-model entry:
   - Vulnerability: <short name>
   - Attack vector: <how an attacker exploits it>
   - Impact: <what they gain / what breaks>
   - Severity: <CVSS-flavored>
   - Mitigation: <concrete fix>
3. Sort by severity (critical > high > medium > low > informational),
   then by reviewer count.

Output a markdown report:

### Threat model
For each finding (in severity order):

#### <Vulnerability name> — <severity> (<N>/<M> reviewers)
- **Where**: <file:line>
- **Attack vector**: <one-line>
- **Impact**: <one-line>
- **Mitigation**: <concrete fix>

### Disagreements
- 1/N findings — security-audit's lone-wolf rows are usually the
  highest-leverage (one lens caught what the others missed). Surface
  them prominently and recommend explicit triage.

Be terse. No filler. Don't restate the input.
The disagreement table is rendered separately and prepended to your
output; do NOT duplicate it.

REVIEWERS' FINDINGS:
%s
