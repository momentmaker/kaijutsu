You are synthesizing a multi-agent DREAM session output. N agents each ran M lenses on a topic, producing N×M cells of structured findings. Aggregate the matrix into a single markdown report with EXACTLY four sections.

DO NOT use these phrases:
- "Great question!" / "Excellent point!" / "You're absolutely right!"
- "This is interesting" / "fascinating" / "thoughtful" — without specifying WHY
- Any opening that validates before substantive content

Required sections, in this order:

### 1. Load-bearing insights (TOP)
Pull every finding whose reasoning starts with "load_bearing: true" — sorted by confidence descending. Include lens name (parsed from summary's [lens:<name>] prefix), source agent(s), the thought, confidence. Cluster cross-agent agreement.

### 2. Cross-lens consensus
Findings (any load_bearing value) that appeared from 2+ DIFFERENT lenses converging on the same insight. Same-lens cross-agent matches go in the load-bearing section if applicable. This section is for CROSS-LENS robustness signal.

### 3. Lens-unique findings
A single agent's lens producing an insight no other cell produced — surface separately. May be noise OR may be the one perspective the others missed.

### 4. Lens-blind-spots
If ALL agents converged on the SAME answer for the SAME lens (zero cross-agent divergence within a lens), flag it: "all-agents-agreed warning: <lens> — possible model-shared bias, not consensus signal."

HARD STOP RULE. End the output at the last section. Do NOT write a closing paragraph, summary, or "let me know if you want to dig deeper." If you find yourself starting any sentence after the final table that doesn't BELONG to one of the four sections, STOP — that sentence is the coda, and dream output must not have one.

Per-cell input below. Each finding's lens identity lives in the summary's [lens:<name>] prefix; load_bearing flag lives in the reasoning's leading "load_bearing: true|false" token.

REVIEWERS' FINDINGS:
%s
