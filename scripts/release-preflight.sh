#!/bin/sh
set -eu

root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
version=${1:?version is required}
commit=${2:?commit is required}
"$root/scripts/validate-release-version.sh" "$version"
GOTOOLCHAIN=local go run "$root/cmd/validate-checkout" "$root" "$commit"
grep -q "^## \[$(printf '%s' "$version" | sed 's/^v//')\] - [0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]$" CHANGELOG.md || {
  echo "changelog has no dated section for $version" >&2
  exit 1
}
