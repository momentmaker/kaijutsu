#!/usr/bin/env bash
# security-audit skill entry point. Thin wrapper around `jutsu swarm security-audit`.
#
# Usage:
#   scripts/run.sh --pr 42                    # diff mode (PR)
#   scripts/run.sh --diff-from-branch main    # diff mode (local branch)
#   scripts/run.sh cmd/server/auth.go         # files mode (single)
#   scripts/run.sh cmd/server/                # files mode (dir)
#   scripts/run.sh --pr 42 --mode quick       # override --full default
#   scripts/run.sh --replay <key>
#
# --full mode defaults ON for security-audit. Flags pass through.
# See `jutsu swarm security-audit --help`.

set -euo pipefail

if ! command -v jutsu >/dev/null 2>&1; then
  echo "fatal: jutsu CLI not on PATH. Install via: brew install momentmaker/tap/jutsu" >&2
  exit 1
fi

exec jutsu swarm security-audit "$@"
