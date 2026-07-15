#!/bin/sh
set -eu

version=${1:?version is required}
commit=${2:?commit is required}
command -v jq >/dev/null 2>&1 || { echo "jq is required" >&2; exit 1; }
printf '%s\n' "$version" | grep -Eq '^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$' || { echo "version must be v-prefixed SemVer" >&2; exit 1; }
test "$(jq -r '.channel' release/surface.json)" = free-alpha || { echo "release surface is not the Free alpha" >&2; exit 1; }
test "$version" = v0.1.0-alpha.1 || { echo "this branch releases v0.1.0-alpha.1 only" >&2; exit 1; }
test "$(git rev-parse "$commit^{commit}")" = "$commit" || { echo "commit must be a full commit SHA" >&2; exit 1; }
test -z "$(git status --porcelain)" || { echo "release worktree is not clean" >&2; exit 1; }
grep -q "^## \[$(printf '%s' "$version" | sed 's/^v//')\] - [0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]$" CHANGELOG.md || {
  echo "changelog has no dated section for $version" >&2
  exit 1
}
git tag --list "$version" | grep -q . && { echo "tag already exists" >&2; exit 1; }
