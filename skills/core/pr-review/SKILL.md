---
name: pr-review
description: Adversarial multi-agent pull-request review. Runs claude / codex / gemini in parallel via `jutsu swarm pr-review`, each with a tailored prompt (claude=architecture+correctness, codex=edge cases, gemini=cross-file patterns), then synthesizes into a single markdown review with a disagreement table. --full mode adds a Pass-2 round-robin debate where agents critique each other. --strict adds a lie-to-them filter on the synthesis draft. Posts the result as a PR comment when --post-comment is set; edits prior kaijutsu-pr-review comments in place. Use when the user says "review this PR", "code review", "review the diff", "look at PR #N", or invokes /pr-review. Refuses to send the diff unless `.kaijutsu/pr-review.yaml` has `allow-multi-model: true` (first-run prompt persists this). Hard-blocks on a pre-flight secrets scan unless --allow-secrets is passed.
---

# pr-review

Adversarial multi-agent review. The author already saw the obvious. Your job is to surface what they missed by hearing from THREE independent reviewers — each with a different lens — and distilling the conversation.

This skill wraps `jutsu swarm pr-review`. The orchestration primitive is in the Go CLI; this skill provides the per-agent prompts, the workflow, and the user-facing wrapper.

## Layout (rich)

```
pr-review/
├── SKILL.md                     (this file — workflow + decision rules)
├── skill.yaml
├── scripts/
│   └── run.sh                   thin wrapper: shells `jutsu swarm pr-review "$@"`
├── prompts/
│   ├── claude.md                "high-level architecture + correctness"
│   ├── codex.md                 "be brutal on edge cases"
│   ├── gemini.md                "cross-file pattern hunt + consistency"
│   ├── synthesizer.md           cluster, dedupe, render markdown
│   └── debate.md                Pass-2 critique template
├── references/
│   ├── prompt-design.md         why each lens is what it is
│   └── disagreement-rubric.md   how to read the disagreement table
└── runbooks/
    ├── tuning-prompts.md        override prompts per repo
    └── interpreting-output.md   what each section of the comment means
```

The CLI loads prompts from this skill's `prompts/` directory at runtime; if a file is missing it falls back to the built-in default. So authors can override one lens (e.g. add domain-specific rules to gemini.md) without recompiling jutsu.

## How agents collaborate

Three independent passes, one synthesized output:

| Pass | Mode | What runs |
|---|---|---|
| 1 | always | Each agent reviews independently. N model calls. |
| 2 | --full only | Each agent sees others' Pass-1, critiques. N more calls. |
| Synth | always | One agent (default claude) synthesizes into markdown. 1 call. |
| Filter | --strict only | lie-to-them filter trims sycophancy from synthesis. 1 more call. |

**Cost math** (N=3 agents):
- `--quick` (default): 4 calls
- `--full`: 7 calls
- `--full --strict`: 8 calls

`--max-cost N` warns when the estimate exceeds N USD.

## Workflow

When the user says "review this PR", "/pr-review #42", or similar:

1. **Run** — invoke via `scripts/run.sh "$@"` or directly:
   ```bash
   jutsu swarm pr-review --pr 42                 # quick mode, print to stdout
   jutsu swarm pr-review --pr 42 --post-comment  # post to GH after printing
   jutsu swarm pr-review --pr 42 --full          # round-robin debate
   jutsu swarm pr-review --pr 42 --full --strict # debate + lie-to-them filter
   jutsu swarm pr-review --diff-from-branch origin/main  # no PR yet
   ```

2. **First run only** — the CLI prompts:
   > This run will send PR diff to N model provider(s). Persist consent to .kaijutsu/pr-review.yaml? [y/N]

   On `y` it writes `allow-multi-model: true` to the config. Subsequent runs skip the prompt.

3. **Pre-flight secrets scan** — if the diff contains `.env*`, `*.pem`, AWS keys, GH PATs, PEM private blocks, etc., the run hard-blocks. Pass `--allow-secrets` ONLY when the apparent hits are intentional (e.g. test fixtures with placeholder credentials).

4. **Read the output** — the markdown comment has three sections:
   - **Disagreement table** — rows are clustered findings, columns are agents, cells show severity each agent assigned.
   - **Synthesis** — the synthesizer's prose review.
   - **Per-agent stats** — collapsed footer with each agent's finding count + cost estimate.

   The 1/N rows in the disagreement table are the conversation-starters. Read those first.

5. **Iterate via --replay** — once cached, re-run synthesis without re-calling the model APIs:
   ```bash
   jutsu swarm pr-review --replay <sha>
   ```
   Useful for tuning the synthesizer prompt.

## What the synthesis looks like

```markdown
## kaijutsu pr-review

**Findings:** 7 total · 2 consensus · 3 contested

### Disagreement Table

| Finding | Severity | Consensus | claude | codex | gemini |
|---|---|---|---|---|---|
| `auth.go:88` — race in token refresh | blocker | 3/3 | ✓ (blocker) | ✓ (issue) | ✓ (blocker) |
| `cache.go:42` — TTL not honored | issue | 1/3 | — | ✓ (issue) | — |
...

### Synthesis

The change introduces ... (synthesizer's prose) ...

### Disagreements

- **cache.go:42** — codex flagged this as a TTL bug; claude+gemini did not.
  Worth a closer look.

<details><summary>Per-agent stats</summary>
- **claude** — 4 finding(s) · 14s · est $0.082
- **codex** — 5 finding(s) · 18s · est $0.094
- **gemini** — 3 finding(s) · 9s · est $0.041
</details>

<!-- kaijutsu-pr-review:run-id=20260505T153022Z sha=abc1234 -->
```

## Configuration

Per-repo `.kaijutsu/pr-review.yaml`:

```yaml
allow-multi-model: true              # required gate
agents: [claude, gemini]             # opt-out of codex if not auth'd
mode: quick                          # default mode for `/pr-review`
max_cost_usd: 1.00
exclude_paths: [pnpm-lock.yaml, vendor/]
synthesizer: claude
```

## Hard rules

- **Never bypass `allow-multi-model`** without explicit user opt-in. The diff goes to 3 model providers — that's a real privacy decision, not a CLI footgun.
- **Never bypass secrets-scan** silently. If `--allow-secrets` is passed, surface the hits in the output so the user knows what they overrode.
- **Adversarial framing is non-negotiable.** Per-agent prompts deliberately push for findings, not validation. "Looks good to me" with no evidence is forbidden.
- **The disagreement is the signal.** A finding 3/3 agents flag is consensus; one 1/3 agent flags is a conversation-starter. Don't suppress disagreements in the synthesis — call them out.
- **Cite real file:line.** Every finding has a path and line range. Made-up locations destroy credibility.
- **Edit-in-place, don't append.** Re-running on the same PR edits the prior comment. New SHAs append a "Previous reviews" footer to preserve timeline.

## When NOT to use this

- **Tiny one-line diffs** — overkill, expensive. Use a single-agent quick review.
- **Diffs with proprietary code under embargo** — the diff goes to remote model providers. If your repo's policy doesn't allow that, don't run it.
- **CI gate** — Phase 1 is local-only. CI integration arrives later.
