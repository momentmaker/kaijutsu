#!/bin/sh
# kaijutsu uninstaller — removes the jutsu binary.
#
# Usage:
#   curl -fsSL https://kaijutsu.dev/uninstall.sh | sh
#
# Override the binary location:
#   curl -fsSL https://kaijutsu.dev/uninstall.sh | KAIJUTSU_BIN=/usr/local/bin/jutsu sh
#
# Also nuke ~/.kaijutsu/ (global manifest + lockfile + cache):
#   curl -fsSL https://kaijutsu.dev/uninstall.sh | KAIJUTSU_PURGE=1 sh
#
# Globally-installed skills under ~/.claude/skills/ and ~/.agents/skills/
# are left in place. Use `jutsu remove -g <skill>` before uninstalling
# the binary if you want them gone too.

set -eu

BIN="${KAIJUTSU_BIN:-}"
if [ -z "$BIN" ]; then
  if command -v jutsu >/dev/null 2>&1; then
    BIN=$(command -v jutsu)
  else
    BIN="${HOME}/.local/bin/jutsu"
  fi
fi

if [ ! -e "$BIN" ]; then
  echo "kaijutsu: jutsu binary not found at ${BIN}" >&2
  echo "kaijutsu: nothing to uninstall (set KAIJUTSU_BIN to override the search)." >&2
  exit 0
fi

rm -f "$BIN"
echo "kaijutsu: removed $BIN"

if [ "${KAIJUTSU_PURGE:-0}" = "1" ]; then
  if [ -d "${HOME}/.kaijutsu" ]; then
    rm -rf "${HOME}/.kaijutsu"
    echo "kaijutsu: purged ~/.kaijutsu/"
  fi
fi

echo "kaijutsu: done. Note that any installed skills under ~/.claude/skills/"
echo "kaijutsu: and ~/.agents/skills/ are left in place; remove them by hand"
echo "kaijutsu: if you want a fully clean slate."
