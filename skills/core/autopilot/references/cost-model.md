# Cost model — soft cap + hard ceiling + env override

Three layers of cost enforcement.

## Layer 1: Soft cap (default $20)

- Source: `.kaijutsu/autopilot.yaml::cost.max_total_usd`, falls back to baked-in default of `$20`.
- Per-run override: `--max-cost N` flag on `jutsu autopilot run`.
- Behavior:
  - Mid-run: at `cost.pause_at_pct` (default 50%), autopilot pauses + asks user to confirm continue.
  - At soft cap: autopilot pauses + requires explicit `--raise-cap N` to continue.

## Layer 2: Hard ceiling ($100)

- Source: `MaxAutopilotCostUSD = 100.0` constant in `cli/internal/cli/autopilot.go`.
- Cannot be raised by editing `.kaijutsu/autopilot.yaml`. Yaml is in-repo writable; ceiling lives in compiled Go binary.
- `--max-cost N` flag is rejected if `N` exceeds the ceiling.
- Skill code is updateable via PR, but `skills/core/` is signed at release time (sigstore); cosign verification gates install. yaml is unsigned + per-project. Different attack-surface → different defense.

## Layer 3: Env-var override

- `KAIJUTSU_AUTOPILOT_HARD_CAP_OVERRIDE=N` raises the ceiling per-shell.
- Read once at autopilot start.
- Not stored in repo; user sets in shell init (rc file, direnv, or single-shell export).
- Use case: rare $200/run advanced runs (large codebase migrations).

## Cost projection

Pre-flight `jutsu swarm <preset> --estimate` runs at Gate 1 against the brainstorm pass. Aggregated estimate shown to user before Gate 1 approval.

## Reverse-drift cost

Phase 5 reverse-drift gate runs an additional `swarm.RunPipeline` with the `reverse` preset. Cost is intentional (different lens than `pr-review`) — drift detection compares spec to impl, pr-review hunts bugs. Same cost as a normal `swarm reverse` invocation.

## Ratio check

Mid-run: if actual per-call cost > 2× pre-flight estimate, autopilot warns. If 5×, autopilot pauses + escalates.
