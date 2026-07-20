#!/bin/sh
set -eu

root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
version=${1:?version is required}
commit=${2:?commit is required}
"$root/scripts/validate-release-version.sh" "$version"
test "$(git rev-parse "$commit^{commit}")" = "$commit" || { echo "commit must be a full commit SHA" >&2; exit 1; }
test -z "$(git status --porcelain)" || { echo "release worktree is not clean" >&2; exit 1; }
grep -q "^## \[$(printf '%s' "$version" | sed 's/^v//')\] - [0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]$" CHANGELOG.md || {
  echo "changelog has no dated section for $version" >&2
  exit 1
}
if git tag --list "$version" | grep -q .; then
  echo "tag already exists" >&2
  exit 1
fi
