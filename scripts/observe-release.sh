#!/bin/sh
set -eu

root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
version=${1:?v-prefixed release version is required}
commit=${2:?full release commit is required}
repository=${3:?GitHub repository is required}

printf '%s\n' "$version" | grep -Eq '^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$' || {
	echo 'version must be v-prefixed SemVer' >&2
	exit 1
}
printf '%s\n' "$commit" | grep -Eq '^[0-9a-f]{40}$' || {
	echo 'commit must be a full lowercase Git SHA' >&2
	exit 1
}
printf '%s\n' "$repository" | grep -Eq '^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$' || {
	echo 'repository must be an owner/name pair' >&2
	exit 1
}
: "${GH_TOKEN:?GH_TOKEN is required}"

cd "$root"
test "$(git rev-parse HEAD)" = "$commit" || {
	echo 'checked-out commit does not match the release commit' >&2
	exit 1
}

gh api "repos/$repository/commits/$commit/check-runs" \
	--jq 'any(.check_runs[]; .name == "Required" and .conclusion == "success")' | grep -Fxq true || {
	echo 'the release commit has no successful Required check' >&2
	exit 1
}

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM

tag_exists=false
release_exists=false
if git ls-remote --exit-code --tags origin "refs/tags/$version" >"$tmp/tag-ref" 2>/dev/null; then
	tag_exists=true
fi
if gh api "repos/$repository/releases/tags/$version" >"$tmp/release.json" 2>/dev/null; then
	release_exists=true
fi

if test "$tag_exists" != "$release_exists"; then
	echo 'immutable conflict: release tag and GitHub release must either both exist or both be absent' >&2
	exit 1
fi
if test "$tag_exists" = false; then
	printf '%s\n' new
	exit 0
fi

git fetch --quiet --force origin "refs/tags/$version:refs/tags/$version"
test "$(git rev-list -n 1 "$version")" = "$commit" || {
	echo 'immutable conflict: existing release tag targets a different commit' >&2
	exit 1
}
: "${GPG_PUBLIC_KEY:?GPG_PUBLIC_KEY is required to verify an existing release}"
mkdir "$tmp/assets"
gh release download "$version" --repo "$repository" --dir "$tmp/assets" >/dev/null
"$root/scripts/verify-release-assets.sh" "$tmp/assets" "$GPG_PUBLIC_KEY"

if jq -e '.draft == true' "$tmp/release.json" >/dev/null; then
	printf '%s\n' draft
else
	printf '%s\n' published
fi
