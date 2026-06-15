#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
# optimize.sh — optimize the CONVERGED svg, once, after the loop.
#
# Usage: optimize.sh <input.svg> <output.svg>
#
# Runs svgo when available (direct binary, then `npx svgo`). Optimization is
# best-effort and version-agnostic: it uses only the universally-supported
# `--multipass` flag, then VERIFIES the result still has its viewBox and any
# <title> the input carried. If svgo is absent, errors, or strips a required
# attribute, fall back to the unoptimized input (correctness beats bytes).
# Never hard-fail, and never run mid-loop (optimizing between patches destroys
# structure the next patch needs).

set -uo pipefail

if [ "$#" -lt 2 ]; then
  echo "usage: optimize.sh <input.svg> <output.svg>" >&2
  exit 2
fi

in_svg="$1"
out_svg="$2"

if [ ! -f "$in_svg" ]; then
  echo "error: input SVG not found: $in_svg" >&2
  exit 2
fi

# What the input carries that optimization must not silently drop.
in_has_viewbox=$(grep -q 'viewBox' "$in_svg" && echo 1 || echo 0)
in_has_title=$(grep -q '<title' "$in_svg" && echo 1 || echo 0)

fallback() {
  echo "optimize: $1 — emitting unoptimized SVG (still valid)." >&2
  cp "$in_svg" "$out_svg"
  exit 0
}

# Pick an svgo invocation, if any.
svgo_cmd=""
if command -v svgo >/dev/null 2>&1; then
  svgo_cmd="svgo"
elif command -v npx >/dev/null 2>&1 && npx --no-install svgo --version >/dev/null 2>&1; then
  svgo_cmd="npx --no-install svgo"
fi

[ -z "$svgo_cmd" ] && fallback "svgo not found"

tmp_out="$(mktemp -t svgo-out.XXXXXX).svg"
trap 'rm -f "$tmp_out"' EXIT

if ! $svgo_cmd --multipass -i "$in_svg" -o "$tmp_out" >/dev/null 2>&1; then
  fallback "svgo exited non-zero"
fi
[ -s "$tmp_out" ] || fallback "svgo produced empty output"

# Verify svgo did not drop attributes the icon needs.
if [ "$in_has_viewbox" = 1 ] && ! grep -q 'viewBox' "$tmp_out"; then
  fallback "svgo stripped the viewBox"
fi
if [ "$in_has_title" = 1 ] && ! grep -q '<title' "$tmp_out"; then
  fallback "svgo stripped the <title>"
fi

cp "$tmp_out" "$out_svg"
exit 0
