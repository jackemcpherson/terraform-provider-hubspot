#!/bin/sh
set -eu

root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
candidate=1111111111111111111111111111111111111111
tag_commit=2222222222222222222222222222222222222222

# These generated commands isolate the release-state policy from GitHub and Git.
# shellcheck disable=SC2016
printf '%s\n' '#!/bin/sh' '
case "$1 $2" in
  "rev-parse HEAD") printf "%s\n" "$CANDIDATE_COMMIT" ;;
  "ls-remote --exit-code") test "$TAG_EXISTS" = true && printf "%s\trefs/tags/%s\n" "$TAG_COMMIT" "$VERSION" ;;
  "fetch --quiet") ;;
  "rev-list -n") printf "%s\n" "$TAG_COMMIT" ;;
  "merge-base --is-ancestor") test "${TAG_ON_MAIN:-true}" = true ;;
  "verify-tag v1.2.3") test "${TAG_SIGNED:-true}" = true ;;
  *) echo "unexpected git call: $*" >&2; exit 1 ;;
esac' >"$tmp/git"
# shellcheck disable=SC2016
printf '%s\n' '#!/bin/sh' '
if test "$1" = api && printf "%s" "$2" | grep -q /check-runs; then
  test "${REQUIRED_RESULT:-success}" = success && printf "true\n" || printf "false\n"
elif test "$1" = api; then
  test "$RELEASE_STATE" != none || exit 1
  test "$RELEASE_STATE" = draft && printf "{\"draft\":true}\n" || printf "{\"draft\":false}\n"
elif test "$1 $2" = "release download"; then
  while test "$#" -gt 0; do
    if test "$1" = --dir; then shift; mkdir -p "$1"; : >"$1/provider.zip"; break; fi
    shift
  done
else
  echo "unexpected gh call: $*" >&2; exit 1
fi' >"$tmp/gh"
# shellcheck disable=SC2016
printf '%s\n' '#!/bin/sh' 'test "$3" = v1.2.3; test "$4" = owner/repository; printf "verified\n" >>"$VERIFY_LOG"' >"$tmp/verifier"
# shellcheck disable=SC2016
printf '%s\n' '#!/bin/sh' 'exit 0' >"$tmp/gpg"
chmod +x "$tmp/git" "$tmp/gh" "$tmp/verifier" "$tmp/gpg"

observe() {
	PATH="$tmp:$PATH" GH_TOKEN=test GPG_PUBLIC_KEY=test \
		CANDIDATE_COMMIT="$candidate" TAG_COMMIT="$tag_commit" VERSION=v1.2.3 \
		TAG_EXISTS="$1" RELEASE_STATE="$2" VERIFY_LOG="$tmp/verified" \
		RELEASE_ASSET_VERIFIER="$tmp/verifier" \
		"$root/scripts/observe-release.sh" v1.2.3 "$candidate" owner/repository
}

test "$(observe false none)" = "new $candidate"
test "$(observe true draft)" = "draft $tag_commit"
test "$(observe true published)" = "published $tag_commit"
test "$(wc -l <"$tmp/verified" | tr -d ' ')" = 2

if observe true none; then
	echo 'tag without release must fail closed' >&2
	exit 1
fi
if observe false draft; then
	echo 'release without tag must fail closed' >&2
	exit 1
fi
if TAG_SIGNED=false observe true draft; then
	echo 'unsigned tag must fail closed' >&2
	exit 1
fi
if REQUIRED_RESULT=failed observe false none; then
	echo 'failed Required check must fail closed' >&2
	exit 1
fi
if TAG_ON_MAIN=false observe true published; then
	echo 'release commit outside main must fail closed' >&2
	exit 1
fi
