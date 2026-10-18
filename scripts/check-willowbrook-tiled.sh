#!/usr/bin/env bash
set -euo pipefail

: "${KARTY_TILED_BIN:?Set KARTY_TILED_BIN to the Tiled 1.11.2 executable}"
: "${KARTY_TILED_RASTERIZER:?Set KARTY_TILED_RASTERIZER to its tmxrasterizer executable}"

map=samples/willowbrook/internal/village/village.tmj
tileset=samples/willowbrook/internal/village/village.tsj
output=dist/validation/willowbrook
mkdir -p "$output"
before=$(sha256sum "$map" "$tileset")
"$KARTY_TILED_BIN" --version
"$KARTY_TILED_BIN" --export-map json "$map" "$output/tiled-export.tmj"
"$KARTY_TILED_RASTERIZER" --no-smoothing "$map" "$output/tiled-map.png"
after=$(sha256sum "$map" "$tileset")
test "$before" = "$after"
printf '%s\n' "$after"
