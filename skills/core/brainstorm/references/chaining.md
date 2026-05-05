# Chaining brainstorm with other skills

`brainstorm` is the IDEATION primitive — it generates options. It rarely produces a final artifact on its own. Real usage chains it with `decide`, `refactor-plan`, or `spec-driven-development`.

## Chain 1: Brainstorm → Decide

```
User: "Should we use Redis or Memcached for our cache layer?"

  → /brainstorm "options for cache layer"
       → ranked options (Redis sliding-window, Memcached LRU, Cloudflare Workers KV, ...)

  → User picks one based on the ranked list

  → /decide "Use Redis with sliding-window TTL for cache layer"
       → 3 questions (alternatives, why, tripwire)
       → multi-agent review (Step 5) via doc-review on the draft
       → finalize the ADR

  → Implementation in a separate session, against the recorded decision
```

Brainstorm surfaces options. Decide records the choice. Two distinct skills, two distinct artifacts.

## Chain 2: Brainstorm → Refactor-plan (Phase 2 Stage 6)

```
User: "How should we restructure our HTTP layer?"

  → /brainstorm "options for restructuring HTTP layer"
       → ranked options (extract handler-per-endpoint service, fold into existing router, ...)

  → User picks one

  → /refactor-plan handlers/ --goal "extract into per-endpoint service"
       → ordered step list with risk per step
       → doc-review the plan before execution
```

Brainstorm = pick the approach. refactor-plan = execute that approach. Each gets its own multi-agent pass at the appropriate level of detail.

## Chain 3: Brainstorm → Spec-driven-development

```
User: "We need to add team workspaces; not sure how to model it"

  → /brainstorm "options for modeling team workspaces"
       → 4 options: shared-table multi-tenancy, schema-per-tenant, ...

  → User narrows to 2 candidates worth speccing

  → /spec for each candidate (parallel sessions if you want)
       → write the PRD with concrete acceptance criteria
       → doc-review the spec

  → User picks based on the two specs side-by-side
```

Brainstorm widens the option space. Spec narrows each option to acceptance criteria. Together you get "we considered N approaches; here's the one we picked, here's the spec."

## Anti-pattern: brainstorm-for-everything

Don't use brainstorm when:
- You already know the answer (skip to decide / implement)
- The question is "review my X" (use doc-review or pr-review)
- The question requires running tools (brainstorm is prompt-only — no file reads, no shell)

Brainstorm is for the moment when you DON'T know what the right answer is and want a structured way to explore. Once you know, move to the next skill.

## Cost discipline

Three agents × `--quick` ≈ $0.05–0.20. Three agents × `--full` ≈ 2× that. Three agents × `--full --strict` ≈ 3× that.

Default `--max-cost $1.00` is a safety belt. Practical brainstorms come in well under. Watch the per-agent stats footer — if one agent burns 4× the budget of the others, your prompt is too long or that agent's lens prompt has been overridden poorly.
