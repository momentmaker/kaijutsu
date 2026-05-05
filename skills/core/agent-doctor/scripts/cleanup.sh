#!/usr/bin/env bash
# agent-doctor cleanup: staged cleanup with dry-run default, trash-not-delete.
# Multi-agent: scans every detected agent dir unless AGENT_DIR is set.

set -uo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "$HERE/lib.sh"

DRY_RUN=1
TIER="safe"
TARGET="all"
CONFIRMED=0

usage() {
  cat <<EOF
Usage: cleanup.sh [--apply] [--tier safe|aggressive] [--target T]

  --dry-run             Show what would happen (default).
  --apply               Actually move files to trash.
  --tier safe           (default) telemetry>7d, paste-cache>30d, image-cache>30d,
                        file-history>90d, security_warnings>14d, orphaned project dirs.
  --tier aggressive     safe + jsonls>180d compressed (zstd) + trash>30d hard-deleted.
  --target T            Filter to one of: all sessions telemetry file-history paste-cache
                        image-cache warnings trash orphans.  Default: all.
  --yes                 Skip the "type APPLY" confirmation gate when using --apply.
  -h, --help            This.

By default scans ~/.claude, ~/.codex, ~/.gemini, ~/.agents.
Set AGENT_DIR=path to limit to one agent.
Set AGENT_DIRS=path1:path2 to override the full list.
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --apply) DRY_RUN=0; shift ;;
    --dry-run) DRY_RUN=1; shift ;;
    --tier) TIER="${2:-}"; shift 2 ;;
    --target) TARGET="${2:-}"; shift 2 ;;
    --yes) CONFIRMED=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown arg: $1" >&2; usage; exit 2 ;;
  esac
done

case "$TIER" in safe|aggressive) ;; *) echo "bad --tier: $TIER" >&2; exit 2 ;; esac
case "$TARGET" in all|sessions|telemetry|file-history|paste-cache|image-cache|warnings|trash|orphans) ;;
  *) echo "bad --target: $TARGET" >&2; exit 2 ;;
esac

# Apply gate.
if (( DRY_RUN == 0 && CONFIRMED == 0 )); then
  if [[ -t 0 ]]; then
    printf "%sAbout to move files to trash.%s tier=%s target=%s\n" "$C_BOLD" "$C_RESET" "$TIER" "$TARGET"
    printf "Type %sAPPLY%s to confirm: " "$C_BOLD" "$C_RESET"
    read -r ans
    [[ "$ans" == "APPLY" ]] || { echo "aborted."; exit 1; }
  else
    echo "non-interactive --apply requires --yes" >&2
    exit 2
  fi
fi

want() { [[ "$TARGET" == "all" || "$TARGET" == "$1" ]]; }

if [[ -n "${AGENT_DIR:-}" ]]; then
  RUN_DIRS=("$AGENT_DIR")
else
  RUN_DIRS=("${AGENT_DIR_LIST[@]}")
fi

reclaimed_bytes=0
acted=0

