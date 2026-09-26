#!/usr/bin/env bash
# Customer-facing checks consume the selected public SDK and host artifacts.
set -euo pipefail
mise run smoke
mise run check-web
mise run check-browser
mise run check-ui
mise run check-guest-allocations
mise run check-dev
