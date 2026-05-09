# IMPLEMENTATION_PLAN.md — v0.13.0

Spec: `docs/specs/2026-05-08-v0.13.0-preset-sdk.md`
Branch: `v0.13.0-preset-sdk`

Three stages: (1) schema + load + registry-merge + validate command, (2) cobra dynamic registration + per-InputKind flag wiring, (3) reference examples + housekeeping + final pr-review.

## Stage 1: Schema + LoadUserPresets + registry-merge + validate command

**Goal**: yaml schema, load function with parse-failure-tolerant best-effort, registry merge with hard-error on built-in shadowing, `jutsu swarm validate` cobra command.

**Success Criteria**:
- New `schemas/swarm.schema.json` — JSON Schema covering every field per spec Decision #4 (name pattern, inputKind enum, defaultPrompt + synthesizer with `%s` requirement, severityVocab predefined-string-list, mode enum, personas string-list, confidenceFloor 0.0-1.0). Compiles cleanly under AJV in `lint-skills.yml::validate-schemas`.
- New `cli/internal/swarm/user_presets.go` exposes `LoadUserPresets(projectRoot, homeDir string) (map[string]*UserPreset, []error, error)`. Both directory paths are explicit args (no `os.UserHomeDir()` call inside) so tests can inject any tmp dir; the cli wrapper resolves `homeDir` once via `os.UserHomeDir()` before calling. Project entries shadow home entries on cross-file collision; intra-file duplicates reject the whole file per Decision #6.
- New `UserPreset` type embeds `Preset` + `Source UserPresetSource` (project / home).
- New `(r *PresetRegistry) RegisterUserPresets(map[string]*UserPreset) (registered []string, collisions []error)` — hard-errors on built-in name collision; valid entries still register. Returns the slice of NAMES that registered cleanly so the caller can iterate ONLY successfully-registered presets when adding cobra subcommands (no string-matching against error messages to filter failures).
- New `cli/internal/cli/swarm_validate.go` — `jutsu swarm validate [path]` subcommand. Default path = `.kaijutsu/swarm.yaml` if it exists; arg overrides. Runs schema validation + name-collision check; reports findings; exits 0 clean, non-zero on issues. Schema is `//go:embed`'d into the binary at build time (new `cli/internal/swarm/schema_embed.go` with `//go:embed swarm.schema.json` directive) so the validate subcommand has runtime access without depending on the install location. Schema file is the same `schemas/swarm.schema.json` from repo root, copied into the swarm package at build time via a small `go generate` rule (or vendored — pick whichever is simpler; favor vendored copy for build-time-determinism).
- Built-in subcommands (`pr-review`, `dream`, etc.) unchanged; root cmd still constructs cleanly when both yamls are missing.

**Tests**:
- `cli/internal/swarm/user_presets_test.go`:
  - `TestLoadUserPresets_ProjectShadowsHome` — same name in both yamls; project wins.
  - `TestLoadUserPresets_IntraFileDuplicateRejectsFile` — two presets with same `name:` in one yaml; whole file rejected with citation; OTHER yaml still loads.
  - `TestLoadUserPresets_MalformedYamlSurfacesAsError` — broken yaml → fatal error returned, NOT silent skip.
  - `TestLoadUserPresets_InvalidEntrySkipped` — one valid + one invalid entry in same file → valid registered, invalid in warnings.
  - `TestLoadUserPresets_MissingFilesNoError` — neither yaml exists → nil map, no error.
  - `TestLoadUserPresets_HomeOnlyLoads` — only home yaml exists (project absent) → home entries load + appear with `Source: SourceHome`.
  - `TestLoadUserPresets_ProjectOnlyLoads` — only project yaml exists (home absent) → project entries load + appear with `Source: SourceProject`.
  - `TestLoadUserPresets_RequiresOneSlotInDefaultPrompt` — DefaultPrompt with 0 or 2 `%s` slots → validation warning; entry skipped.
  - `TestLoadUserPresets_RequiresOneSlotInSynthesizer` — same for Synthesizer.