clean_one() {
  local AGENT_DIR="$1"
  [[ -d "$AGENT_DIR" ]] || return 0
  compute_protected "$AGENT_DIR"
  local TRASH_DIR="$AGENT_DIR/.trash"
  TRASH_BATCH=""

  if (( DRY_RUN )); then
    printf "\n%s%sDRY RUN%s  %s  tier=%s  target=%s\n" "$C_BOLD" "$C_YELLOW" "$C_RESET" "$AGENT_DIR" "$TIER" "$TARGET"
    printf "%s  re-run with --apply to commit%s\n" "$C_DIM" "$C_RESET"
  else
    trash_init "$AGENT_DIR"
    printf "\n%s%sAPPLY%s  %s  tier=%s  target=%s\n" "$C_BOLD" "$C_RED" "$C_RESET" "$AGENT_DIR" "$TIER" "$TARGET"
    printf "%s  trash batch: %s%s\n" "$C_DIM" "$TRASH_BATCH" "$C_RESET"
  fi

  act() {
    local path="$1" reason="$2" sz=0
    if [[ -d "$path" ]]; then
      sz=$(du -sk "$path" 2>/dev/null | awk '{print $1*1024}')
    elif [[ -e "$path" ]]; then
      sz=$(stat_size "$path" || echo 0)
    else
      return
    fi
    if is_protected "$path"; then
      printf "  %sSKIP (protected):%s %s\n" "$C_YELLOW" "$C_RESET" "$path"
      return
    fi
    if (( DRY_RUN )); then
      printf "  %s[dry] would trash:%s %s  %s(%s, %s)%s\n" \
        "$C_DIM" "$C_RESET" "$path" "$C_DIM" "$(human_bytes "$sz")" "$reason" "$C_RESET"
    else
      if trash_move "$path" "$AGENT_DIR"; then
        printf "  %strashed:%s %s  %s(%s)%s\n" \
          "$C_GREEN" "$C_RESET" "$path" "$C_DIM" "$(human_bytes "$sz")" "$C_RESET"
        reclaimed_bytes=$((reclaimed_bytes + sz))
      else
        printf "  %sfail:%s %s\n" "$C_RED" "$C_RESET" "$path"
        return
      fi
    fi
    acted=$((acted + 1))
  }

  if want telemetry; then
    section "telemetry >7d"
    while IFS= read -r f; do act "$f" "telemetry"; done < <(find "$AGENT_DIR/telemetry" -type f -mtime +7 2>/dev/null)
  fi

  if want paste-cache; then
    section "paste-cache >30d"
    while IFS= read -r f; do act "$f" "paste-cache"; done < <(find "$AGENT_DIR/paste-cache" -type f -mtime +30 2>/dev/null)
  fi

  if want image-cache; then
    section "image-cache >30d"
    while IFS= read -r f; do act "$f" "image-cache"; done < <(find "$AGENT_DIR/image-cache" -type f -mtime +30 2>/dev/null)
  fi

  if want file-history; then
    section "file-history >90d"
    while IFS= read -r d; do act "$d" "file-history"; done < <(find "$AGENT_DIR/file-history" -maxdepth 1 -mindepth 1 -type d -mtime +90 2>/dev/null)
  fi

  if want warnings; then
    section "stale security_warnings_state >14d"
    while IFS= read -r f; do act "$f" "warning"; done < <(find "$AGENT_DIR" -maxdepth 1 -name 'security_warnings_state_*.json' -mtime +14 2>/dev/null)
  fi

  if want orphans; then
    section "orphaned project sessions (repo gone)"
    if [[ -d "$AGENT_DIR/projects" ]]; then
      for d in "$AGENT_DIR/projects"/*; do
        [[ -d "$d" ]] || continue
        local base decoded
        base=$(basename "$d")
        [[ "$base" == -* ]] || continue
        if ! resolve_project_dir "$base" >/dev/null; then
          decoded="/$(echo "${base:1}" | tr '-' '/')"
          act "$d" "orphan: tried path $decoded"
        fi
      done
    fi
  fi

  if [[ "$TIER" == "aggressive" ]]; then
    if want sessions; then
      section "sessions: compress *.jsonl >180d (>1MB) with zstd"
      if ! command -v zstd &>/dev/null; then
        printf "  %szstd not installed — skipping. Install: brew install zstd%s\n" "$C_YELLOW" "$C_RESET"
      else
        while IFS= read -r f; do
          local sz new_sz saved
          sz=$(stat_size "$f" || echo 0)
          if (( DRY_RUN )); then
            printf "  %s[dry] would zstd:%s %s  %s(%s)%s\n" \
              "$C_DIM" "$C_RESET" "$f" "$C_DIM" "$(human_bytes "$sz")" "$C_RESET"
          else
            if zstd -q --rm "$f"; then
              new_sz=$(stat_size "${f}.zst" || echo 0)
              saved=$((sz - new_sz))
              reclaimed_bytes=$((reclaimed_bytes + saved))
              printf "  %scompressed:%s %s.zst  %s(saved %s)%s\n" \
                "$C_GREEN" "$C_RESET" "$f" "$C_DIM" "$(human_bytes "$saved")" "$C_RESET"
              acted=$((acted + 1))
            else
              printf "  %sfail:%s %s\n" "$C_RED" "$C_RESET" "$f"
            fi
          fi
        done < <(find "$AGENT_DIR/projects" -type f -name '*.jsonl' -mtime +180 -size +1M 2>/dev/null)
      fi
    fi
    if want trash; then
      section "trash: hard-purge batches >30d"
      while IFS= read -r batch; do
        local sz
        sz=$(du -sk "$batch" 2>/dev/null | awk '{print $1*1024}')
        if (( DRY_RUN )); then
          printf "  %s[dry] would rm -rf:%s %s  %s(%s)%s\n" \
            "$C_DIM" "$C_RESET" "$batch" "$C_DIM" "$(human_bytes "$sz")" "$C_RESET"
        else
          if rm -rf "$batch"; then
            printf "  %spurged:%s %s  %s(%s)%s\n" \
              "$C_GREEN" "$C_RESET" "$batch" "$C_DIM" "$(human_bytes "$sz")" "$C_RESET"
            reclaimed_bytes=$((reclaimed_bytes + sz))
            acted=$((acted + 1))
          fi
        fi
      done < <(find "$TRASH_DIR" -maxdepth 1 -mindepth 1 -type d -mtime +30 2>/dev/null)
    fi
  fi
}

for d in "${RUN_DIRS[@]}"; do
  clean_one "$d"
done

# Footer
echo
if (( DRY_RUN )); then
  printf "%sdry-run done.%s  %s items would be touched.  re-run with %s--apply%s to commit.\n" \
    "$C_BOLD" "$C_RESET" "$acted" "$C_BOLD" "$C_RESET"
else
  printf "%sdone.%s  %s items moved/compressed.  reclaimed: %s%s%s\n" \
    "$C_BOLD" "$C_RESET" "$acted" "$C_BOLD" "$(human_bytes "$reclaimed_bytes")" "$C_RESET"
fi
