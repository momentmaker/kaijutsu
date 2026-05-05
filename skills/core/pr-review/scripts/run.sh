#!/usr/bin/env bash
# pr-review skill entry point. Thin wrapper around `jutsu swarm pr-review`.
#
# Usage:
#   scripts/run.sh                           # auto-detect PR, quick mode
#   scripts/run.sh --pr 42                   # specific PR
#   scripts/run.sh --pr 42 --post-comment    # post to GH after synthesis
#   scripts/run.sh --diff-from-branch origin/main   # no PR yet
#   scripts/run.sh --full --strict --post-comment   # round-robin debate
#   scripts/run.sh --replay <sha>            # re-synthesize from cache
#
# All flags pass through. See `jutsu swarm pr-review --help`.

set -euo pipefail

if ! command -v jutsu >/dev/null 2>&1; then
  echo "fatal: jutsu CLI not on PATH. Install via: brew install momentmaker/tap/jutsu" >&2
  exit 1
fi

exec jutsu swarm pr-review "$@"
