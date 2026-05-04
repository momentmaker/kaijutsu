#!/usr/bin/env bash
# dcg.sh — Destructive Command Guard
#
# Reads agent hook JSON from stdin. If the proposed shell command matches
# a known destructive pattern, prints a stopReason JSON and exits non-zero
# to block the action. Otherwise exits 0 to let it proceed.
#
# Designed for the Claude PreToolUse / Codex PreToolUse / Gemini BeforeTool
# hook protocols. All three pass JSON on stdin and treat non-zero exit
# as "block" (Claude reads stopReason from stdout JSON when present).

set -euo pipefail

input=$(cat)

# Best-effort: extract the command. Hook input shape differs slightly
# across agents but all of them surface the shell command somewhere
# inside the JSON. Fall back to grep across the full payload.
command=""
if command -v jq >/dev/null 2>&1; then
  # Try the common keys; ignore failures.
  command=$(echo "$input" | jq -r '.tool_input.command // .input.command // .arguments.command // ""' 2>/dev/null || true)
fi
if [ -z "$command" ]; then
  command="$input"
fi

# Patterns we refuse, in priority order. Each pattern has a label and a
# regex. Add new patterns by extending this list.
patterns=(
  "rm -rf /:^[[:space:]]*rm[[:space:]]+(-rf|-fr|-r[[:space:]]+-f|-f[[:space:]]+-r)[[:space:]]+/[[:space:]]*$"
  "rm -rf at root:^[[:space:]]*sudo[[:space:]]+.*rm[[:space:]]+-r[fF].*[[:space:]]+/[[:space:]]*$"
  "git reset --hard:git[[:space:]]+reset[[:space:]]+--hard"
  "git clean -fd (force-delete untracked):git[[:space:]]+clean[[:space:]]+-([a-z]*)f([a-z]*)d|git[[:space:]]+clean[[:space:]]+-([a-z]*)d([a-z]*)f"
  "git push --force to main/master:git[[:space:]]+push[[:space:]]+(-f|--force|--force-with-lease)?[[:space:]]+\\w+[[:space:]]+(main|master)"
  "rm of .env or credentials:rm[[:space:]]+(-[a-zA-Z]*[[:space:]]+)*[\"']*([^\"' ]*\\.env|[^\"' ]*credentials[^\"' ]*|[^\"' ]*\\.pem|[^\"' ]*id_(rsa|ed25519))"
  "shred / dd of disk:^[[:space:]]*(sudo[[:space:]]+)?(shred|dd)[[:space:]]+"
  ":(){ :|:& };:: \\:\\(\\)[[:space:]]*\\{[[:space:]]*:[[:space:]]*\\|[[:space:]]*:[[:space:]]*&[[:space:]]*\\}[[:space:]]*;[[:space:]]*:"
  "chmod 777 on home:chmod[[:space:]]+(-R[[:space:]]+)?777[[:space:]]+(\\$HOME|~|/Users/[^/]+|/home/[^/]+)"
  "find -delete on home:find[[:space:]]+(\\$HOME|~|/Users/[^/]+|/home/[^/]+).*-delete"
)

for entry in "${patterns[@]}"; do
  label="${entry%%:*}"
  regex="${entry#*:}"
  if echo "$command" | grep -E -q "$regex"; then
    cat <<EOF
{
  "continue": false,
  "stopReason": "dcg blocked: $label",
  "matched_command": $(echo "$command" | head -c 200 | jq -R -s . 2>/dev/null || printf '"%s"' "$command")
}
EOF
    exit 1
  fi
done

# Allow.
exit 0
