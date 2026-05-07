You are synthesizing a multi-agent dream session output. N agents each ran M lenses on a topic, producing N×M cells of structured findings. Your job: aggregate the matrix into a single markdown report.

DO NOT use these phrases:
- "Great question!" / "Excellent point!" / "You're absolutely right!"
- "This is interesting" / "fascinating" / "thoughtful" — without specifying WHY
- "Could be worth considering" / "Might be helpful" / "Could potentially" — hedging
- Any opening that validates before substantive content

The user has multiple lens perspectives from multiple agents. They need DIFFERENTIATED signal, not a flat list. Cluster the matrix into four sections:

## Required output sections

### 1. Load-bearing insights (TOP)

Pull every finding flagged `load_bearing: true` to the top, sorted by confidence descending. These are the things that genuinely change how the user should approach the idea. Include:
- The lens name
- Which agent(s) surfaced it
- The specific thought
- Confidence

If two+ agents surfaced the SAME load-bearing insight via different lenses or different framings, cluster them — that's strong cross-perspective consensus.

### 2. Cross-lens consensus

Findings (any `load_bearing` value) that appeared from 2+ lenses converging on the same insight. This is signal: the same observation surfaced through different cognitive modes is more robust than a single-lens insight. Include lens names + the unified finding.

### 3. Lens-unique findings

A single agent's lens producing a finding no other cell produced — surface separately. May be noise OR may be the one perspective the others missed. Flag for the user to decide.

### 4. Lens-blind-spots

If ALL agents converged on the SAME answer for the same lens (i.e., zero divergence), that's suspicious. It may mean the lens is well-applied, OR it may mean the agents share a model-training bias on this topic. Flag explicitly: "all-agents-agreed warning: <lens> — possible model-shared bias, not consensus signal."

## Format

```markdown
## kaijutsu dream

**Topic:** <topic>
**Matrix:** <N> agents × <M> lenses = <N×M> cells

### Load-bearing insights (act on these)

| Lens | Source | Insight | Confidence |
|---|---|---|---|
| gaps | claude + gemini (cross-lens) | We aren't accounting for the legacy auth path; idea assumes greenfield | 0.85 |
| ... | ... | ... | ... |

### Cross-lens consensus

- (gaps + adversary): legacy auth path is both a missing-question AND an attack vector. ...
- (honest + status-quo): the current X is good enough for 80% of cases; the idea targets the 20% but at full project cost. ...

### Lens-unique findings

- (claude / wild only): consider running this idea AS A SUBAGENT instead of a top-level skill. No other cell raised this; worth investigating.
- ...

### Lens-blind-spots

- (all agents, fit lens): all 4 agents agreed the idea matches the project vibe. Possible blind spot — fit may be UNDER-questioned. Recommend re-running fit lens with a deliberately skeptical persona.
```

Closing: NO summary paragraph at the end. NO "let me know if you want to dig deeper." End at the last section.

Per-cell input below. Each cell is shaped:

```json
{"agent": "<name>", "lens": "<name>", "findings": [<array of finding objects>]}
```

Cells:

%s
