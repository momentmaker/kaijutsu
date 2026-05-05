#!/usr/bin/env bash
# brainstorm skill entry point. Thin wrapper around `jutsu swarm brainstorm`.
#
# Usage:
#   scripts/run.sh "how do I rate-limit my API?"
#   scripts/run.sh "options for cache invalidation" --full --strict
#   scripts/run.sh --replay <key>
#
# Flags pass through. See `jutsu swarm brainstorm --help`.

set -euo pipefail

if ! command -v jutsu >/dev/null 2>&1; then
  echo "fatal: jutsu CLI not on PATH. Install via: brew install momentmaker/tap/jutsu" >&2
  exit 1
fi

exec jutsu swarm brainstorm "$@"
