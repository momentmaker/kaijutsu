#!/usr/bin/env bash
# refactor-plan skill entry point. Thin wrapper around `jutsu swarm refactor-plan`.
#
# Usage:
#   scripts/run.sh handlers/users.go --goal "extract auth into middleware"
#   scripts/run.sh handlers/*.go --goal "split into per-resource services" --full
#   scripts/run.sh --replay <key>
#
# --goal is REQUIRED unless --replay or --grant-consent is set.
# Flags pass through. See `jutsu swarm refactor-plan --help`.

set -euo pipefail

if ! command -v jutsu >/dev/null 2>&1; then
  echo "fatal: jutsu CLI not on PATH. Install via: brew install momentmaker/tap/jutsu" >&2
  exit 1
fi

exec jutsu swarm refactor-plan "$@"
