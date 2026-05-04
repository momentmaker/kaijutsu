# convergence-detect

Detects when an iterative refinement loop has stopped producing new signal, using three quantitative signals (output size shrinking, new-item ratio dropping, content similarity rising). Used by `polish`, `session-retro`, and other iterative skills as a stop condition smarter than "ran N rounds".

Install:
```bash
jutsu install convergence-detect
```

Composed via `deps.skills` by skills that run iterative loops. Trigger phrases: "convergence", "are we done iterating", `/convergence-detect`.
