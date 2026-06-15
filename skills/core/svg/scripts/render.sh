#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
# render.sh — rasterize an SVG to PNG so the agent can see its own output.
#
# Usage: render.sh <input.svg> <output.png> [size_px]
#
# Probes for a rasterizer in preference order and uses the first available.
# Prints the chosen engine family to stderr (the SKILL.md uses this for
# engine-match awareness: an approved render only predicts the shipped
# browser when the in-loop engine matches the target family).
#
# Degrade contract (never hard-fail the skill): if NO rasterizer is found,
# print RASTERIZER_ABSENT to stderr and exit 3. The SKILL.md reads exit 3 as
# "skip the visual pass, emit style-system output, tell the user" — not as an
# error. Real failures (bad args, a rasterizer that errored) exit 1/2.

set -euo pipefail

if [ "$#" -lt 2 ]; then
  echo "usage: render.sh <input.svg> <output.png> [size_px]" >&2
  exit 2
fi

in_svg="$1"
out_png="$2"
size="${3:-256}"

if [ ! -f "$in_svg" ]; then
  echo "error: input SVG not found: $in_svg" >&2
  exit 2
fi

# Probe order: resvg (single static binary, best fidelity, no node) first,
# then librsvg, then a headless browser (best CSS/filter fidelity, heaviest),
# then cairosvg (weakest filter support). Engine family is reported so the
# caller can match it to the SVG's shipping target.
if command -v resvg >/dev/null 2>&1; then
  echo "engine=resvg" >&2
  resvg --width "$size" "$in_svg" "$out_png"
  exit 0
fi

if command -v rsvg-convert >/dev/null 2>&1; then
  echo "engine=librsvg" >&2
  rsvg-convert -w "$size" -h "$size" -o "$out_png" "$in_svg"
  exit 0
fi

# chromium is a best-effort fallback: it screenshots the SVG file sized by the
# window, so a 24-viewBox SVG with no intrinsic width/height won't scale to fill
# the frame. Prefer resvg/librsvg/cairosvg above; use chromium only when it's all
# that's available, and treat its render as approximate.
for chrome in chromium chromium-browser "google-chrome" "google-chrome-stable"; do
  if command -v "$chrome" >/dev/null 2>&1; then
    echo "engine=chromium" >&2
    abs_svg="$(cd "$(dirname "$in_svg")" && pwd)/$(basename "$in_svg")"
    "$chrome" --headless --disable-gpu --force-device-scale-factor=1 \
      --screenshot="$out_png" --window-size="${size},${size}" \
      --default-background-color=00000000 "file://${abs_svg}"
    exit 0
  fi
done

if command -v cairosvg >/dev/null 2>&1; then
  echo "engine=cairosvg" >&2
  cairosvg "$in_svg" -o "$out_png" --output-width "$size" --output-height "$size"
  exit 0
fi

# No rasterizer available — signal the degrade path, do not error.
echo "RASTERIZER_ABSENT" >&2
echo "No rasterizer found (looked for resvg, rsvg-convert, chromium, cairosvg)." >&2
echo "The svg skill will skip the visual-verification loop and emit style-system output." >&2
exit 3