- `cli/internal/swarm/registry_test.go`:
  - `TestRegisterUserPresets_RejectsBuiltinShadow` — user preset name `pr-review` → returns collision error; built-in still findable.
  - `TestRegisterUserPresets_RegistersValidEntries` — user preset with unique name → findable via `Find`.
  - `TestRegisterUserPresets_PartialFailurePreservesValidEntries` — mix of valid + colliding names; valid entries register, errors returned for the rest.
- `cli/internal/cli/swarm_validate_test.go`:
  - `TestSwarmValidate_CleanYamlExitsZero` — valid swarm.yaml → exit 0.
  - `TestSwarmValidate_NameCollisionExitsNonZero` — yaml with `name: pr-review` → non-zero exit + error names the conflict.
  - `TestSwarmValidate_MissingFileErrorsHelpfully` — pass nonexistent path → cobra error with "file not found" + suggestion.
  - `TestSwarmValidate_FieldCitationOnBrokenEntry` — yaml with empty description → error names the field + line.

**Status**: Not Started

## Stage 2: Cobra dynamic registration + per-InputKind flag wiring

**Goal**: At root cmd construction, load both yamls + register each user preset as a `jutsu swarm <name>` subcommand. Per-InputKind flag wiring reuses existing patterns from built-in subcommands.

**Success Criteria**:
- New `cli/internal/cli/swarm_user.go`:
  - `addUserPresetSubcommands(parent *cobra.Command, registry *swarm.PresetRegistry, projectRoot, homeDir string, stderrW io.Writer)` — called from `newSwarmCmd()`. Registry + dirs passed explicitly (dependency injection; no global state). Loads via `swarm.LoadUserPresets(projectRoot, homeDir)`, calls `registry.RegisterUserPresets`, surfaces warnings + collision errors + load errors to stderr (NOT panics), iterates the `registered []string` return + adds one cobra subcommand per name.
  - `newUserPresetSubcommand(name string, p *swarm.UserPreset) *cobra.Command` — selects the correct flag-binding helper based on `p.InputKind`:
    - `InputDiff` → reuses `--pr` / `--diff-from-branch` like pr-review's subcommand
    - `InputFiles` → positional paths like doc-review's subcommand
    - `InputPrompt` → positional prompt like brainstorm's subcommand
  - Subcommand `Short` field tagged `[user:project]` or `[user:home]` per spec Decision #8.
- `cli/internal/cli/swarm.go::newSwarmCmd()` calls `addUserPresetSubcommands` after the existing `cmd.AddCommand(...)` calls for built-ins. Built-in registration is unaffected.
- **Registration ordering**: `RegisterUserPresets` must happen at root construction time (before any `RunE` fires) so that when a `RunE` calls `LoadPresetWithSkillOverrides(name)` at command execution, the user preset is already in the registry. Cobra's `RunE` fires at command-execution (after `Execute()`), not at construction; the registration step happens during `newSwarmCmd()` so it's complete by the time any subcommand's `RunE` runs.
- Yaml parse error → stderr warning + zero user-preset subcommands; ALL built-in subcommands work normally (zero-impact failure mode).
- `recover` block around `addUserPresetSubcommands` defends against an unexpected panic in user-yaml handling.

