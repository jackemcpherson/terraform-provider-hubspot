#!/bin/sh
set -eu

root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
version=${1:?v-prefixed release version is required}
candidate_commit=${2:?full candidate commit is required}
repository=${3:?GitHub repository is required}

"$root/scripts/validate-release-version.sh" "$version"
printf '%s\n' "$candidate_commit" | grep -Eq '^[0-9a-f]{40}$' || {
	echo 'commit must be a full lowercase Git SHA' >&2
	exit 1
}
printf '%s\n' "$repository" | grep -Eq '^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$' || {
	echo 'repository must be an owner/name pair' >&2
	exit 1
}
: "${GH_TOKEN:?GH_TOKEN is required}"

cd "$root"
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
	test "$(git rev-parse HEAD)" = "$candidate_commit" || {
		echo 'checked-out commit does not match the new release commit' >&2
		exit 1
	}
	release_commit=$candidate_commit
else
	git fetch --quiet --force origin "refs/tags/$version:refs/tags/$version"
	release_commit=$(git rev-list -n 1 "$version")
	printf '%s\n' "$release_commit" | grep -Eq '^[0-9a-f]{40}$' || {
		echo 'existing release tag does not resolve to a commit' >&2
		exit 1
	}
	git merge-base --is-ancestor "$release_commit" refs/remotes/origin/main || {
		echo 'immutable conflict: existing release commit is not on main' >&2
		exit 1
	}
fi

gh api "repos/$repository/commits/$release_commit/check-runs" \
	--jq 'any(.check_runs[]; .name == "Required" and .conclusion == "success")' | grep -Fxq true || {
	echo 'the release commit has no successful Required check' >&2
	exit 1
}

if test "$tag_exists" = false; then
	printf 'new %s\n' "$release_commit"
	exit 0
fi

: "${GPG_PUBLIC_KEY:?GPG_PUBLIC_KEY is required to verify an existing release}"
printf '%s' "$GPG_PUBLIC_KEY" | gpg --batch --import >/dev/null 2>&1
git verify-tag "$version" >/dev/null
mkdir "$tmp/assets"
gh release download "$version" --repo "$repository" --dir "$tmp/assets" >/dev/null
if test -n "${RELEASE_ASSET_VERIFIER:-}"; then
	"$RELEASE_ASSET_VERIFIER" "$tmp/assets" "$GPG_PUBLIC_KEY" "$version" "$repository"
else
	"$root/scripts/verify-release-assets.sh" "$tmp/assets" "$GPG_PUBLIC_KEY"
	"$root/scripts/verify-release-bundle.sh" "$tmp/assets" "$version"
	for archive in "$tmp/assets"/*.zip; do
		gh attestation verify "$archive" --repo "$repository" >/dev/null
	done
fi

if jq -e '.draft == true' "$tmp/release.json" >/dev/null; then
	printf 'draft %s\n' "$release_commit"
else
	printf 'published %s\n' "$release_commit"
fi
