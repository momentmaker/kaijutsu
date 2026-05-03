# polish

Automated post-implementation review-and-fix loop. Runs up to 4 passes; each pass reviews the diff, lists findings (CRITICAL / ISSUE / MINOR), fixes all of them, and verifies with tests/linters. Stops early on the first clean pass.

Install:
```bash
jutsu install polish
```

Trigger: `/polish` after a feature is implemented and before requesting review.
