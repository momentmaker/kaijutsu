# spec-driven-development

Author a PRD-style spec before writing implementation. Adapted from [`addyosmani/agent-skills`](https://github.com/addyosmani/agent-skills) under MIT.

The deliverable is a markdown spec at `docs/specs/YYYY-MM-DD-<slug>.md` covering: problem statement, scope, non-goals, acceptance criteria, open questions, risks. A `blunder-hunt` review pass is mandatory before the spec is considered done.

```sh
jutsu install spec-driven-development
```

Trigger phrases: "write a spec", "PRD", "design doc", "spec out", `/spec`.

Composes `blunder-hunt` (final review) and `decide` (architectural decisions).
