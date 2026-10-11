#!/usr/bin/env bash
set -euo pipefail

level_package=$(go list ./internal/levelbuild)
package_list=$(go list ./...)
unit_packages=()
while IFS= read -r package; do
  if [[ "$package" != "$level_package" ]]; then
    unit_packages+=("$package")
  fi
done <<< "$package_list"
go test -timeout=9s "${unit_packages[@]}"

# The level builder's cache/bake and real WASM cases have outgrown one shared
# nine-second package deadline on small CI runners. Discover every root test
# and run bounded batches, retaining the same deadline and all test coverage.
listing=$(go test -timeout=9s "$level_package" -list '^(Test|Example|Fuzz)')
level_tests=()
while IFS= read -r name; do
  if [[ "$name" =~ ^(Test|Example|Fuzz)[[:alnum:]_]+$ ]]; then
    level_tests+=("$name")
  fi
done <<< "$listing"
if (( ${#level_tests[@]} == 0 )); then
  printf 'No level builder tests discovered\n' >&2
  exit 1
fi

batch_size=8
for (( offset=0; offset<${#level_tests[@]}; offset+=batch_size )); do
  pattern=$(printf '%s|' "${level_tests[@]:offset:batch_size}")
  go test -timeout=9s "$level_package" -run "^(${pattern%|})$"
done