**Tests**:
- `cli/internal/cli/swarm_user_test.go`:
  - `TestAddUserPresetSubcommands_RegistersFromValidYaml` — synthetic project root with one valid `concise-review` entry; `parent.Commands()` contains a `concise-review` subcommand.
  - `TestAddUserPresetSubcommands_TagsSourceInShort` — registered subcommand's `Short` ends with `[user:project]`.
  - `TestAddUserPresetSubcommands_BrokenYamlDoesNotBreakRoot` — malformed `.kaijutsu/swarm.yaml`; root cmd still constructs; built-in subcommands findable; stderr contains a warning.
  - `TestAddUserPresetSubcommands_PanicRecovers` — synthetic input that triggers a panic deep inside loading; recovery block catches it; root cmd works.
  - `TestNewUserPresetSubcommand_FlagSetForInputDiff` — InputDiff preset → resulting subcommand has `--pr` + `--diff-from-branch` flags.
  - `TestNewUserPresetSubcommand_FlagSetForInputFiles` — InputFiles preset → subcommand accepts positional path args.
  - `TestNewUserPresetSubcommand_FlagSetForInputPrompt` — InputPrompt preset → subcommand accepts positional prompt arg.
  - `TestSwarmHelp_DifferentiatesUserFromBuiltin` — root `swarm --help` output contains both built-in (no tag) and user (with `[user:...]` tag) preset rows.
  - `TestUserPresetEndToEnd_RegistryLookupSucceedsAtRunE` — end-to-end pin: synthetic project root with one user preset; cobra subcommand registered; subcommand's `RunE` calls `swarm.LoadPresetWithSkillOverrides(name)` against the registry and finds the preset. Validates that registration ordering actually delivers — not just that the subcommand exists.

**Status**: Not Started

## Stage 3: Reference examples + housekeeping + final pr-review

**Goal**: 3 reference `swarm.yaml` files in `docs/examples/swarm/`, marker bump, CHANGELOG/ROADMAP/README, final swarm pr-review pass.

