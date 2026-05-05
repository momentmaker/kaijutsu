#!/usr/bin/env bash
# Shared helpers for agent-doctor scripts.
# Multi-agent: scans ~/.claude, ~/.codex, ~/.gemini, ~/.agents.
# Targets macOS BSD userland (the user's environment); GNU coreutils
# also work — find/du/stat flags chosen for portability where possible.

set -uo pipefail

# Default agent dirs probed by doctor + cleanup. Override by setting
# AGENT_DIRS to a colon-separated list before invoking the scripts.
DEFAULT_AGENT_DIRS=(
  "$HOME/.claude"
  "$HOME/.codex"
  "$HOME/.gemini"
  "$HOME/.agents"
)

if [[ -n "${AGENT_DIRS:-}" ]]; then
  IFS=':' read -ra AGENT_DIR_LIST <<< "$AGENT_DIRS"
else
  AGENT_DIR_LIST=("${DEFAULT_AGENT_DIRS[@]}")
fi

# Colors — degrade in non-tty.
if [[ -t 1 ]] && command -v tput &>/dev/null; then
  C_BOLD=$(tput bold) C_DIM=$(tput dim)
  C_RED=$(tput setaf 1) C_GREEN=$(tput setaf 2)
  C_YELLOW=$(tput setaf 3) C_BLUE=$(tput setaf 4)
  C_RESET=$(tput sgr0)
else
  C_BOLD="" C_DIM="" C_RED="" C_GREEN="" C_YELLOW="" C_BLUE="" C_RESET=""
fi

# Bytes -> human (no GNU numfmt dependency).
human_bytes() {
  awk -v b="${1:-0}" 'BEGIN {
    split("B K M G T P", u)
    i = 1
    while (b >= 1024 && i < 6) { b /= 1024; i++ }
    if (i == 1) printf "%dB", b
    else        printf "%.1f%s", b, u[i]
  }'
}

# Cross-platform stat -size.
stat_size() {
  if stat -f%z "$1" >/dev/null 2>&1; then
    stat -f%z "$1" 2>/dev/null
  else
    stat -c%s "$1" 2>/dev/null
  fi
}

# Sum of byte sizes for matched paths. Args: passed verbatim to `find`.
size_of() {
  local total=0 sz f
  while IFS= read -r -d '' f; do
    [[ -e "$f" ]] || continue
    if [[ -d "$f" ]]; then
      sz=$(du -sk "$f" 2>/dev/null | awk '{print $1*1024}')
    else
      sz=$(stat_size "$f" || echo 0)
    fi
    total=$((total + ${sz:-0}))
  done < <(find "$@" -print0 2>/dev/null)
  echo "$total"
}

count_of() {
  find "$@" 2>/dev/null | wc -l | tr -d ' '
}

# Section / metric printers
section() {
  printf "\n%s%s== %s ==%s\n" "$C_BOLD" "$C_BLUE" "$1" "$C_RESET"
}

metric() {
  printf "  %-32s %s%8s%s  %s(%s items)%s\n" \
    "$1" "$C_BOLD" "$(human_bytes "$2")" "$C_RESET" \
    "$C_DIM" "$3" "$C_RESET"
}

# Per-agent protected path computation.
# Args: $1 = AGENT_DIR (e.g. ~/.claude). Sets the global
# PROTECTED_PATHS array for the current agent.
compute_protected() {
  local d="$1"
  PROTECTED_PATHS=(
    "$d/CLAUDE.md"
    "$d/AGENTS.md"
    "$d/GEMINI.md"
    "$d/agents"
    "$d/skills"
    "$d/rules"
    "$d/hooks"
    "$d/identities"
    "$d/plans"
    "$d/settings.json"
    "$d/settings.local.json"
    "$d/keybindings.json"
    "$d/ecosystem.yaml"
    "$d/config.toml"
    "$d/outreach"
    "$d/outreach-log.md"
  )
}

# Filename + path rules that protect data anywhere in a tree.
# Returns 0 (true) if target is protected.
is_protected() {
  local target="$1" base p
  base=$(basename "$target")
  # Memory protection — anywhere in the tree.
  [[ "$target" == */memory || "$target" == */memory/* ]] && return 0
  case "$base" in
    MEMORY.md|CLAUDE.md|AGENTS.md|GEMINI.md) return 0 ;;
  esac
  # Directories that contain memory/ or MEMORY.md must not be moved
  # wholesale — caller has to extract memory first or skip.
  if [[ -d "$target" ]]; then
    [[ -d "$target/memory" ]] && return 0
    [[ -e "$target/MEMORY.md" ]] && return 0
  fi
  for p in "${PROTECTED_PATHS[@]}"; do
    [[ "$target" == "$p" || "$target" == "$p"/* ]] && return 0
  done
  return 1
}

# Move to trash with original relative path preserved.
# Args: $1 = src, $2 = AGENT_DIR.
TRASH_BATCH=""
trash_init() {
  local agent_dir="$1"
  TRASH_BATCH="$agent_dir/.trash/$(date +%Y-%m-%d-%H%M%S)"
  mkdir -p "$TRASH_BATCH"
}
trash_move() {
  local src="$1" agent_dir="$2" rel dest
  if is_protected "$src"; then
    return 2
  fi
  [[ -n "$TRASH_BATCH" ]] || trash_init "$agent_dir"
  rel="${src#$agent_dir/}"
  dest="$TRASH_BATCH/$rel"
  mkdir -p "$(dirname "$dest")"
  mv "$src" "$dest"
}

# Resolve an encoded project dir name to a real filesystem path.
# Encoded form (Claude/Codex/Gemini share this convention): leading "-"
# then path with "/" replaced by "-". Ambiguity: repo names can contain
# dashes too. Strategy — try the "most slashes" partition first; rejoin
# rightmost segments with "-" and re-test on each iteration.
# Returns the first existing dir, or non-zero if no match.
resolve_project_dir() {
  local base="$1" trimmed prefix suffix candidate
  trimmed="${base:1}"
  IFS='-' read -ra p <<< "$trimmed"
  local n=${#p[@]} i has_empty=0 x
  for (( i=n; i>=1; i-- )); do
    prefix="/$(IFS=/; echo "${p[*]:0:i}")"
    suffix=""
    if (( i < n )); then
      suffix="-$(IFS=-; echo "${p[*]:i}")"
    fi
    candidate="${prefix}${suffix}"
    if [[ -d "$candidate" ]]; then
      echo "$candidate"
      return 0
    fi
  done
  for x in "${p[@]}"; do [[ -z "$x" ]] && has_empty=1; done
  if (( has_empty )); then
    local pp=()
    for x in "${p[@]}"; do
      [[ -z "$x" ]] && pp+=(".") || pp+=("$x")
    done
    for (( i=n; i>=1; i-- )); do
      prefix="/$(IFS=/; echo "${pp[*]:0:i}")"
      suffix=""
      if (( i < n )); then
        suffix="-$(IFS=-; echo "${pp[*]:i}")"
      fi
      candidate="${prefix}${suffix}"
      if [[ -d "$candidate" ]]; then
        echo "$candidate"
        return 0
      fi
    done
  fi
  return 1
}
