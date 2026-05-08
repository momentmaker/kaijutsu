# `.kaijutsu/autopilot.yaml` — schema reference

Optional per-project config. Autopilot runs with baked-in defaults when this file is absent. `jutsu autopilot init` writes the default config to this path.

## Schema

```yaml
gates:
  brainstorm: review | skip          # default review; skip bypasses Gate 1 (CI-friendly)
  post_spec: review | skip           # default skip; opt-in extra gate after spec doc-review
  post_plan: review | skip           # default skip; opt-in extra gate after plan doc-review
  pr: github                         # always github; not config-overridable

brainstorm:
  preset: dream                       # swarm preset for the brainstorm pass
  personas: [<persona-name>, ...]     # built-ins + agents.yaml-defined names
  mode: quick | full                  # full = round-robin debate
  lenses: base | all | <comma-list>  # dream lens set

spec_review:
  preset: doc-review
  personas: [...]
  mode: quick | full

plan_review:
  preset: doc-review
  personas: [...]
  mode: quick | full

per_stage_polish:
  max_passes: <int>                   # default 4; matches /polish skill cap

final_review:
  preset: pr-review
  personas: [...]
  mode: quick | full
  strict: true | false                # lie-to-them filter on synthesis

findings:
  auto_apply: false                   # always false; report-only by design
  escalate_threshold: blocker | issue | minor

cost:
  max_total_usd: <float>              # soft cap (subject to hard ceiling = $100)
  pause_at_pct: <int>                 # 0-100; pause + confirm at this % of soft cap

orchestration:
  skill_suggest: true | false         # call jutsu suggest at each phase (deferred to v0.12+)
  drift_check: tag | block | off      # reverse-drift behavior
```

## Override precedence

1. CLI flag (`--max-cost N`) overrides config for the run.
2. `.kaijutsu/autopilot.yaml` overrides baked-in defaults.
3. Baked-in defaults apply when nothing above is set.

`KAIJUTSU_AUTOPILOT_HARD_CAP_OVERRIDE` env var raises the **ceiling** (not the soft cap) per-shell. Read once at autopilot start.
