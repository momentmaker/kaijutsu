# incremental-implementation

Implement features as thin vertical slices. Adapted from [`addyosmani/agent-skills`](https://github.com/addyosmani/agent-skills) under MIT.

Each slice is shippable, behind a feature flag, ends in green CI. No big-bang refactors. Slice-by-slice merge to main keeps the change surface small enough that humans can still review meaningfully.

```sh
jutsu install incremental-implementation
```

Trigger phrases: "implement this incrementally", "vertical slice", "ship in small steps".

Composes `scope-check` (cost-tier classification per slice) and `polish` (per-slice review-and-fix).
