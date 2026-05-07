---
name: code-simplification
description: Reduce code without changing behavior. Apply Chesterton's Fence (don't remove what you don't understand) and the Rule of 500 (files over 500 lines usually need splitting). Use when the user says "simplify this", "clean up", "remove dead code", "tighten", or invokes /simplify. Composes polish (review-fix loop) and blunder-hunt (regression risk).
---

# Code Simplification

> Adapted from [`addyosmani/agent-skills`](https://github.com/addyosmani/agent-skills/blob/main/skills/code-simplification) under the MIT License. Copyright (c) Addy Osmani. Modifications by kaijutsu maintainers — composed with `polish` and `blunder-hunt`.

Goal: smaller code, same behavior. The deliverable is a diff that reduces LOC, removes abstractions that earn nothing, and consolidates near-duplicates.

## When to invoke

- A file is over 500 lines
- A module's surface has more functions than it has callers per function
- Duplicate logic spotted in 2+ places
- User says "this feels overengineered" or "clean this up"

## Process

### Step 1: Read before deleting (Chesterton's Fence)

For every candidate removal, ask:
- Why is this here? Find the original commit (`git log -p <file>`) and read the message.
- Who calls it? `grep -r '<symbol>'` or use LSP find-references.
- What test covers it?

If you can't answer all three, DO NOT remove it. The fence exists for a reason; first understand the reason, then decide.

### Step 2: Find candidates

- **Dead code**: unreferenced exports, unreachable branches, unused imports
- **Duplicate logic**: 2+ functions doing the same thing with minor variations
- **Premature abstraction**: a base class / generic with one concrete subclass / instance
- **Leaky helper**: a util that exposes implementation details its only caller could inline
- **File-over-500**: split by cohesion (related functions stay together; orthogonal concerns split out)

### Step 3: Apply Rule of 500

If a single file exceeds 500 LOC:
1. Identify cohesion clusters within the file
2. Split into 2-3 files of <500 LOC each
3. Move tests with their code
4. Verify imports compile

### Step 4: Behavior-preservation gate

After every change:
- Tests still pass
- No public API change (or if there is one, it's marked breaking)
- Diff is reviewable in <10 minutes

If the diff exceeds 500 LOC, you've gone too far — split the simplification into multiple PRs.

### Step 5: Compose blunder-hunt for regression risk

Invoke `blunder-hunt` with a "what might I have broken" lens specifically:
- Caller signatures unchanged?
- Error semantics preserved?
- Concurrency invariants intact?
- Public APIs (HTTP routes, exported types, library entry points) unchanged?

Findings become test additions or revert candidates.

### Step 6: Compose polish

Run `polish` on the simplification PR. Polish's review-fix loop catches the small breakages introduced during the cleanup.

## Anti-rationalization table

| Excuse | Rebuttal |
|---|---|
| "Nobody uses this anymore" | Verify with grep + reverse-deps. Internal repo + public API + dynamic call sites all count. |
| "This is dead code" | If it has a test, the test wouldn't be passing if the code were truly dead. Read the test first. |
| "This abstraction is over-engineered" | Maybe. Or maybe a future caller is planned. Check the git blame; check the issues tracker. |
| "I'll DRY this up" | DAMP > DRY. Tests benefit from descriptiveness over de-duplication. Check each candidate for actual cost-of-duplication vs cost-of-abstraction. |
| "The file is too big" | Length isn't always the right metric — cohesion is. A 600-LOC file with one focused concern beats two 300-LOC files with split-brain logic. |

## Hard rules

- **Never delete what you don't understand.** Read the commit history first.
- **Behavior-preserving means tests pass.** If your "simplification" changes behavior, it's a refactor — different skill.
- **Public APIs are sacred** unless the user explicitly requested a breaking change.
- **One simplification per PR.** Multiple unrelated cleanups in one PR = unreviewable.
- **Compose `blunder-hunt` AND `polish` before declaring done.**

## Composes

- `polish` — review-and-fix loop after the simplification
- `blunder-hunt` — regression-risk lens

## When NOT to use

- Code you've never read before in a domain you don't understand (read first, simplify later)
- Code under active feature development (wait until the feature lands)
- Code with no tests (write tests first, then simplify)
