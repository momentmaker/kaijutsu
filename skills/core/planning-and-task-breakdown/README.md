# planning-and-task-breakdown

Decompose a spec into a verifiable task list. Adapted from [`addyosmani/agent-skills`](https://github.com/addyosmani/agent-skills) under MIT.

Output: a markdown plan at `docs/plans/YYYY-MM-DD-<slug>.md` with vertical slices, explicit dependencies, per-task acceptance criteria, and an estimate column. A `blunder-hunt` + `scope-check` review pass is mandatory before tasks are considered ready for implementation.

```sh
jutsu install planning-and-task-breakdown
```

Trigger phrases: "break this down", "task list", "make a plan", `/plan`.

Composes `blunder-hunt` (final review) and `scope-check` (cost-tier classification).
