# Reverse-drift gate — runbook

Phase 5 gate that detects divergence between the user-approved spec and the actual implementation.

## What it does

After the final swarm pr-review passes, autopilot calls `swarm.RunPipeline` with the `reverse` preset:

```
swarm.RunPipeline(ctx, swarm.PipelineOpts{
    Preset: <reverse preset>,
    Input: { spec: <state.approved_spec_path>, diff: origin/main...HEAD },
    ...
})
```

The reverse preset categorizes each spec-vs-impl divergence into:

- **ADDED** — implementation has features the spec didn't promise
- **OMITTED** — spec promised features the implementation skipped
- **CHANGED** — implementation differs from how the spec described it
- **AMBIGUOUS** — spec could be read either way; flag for human review

## Output

Drift findings are appended to the PR description as a `<details>` block:

```markdown
<details><summary>autopilot reverse-drift findings</summary>

### ADDED (3)
- ...

### OMITTED (1)
- ...

### CHANGED (2)
- ...

### AMBIGUOUS (1)
- ...

</details>
```

PR is labeled `autopilot-drift` if ANY drift category surfaced findings.

## Behavior

Per spec Decision #9: **informational, not blocking**.

- autopilot does NOT pause or abort on drift findings.
- autopilot does NOT auto-revise the spec or impl to match.
- User reviews drift on GitHub. PR review is Gate 2 — that's where drift gets adjudicated.

## False-positive handling

Reverse-drift on AI-generated specs is noisy in practice. Common false positives:

- **Refactor-only commits** (rename a function): reverse may flag as "CHANGED" even though semantics are identical.
- **Internal helpers** (private functions added during impl): reverse may flag as "ADDED" even though spec naturally couldn't enumerate every internal helper.
- **Test-only changes**: spec describes behavior; tests verify it. Tests being added shouldn't surface as drift.

Mitigation: the reverse preset's `--confidence-threshold` defaults to `0.30` (lower than pr-review's `0.55`) — admits more findings but also more noise. autopilot filters drift findings below `0.30` before surfacing.

## When drift is real

Drift is **real and load-bearing** when:
- Implementation introduced a NEW external dependency the spec didn't approve.
- Implementation skipped a spec section because it was "too hard" — silent scope reduction.
- Implementation chose a different architectural pattern than the spec described.

In all cases: PR review by user is the resolution. autopilot surfaces; user decides.

## Disable

Set `orchestration.drift_check: off` in `.kaijutsu/autopilot.yaml` to skip Phase 5 reverse-drift entirely. Use cases:
- v0.11.x runs where the reverse preset is too noisy on your codebase.
- Greenfield repos where there's no spec to drift from.

`drift_check: block` (instead of `tag`) makes drift findings BLOCK the PR creation. Off by default — matches spec's informational-not-blocking principle.
