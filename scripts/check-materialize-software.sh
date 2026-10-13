#!/usr/bin/env bash
# Keep downloads and first-use driver setup outside the native test's deadline.
set -euo pipefail

export KARTY_HOME="${KARTY_HOME:-$PWD/dist/validation/materialize/karty}"
export XDG_CACHE_HOME="${XDG_CACHE_HOME:-$PWD/dist/validation/materialize/cache}"
mkdir -p "$KARTY_HOME" "$XDG_CACHE_HOME"

go run ./cmd/karty-test-sdk
go run ./cmd/karty-materialize-check

# Preparation uses its own discarded project cache. This fixture still checks a
# fresh atlas cache followed by reuse, using the same verified tool/driver cache.
go test -timeout=9s ./internal/assetpipeline -run '^TestWorldMaterialReleaseAtlas$' -count=1 -v
