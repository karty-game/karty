#!/usr/bin/env bash
set -euo pipefail

if [[ ! "${RELEASE_TAG:-}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  printf 'Release tag must be vMAJOR.MINOR.PATCH\n' >&2
  exit 1
fi

commit=$(git rev-parse --verify "refs/tags/${RELEASE_TAG}^{commit}")
printf 'tag=%s\ncommit=%s\n' "$RELEASE_TAG" "$commit" >> "$GITHUB_OUTPUT"
