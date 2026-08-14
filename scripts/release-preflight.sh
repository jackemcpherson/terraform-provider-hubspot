#!/bin/sh
set -eu

root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
version=${1:?version is required}
commit=${2:?commit is required}
"$root/scripts/validate-release-version.sh" "$version"
(cd "$root" && GOTOOLCHAIN=local go run ./cmd/validate-checkout "$root" "$commit")
grep -q "^## \[$(printf '%s' "$version" | sed 's/^v//')\] - [0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]$" "$root/CHANGELOG.md" || {
  echo "changelog has no dated section for $version" >&2
  exit 1
}
release_notes="$root/docs/releases/$version.md"
test -s "$release_notes" || {
  echo "release notes are missing for $version" >&2
  exit 1
}
grep -Fq "# $version:" "$release_notes" || {
  echo "release notes do not describe $version" >&2
  exit 1
}
