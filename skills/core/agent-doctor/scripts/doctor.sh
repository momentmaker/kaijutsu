#!/usr/bin/env bash
# agent-doctor: read-only health/disk report.
# Loops over ~/.claude, ~/.codex, ~/.gemini, ~/.agents and reports on
# every directory that exists. Never modifies anything.

set -uo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "$HERE/lib.sh"

audit_one() {
  local AGENT_DIR="$1"
  [[ -d "$AGENT_DIR" ]] || return 0
  compute_protected "$AGENT_DIR"
  local TRASH_DIR="$AGENT_DIR/.trash"

  printf "\n%sagent-doctor%s  %s%s%s\n" "$C_BOLD" "$C_RESET" "$C_DIM" "$AGENT_DIR" "$C_RESET"

  section "Disk Overview"
  local total
  total=$(du -sk "$AGENT_DIR" 2>/dev/null | awk '{print $1*1024}')
  printf "  total: %s%s%s\n\n" "$C_BOLD" "$(human_bytes "$total")" "$C_RESET"
  du -sk "$AGENT_DIR"/* 2>/dev/null | sort -rn | head -10 | while read -r kb path; do
    local name bytes
    name=$(basename "$path")
    bytes=$((kb * 1024))
    printf "  %-32s %8s\n" "$name" "$(human_bytes "$bytes")"
  done

  section "Stale Targets (safe tier)"
  local tel_size tel_count pc_size pc_count ic_size ic_count fh_size fh_count sw_size sw_count
  tel_size=$(size_of "$AGENT_DIR/telemetry" -type f -mtime +7)
  tel_count=$(count_of "$AGENT_DIR/telemetry" -type f -mtime +7)
  metric "telemetry >7d" "$tel_size" "$tel_count"

  pc_size=$(size_of "$AGENT_DIR/paste-cache" -type f -mtime +30)
  pc_count=$(count_of "$AGENT_DIR/paste-cache" -type f -mtime +30)
  metric "paste-cache >30d" "$pc_size" "$pc_count"

  ic_size=$(size_of "$AGENT_DIR/image-cache" -type f -mtime +30)
  ic_count=$(count_of "$AGENT_DIR/image-cache" -type f -mtime +30)
  metric "image-cache >30d" "$ic_size" "$ic_count"

  fh_size=$(size_of "$AGENT_DIR/file-history" -maxdepth 1 -mindepth 1 -type d -mtime +90)
  fh_count=$(count_of "$AGENT_DIR/file-history" -maxdepth 1 -mindepth 1 -type d -mtime +90)
  metric "file-history >90d" "$fh_size" "$fh_count"

  sw_size=$(size_of "$AGENT_DIR" -maxdepth 1 -name 'security_warnings_state_*.json' -mtime +14)
  sw_count=$(count_of "$AGENT_DIR" -maxdepth 1 -name 'security_warnings_state_*.json' -mtime +14)
  metric "stale security warnings >14d" "$sw_size" "$sw_count"

  section "Orphaned Project Sessions"
  local orphan_total=0 orphan_count=0
  if [[ -d "$AGENT_DIR/projects" ]]; then
    for d in "$AGENT_DIR/projects"/*; do
      [[ -d "$d" ]] || continue
      local base decoded sz
      base=$(basename "$d")
      [[ "$base" == -* ]] || continue
      if ! resolve_project_dir "$base" >/dev/null; then
        sz=$(du -sk "$d" 2>/dev/null | awk '{print $1*1024}')
        orphan_total=$((orphan_total + sz))
        orphan_count=$((orphan_count + 1))
        decoded="/$(echo "${base:1}" | tr '-' '/')"
        printf "  %s%s%s  %s  %s(repo gone; tried path: %s)%s\n" \
          "$C_YELLOW" "$base" "$C_RESET" "$(human_bytes "$sz")" \
          "$C_DIM" "$decoded" "$C_RESET"
      fi
    done
  fi
  [[ $orphan_count -eq 0 ]] && printf "  %snone%s\n" "$C_GREEN" "$C_RESET"
  echo
  metric "orphan total" "$orphan_total" "$orphan_count"

  section "Compressible (aggressive tier preview)"
  local comp_size comp_count
  comp_size=$(size_of "$AGENT_DIR/projects" -type f -name '*.jsonl' -mtime +180 -size +1M)
  comp_count=$(count_of "$AGENT_DIR/projects" -type f -name '*.jsonl' -mtime +180 -size +1M)
  metric "*.jsonl >180d, >1MB" "$comp_size" "$comp_count"

  section "Trash"
  if [[ -d "$TRASH_DIR" ]]; then
    local trash_size trash_count oldest ttl_count
    trash_size=$(du -sk "$TRASH_DIR" 2>/dev/null | awk '{print $1*1024}')
    trash_count=$(find "$TRASH_DIR" -maxdepth 1 -mindepth 1 -type d 2>/dev/null | wc -l | tr -d ' ')
    oldest=$(ls -1tr "$TRASH_DIR" 2>/dev/null | head -1)
    ttl_count=$(count_of "$TRASH_DIR" -maxdepth 1 -mindepth 1 -type d -mtime +30)
    metric "trash" "$trash_size" "$trash_count"
    printf "  %soldest batch: %s  |  due for purge (>30d): %s%s\n" \
      "$C_DIM" "${oldest:-none}" "$ttl_count" "$C_RESET"
  else
    printf "  %sempty (will be created at %s on first cleanup)%s\n" "$C_DIM" "$TRASH_DIR" "$C_RESET"
  fi

  section "Health"
  local settings_file=""
  for sf in "$AGENT_DIR/settings.json" "$AGENT_DIR/config.toml"; do
    [[ -f "$sf" ]] && { settings_file="$sf"; break; }
  done
  if [[ -n "$settings_file" ]]; then
    local broken=0
    while IFS= read -r path; do
      [[ -z "$path" ]] && continue
      local expanded="${path/#\~/$HOME}"
      [[ -e "$expanded" ]] || { broken=$((broken + 1)); printf "  %sbroken hook ref:%s %s\n" "$C_RED" "$C_RESET" "$path"; }
    done < <(grep -oE '"command"[[:space:]]*:[[:space:]]*"[^"]+"' "$settings_file" 2>/dev/null \
              | grep -oE '(~|/)[^"[:space:]]+' | head -50)
    [[ $broken -eq 0 ]] && printf "  %shooks: ok%s\n" "$C_GREEN" "$C_RESET"
  else
    printf "  %sno settings.json/config.toml — skipping hook check%s\n" "$C_DIM" "$C_RESET"
  fi

  section "Recoverable (safe tier total)"
  local recoverable=$((tel_size + pc_size + ic_size + fh_size + sw_size + orphan_total))
  printf "  %s%s%s%s reclaimable safely\n" "$C_BOLD" "$C_GREEN" "$(human_bytes "$recoverable")" "$C_RESET"
  printf "  %srun: AGENT_DIR=%s %s/cleanup.sh --dry-run%s\n" "$C_DIM" "$AGENT_DIR" "$HERE" "$C_RESET"
}

found=0
for d in "${AGENT_DIR_LIST[@]}"; do
  if [[ -d "$d" ]]; then
    audit_one "$d"
    found=1
  fi
done
if (( found == 0 )); then
  printf "%sno agent dirs found%s under: %s\n" "$C_YELLOW" "$C_RESET" "${AGENT_DIR_LIST[*]}"
  printf "  Set AGENT_DIRS=path1:path2 to override.\n"
  exit 0
fi
