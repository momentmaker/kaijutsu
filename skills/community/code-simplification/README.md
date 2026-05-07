# code-simplification

Reduce code without changing behavior. Adapted from [`addyosmani/agent-skills`](https://github.com/addyosmani/agent-skills) under MIT.

Applies Chesterton's Fence (don't remove what you don't understand) and the Rule of 500 (if a file is over 500 lines, it probably needs splitting). Finds dead code, duplicate logic, unused abstractions, leaky helpers, premature factoring.

```sh
jutsu install code-simplification
```

Trigger phrases: "simplify this", "remove dead code", "code cleanup", "tighten this".

Composes `polish` (review-fix loop) and `blunder-hunt` (regression risk).
