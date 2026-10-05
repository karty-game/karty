#!/usr/bin/env bash
set -euo pipefail
cli_root=$(pwd)
smoke_dir=$(mktemp -d)
trap 'rm -rf "$smoke_dir"' EXIT
sdk_version=${KARTY_TEST_SDK:-$("$cli_root/dist/karty" sdk current)}
cd "$smoke_dir"
"$cli_root/dist/karty" new --sdk "$sdk_version" pong
cd pong
native_args=()
web_args=()
if [[ -n "${KARTY_HOST_NATIVE:-}" ]]; then native_args=(--host "$KARTY_HOST_NATIVE"); fi
if [[ -n "${KARTY_HOST_WEB:-}" ]]; then web_args=(--host "$KARTY_HOST_WEB"); fi
"$cli_root/dist/karty" build --target native "${native_args[@]}"
runner=()
if command -v xvfb-run >/dev/null; then runner=(xvfb-run -a); fi
"${runner[@]}" ./dist/native/karty-host --check --cartridge dist/native/game.kart
"$cli_root/dist/karty" build --target web "${web_args[@]}"
