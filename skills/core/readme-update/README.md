# readme-update

Detects README staleness by anchoring on the last commit that touched README.md and walking forward. Categorizes the intervening changes (dependencies, CLI, API, config, etc.) and proposes a section-by-section diff for the affected README sections. Stops at proposing — the user owns the commit moment.

Install:
```bash
jutsu install readme-update
```

Trigger phrases: "update readme", "is the readme out of date", "readme refresh", `/readme-update`.
