---
name: readme-update
description: Detect README staleness and propose a targeted update. Use when the user says "update readme", "is the readme out of date", "readme refresh", "/readme-update", or after merging a feature that likely changes installation, usage, or public API. Reads the README's last-touched commit, walks the commits since, and proposes a section-by-section diff.
---

# readme-update

Most READMEs decay silently. This skill catches the decay early by anchoring on the last commit that touched README.md, walking forward, and matching what changed against which README sections are likely affected.

## Step 1: Find the README

Look for, in order:
- `README.md` at the repo root
- `README.MD`, `Readme.md`, `readme.md` (case variants)

If multiple Markdown files plausibly serve as the README (e.g., `docs/README.md`), ask the user which one to update.

## Step 2: Find the Last Update

```bash
LAST_SHA=$(git log -n 1 --format="%H" -- README.md)
LAST_DATE=$(git log -n 1 --format="%ad" --date=iso -- README.md)
```

If `LAST_SHA` is empty, the README has never been committed. In that case, ask the user "should I generate a fresh README from scratch?" and stop here.

## Step 3: List Commits Since

```bash
git log "${LAST_SHA}..HEAD" --format="%h %s"
git diff --name-only "${LAST_SHA}..HEAD"
```

If the list is empty, report "README is up to date with HEAD (last touched <date>)" and stop.

## Step 4: Categorize the Changes

Bucket the changed files into themes:

| Theme | File patterns | README sections likely affected |
| --- | --- | --- |
| Dependencies | `package.json`, `go.mod`, `Cargo.toml`, `pyproject.toml`, `requirements.txt`, `Gemfile`, `composer.json` | Installation, Prerequisites |
| Build / scripts | `package.json` scripts, `Makefile`, `justfile`, `Taskfile.yml`, `.github/workflows/*` | Build, Development, CI |
| CLI / binaries | new files under `cmd/`, `bin/`, `src/cli/`, executable shell scripts | Usage, Examples |
| Public API | exported symbols, route handlers, OpenAPI specs, generated SDKs | API, Reference |
| Configuration | `.env.example`, config schemas, settings files | Configuration |
| Schema / data | migration files, prisma/sqlx schemas, GraphQL SDL | Schema, Data Model |
| Docs links | `docs/**` additions or renames | Documentation, Further Reading |

## Step 4.5: Check badges + image staleness

While scanning, audit:
- Version badges in the README header — does the version match the latest tag? Latest release? Latest published package?
- License badges — match LICENSE file?
- Build / lint badges — link still valid? Workflow still exists?
- Embedded images / diagrams — `mtime` newer or older than the related code's `mtime`? An out-of-date architecture diagram is often worse than no diagram.

## Step 5: Read the README

Parse the README's heading structure (h1/h2/h3). Map each detected theme to the matching section.

**Semantic match, not just header text.** A "Quickstart" section may be called "Get Started", "Hello world", "First steps", or simply have no header but lead with `npm install`. Match by content patterns:

- Installation = section that contains install commands or package-manager mentions
- Usage = section with example invocations
- API = section with type signatures or endpoint definitions
- Configuration = section with env vars, config keys, or settings tables

If a theme has no corresponding section, propose a *new* section rather than forcing an edit into an unrelated section.

## Multi-doc support

This skill works on any project markdown, not just `README.md`. Apply the same anchor-on-last-touch + walk-forward pattern to:

- `CONTRIBUTING.md` (contributor flow changed, new tooling required, new conventions)
- `ARCHITECTURE.md` (major refactors, new modules, removed components)
- `CHANGELOG.md` (releases since last touch — auto-generate entries from commits)
- `docs/**/*.md` (any deep doc that references specific code paths)

Pass the doc path explicitly: `/readme-update CONTRIBUTING.md`. Default is `README.md`.

## Step 6: Propose Concrete Edits

For each section that needs an update, propose a unified-diff-style change:

```
## Updates proposed for README.md

### Installation (last touched 2025-12-04, dependencies changed in 4 commits)
- old: `pip install foo==1.0`
- new: `pip install foo==2.0`

### Usage (new commands added)
+ `foo bake` — bake the dough; replaces `foo prepare` (deprecated 1.4.0)

### Configuration (new section)
+ A new section is needed: env vars `FOO_TIMEOUT` and `FOO_RETRIES` are now read by the binary.
```

Include the rationale (what change in the codebase prompted each suggestion) so the user can sanity-check.

## Step 7: Confirm and Apply

Prompt: "Apply these edits to README.md? [y/N]"

On `y`:
- Apply each edit using your agent's edit primitive
- Stage the file but do not commit (leave the commit message + commit moment to the user)

On `n`:
- Print the proposed diff so the user can copy-paste pieces

## Hard rules

- Never commit. The user owns the commit moment.
- Never invent commands, flags, or APIs. If the change since the last README touch isn't obviously documented in the source, mark it as "needs the user to clarify" and skip that section.
- Don't rewrite the whole README in one go. Section-by-section diffs only.
- If the README looks fundamentally outdated (missing required sections like Installation or License), suggest the user run `/readme-init` (or equivalent) first; this skill is for incremental maintenance, not greenfield generation.
