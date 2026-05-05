#!/usr/bin/env bash
# doc-review skill entry point. Thin wrapper around `jutsu swarm doc-review`.
#
# Usage:
#   scripts/run.sh SPEC.md
#   scripts/run.sh PLAN.md SPEC.md           # multi-file concatenated review
#   scripts/run.sh SPEC.md --full --strict   # high-stakes review
#   scripts/run.sh --replay <key>            # re-synthesize from cache
#
# Flags pass through. See `jutsu swarm doc-review --help`.

set -euo pipefail

if ! command -v jutsu >/dev/null 2>&1; then
  echo "fatal: jutsu CLI not on PATH. Install via: brew install momentmaker/tap/jutsu" >&2
  exit 1
fi

exec jutsu swarm doc-review "$@"