**Success Criteria**:
- New `docs/examples/swarm/concise-pr-review.yaml`:
  - Single preset `concise-pr-review`, InputDiff.
  - Synthesizer prompt drops the disagreement-table block from pr-review's built-in synthesizer and emits a top-3-findings-only summary. Concrete delta: pr-review's synthesizer emits both "## Disagreement Table" and "## Synthesis" sections; this preset's synthesizer emits ONLY a 5-row top-findings table — no disagreement breakdown. Acceptance check: spawn the example yaml, dispatch a stub run, assert the rendered markdown contains zero `## Disagreement` headers and at most 5 finding entries.
  - Personas: `[paranoid-security-claude, claim-auditor-claude]` (2 instead of pr-review's auto-detect 3).
  - Mode: `quick`. ConfidenceFloor: `0.55` (matches pr-review default).
  - Leading comment block: copy this to `.kaijutsu/swarm.yaml`, edit, run `jutsu swarm validate`, then `jutsu swarm concise-pr-review --diff-from-branch main`.
- New `docs/examples/swarm/deep-test-gap.yaml`:
  - Single preset `deep-test-gap`, InputFiles.
  - DefaultPrompt extends test-gap's missing-scenarios framing with explicit "imagine attacker with intermediate domain knowledge" framing.
  - Personas: `[paranoid-security-claude, claim-auditor-claude, performance-deepseek, perf-purist-codex]` (4 personas; cost trade-off documented in the comment block).
  - Mode: `full`. ConfidenceFloor: `0.30` (matches test-gap default).
- New `docs/examples/swarm/legacy-audit.yaml`:
  - Single preset `legacy-audit`, InputFiles.
  - Synthesizer prompt blends code-archaeology's "WHY does this look this way" framing with security-audit's "what could break" lens. Single-preset composition; no auxiliary input plumbing per Decision #3.
  - Personas: `[architecture-purist-gemini, paranoid-security-claude, claim-auditor-claude]`.
  - Run with `jutsu swarm legacy-audit <path-to-some-legacy-code>` — comment block in the yaml uses placeholder paths (NOT specific files in this repo) so the example doesn't break when those files move/rename. Example invocation in the comment cites two example paths users can substitute.
- All 3 examples pass `jutsu swarm validate <path>`.
- `cli/internal/cli/init_agents_fragment.go` — marker `0.12.0` → `0.13.0`. `init_agents_fragment_test.go` assertion updated.
- `CHANGELOG.md` `[0.13.0]` entry: 3 stages summarized + the v0.13 ordering reversal context (Preset SDK landed before Persona SDK after a `jutsu swarm dream` adversarial pass on the original Persona-SDK-first plan surfaced 3-agent consensus that real user pain is lens routing, not lens authoring — so authoring SDK was reordered to v0.14+) + the dream session's lens-blindspot warnings noted as ongoing concerns (RLHF convergence, status-quo bias, model-shared adversary archetypes) + v0.13.x read-only persona-browse + v0.14+ Persona SDK gating conditions.
- `ROADMAP.md` — Preset SDK row [ ] → [x]; v0.13.x persona-browse row added; v0.14+ Persona SDK gating-condition row preserved.
- `README.md` v0.13 section: how to write swarm.yaml + pointer to the 3 examples + when to use yaml vs built-in.
- All existing tests stay green.

**Final swarm pr-review** (after Stage 3 commits, before merge): `jutsu swarm pr-review --diff-from-branch main --strict --max-cost 5.0 --yes` against `main..HEAD`. Apply load-bearing findings; defensible-skip the rest.

**Tests**:
- `cli/internal/cli/init_agents_fragment_test.go` — `version=0.13.0` assertion.
- `docs/examples/swarm/*.yaml` — manual `jutsu swarm validate` smoke (NOT a Go test; verified in Verification block below).

**Status**: Not Started

---

## End-state verification (after all 3 stages)

```bash
brew upgrade momentmaker/tap/jutsu
jutsu --version                                              # 0.13.0

# Validate the bundled examples
jutsu swarm validate docs/examples/swarm/concise-pr-review.yaml
jutsu swarm validate docs/examples/swarm/deep-test-gap.yaml
jutsu swarm validate docs/examples/swarm/legacy-audit.yaml

# Smoke: copy an example into a real project
mkdir /tmp/v013-smoke && cd /tmp/v013-smoke
git init && jutsu init
mkdir -p .kaijutsu
cp <repo>/docs/examples/swarm/concise-pr-review.yaml .kaijutsu/swarm.yaml
jutsu swarm --help | grep concise-pr-review                  # appears with [user:project] tag
jutsu swarm concise-pr-review --estimate --diff-from-branch main

# Built-in shadow rejection
cat <<'YAML' >> .kaijutsu/swarm.yaml
- name: pr-review
  description: shadows built-in
  inputKind: diff
  defaultPrompt: "%s"
  synthesizer: "%s"
  severityVocab: [issue]
YAML
jutsu swarm validate .kaijutsu/swarm.yaml                    # error names the conflict

# Marker
jutsu init && grep version=0.13.0 AGENTS.md

# CI
cd <repo>/cli && go test ./... -count=1                      # every package green
```

CI:
- `lint-skills.yml::validate-schemas` adds `swarm.schema.json` to the AJV compile set.
- `release-jutsu.yml` produces v0.13.0 tarballs + brew tap update.

---

## Squash + tag

After Stage 3 + final swarm pr-review pass:

```bash
git checkout main
git merge --no-ff v0.13.0-preset-sdk -m "Merge v0.13.0-preset-sdk: ..."
git tag -a v0.13.0 -m "v0.13.0 — Preset SDK (swarm.yaml per-project + per-user)"
git push origin main
git push origin v0.13.0
brew upgrade momentmaker/tap/jutsu
```

---

## Risks (recap from spec)

| Risk | Mitigation |
|---|---|
| User yaml parse error breaks ALL swarm commands | Stage 2 best-effort load + recover; built-in subcommands NEVER blocked |
| User preset's prompt has %s count mismatch | Stage 1 schema validation requires exactly-one slot; `jutsu swarm validate` surfaces before runtime |
| Built-in shadowing hard-error feels too strict | Defensible per spec Decision #2; alternative names are unique + the rule prevents docs/yaml drift |
| Help-text bloat with many user presets | Acceptable for v0.13; v0.13.x can add `jutsu swarm list --user` if usage data shows the bloat |
| Dream "ghost town" — built-ins cover 90%, SDK sees ~zero adoption | v0.13.x track via findings.db (preset_name, source); if <5% from user presets at 90 days, demote Persona SDK ambitions, refocus built-in depth |
| RLHF convergence erodes cross-model dispatch (deeper dream concern) | Tracked separately as v0.12.x A/B benchmark commitment; v0.13 ships routing surface either way |
