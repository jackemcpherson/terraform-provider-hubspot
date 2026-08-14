#!/bin/sh
set -eu

root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM

new_fixture() {
  name=$1
  fixture="$tmp/$name"
  git clone --quiet --no-hardlinks "$root" "$fixture"
  cp "$root/scripts/release-preflight.sh" "$fixture/scripts/release-preflight.sh"
  git -C "$fixture" config user.name fixture
  git -C "$fixture" config user.email fixture@example.com
}

commit_fixture() {
  git -C "$fixture" add -A
  git -C "$fixture" commit --quiet --allow-empty -m fixture
  git -C "$fixture" rev-parse HEAD
}

expect_failure() {
  expected=$1
  commit=$2
  if GOCACHE="${GOCACHE:-$tmp/go-cache}" \
    "$fixture/scripts/release-preflight.sh" v0.4.0 "$commit" \
    >"$tmp/output" 2>&1; then
    echo "release preflight accepted $description" >&2
    exit 1
  fi
  grep -Fq "$expected" "$tmp/output" || {
    echo "$description did not report: $expected" >&2
    cat "$tmp/output" >&2
    exit 1
  }
}

new_fixture success
commit=$(commit_fixture)
GOCACHE="${GOCACHE:-$tmp/go-cache}" \
  "$fixture/scripts/release-preflight.sh" v0.4.0 "$commit"

new_fixture alternate-title-separator
sed '1s/^# v0\.4\.0:/# v0.4.0 -/' \
  "$fixture/docs/releases/v0.4.0.md" \
  >"$fixture/docs/releases/v0.4.0.md.tmp"
mv "$fixture/docs/releases/v0.4.0.md.tmp" \
  "$fixture/docs/releases/v0.4.0.md"
commit=$(commit_fixture)
GOCACHE="${GOCACHE:-$tmp/go-cache}" \
  "$fixture/scripts/release-preflight.sh" v0.4.0 "$commit"

new_fixture missing-notes
description='missing release notes'
rm "$fixture/docs/releases/v0.4.0.md"
commit=$(commit_fixture)
expect_failure 'release notes are missing for v0.4.0' "$commit"

new_fixture empty-notes
description='empty release notes'
: >"$fixture/docs/releases/v0.4.0.md"
commit=$(commit_fixture)
expect_failure 'release notes are missing for v0.4.0' "$commit"

new_fixture mismatched-notes
description='release notes for another version'
sed '1s/^# v0\.4\.0:/# v0.4.1:/' "$fixture/docs/releases/v0.4.0.md" \
  >"$fixture/docs/releases/v0.4.0.md.tmp"
mv "$fixture/docs/releases/v0.4.0.md.tmp" \
  "$fixture/docs/releases/v0.4.0.md"
commit=$(commit_fixture)
expect_failure 'release notes do not describe v0.4.0' "$commit"

new_fixture decoy-version-marker
description='release notes with a decoy version marker'
sed '1s/^# v0\.4\.0:/# v0.4.1:/' "$fixture/docs/releases/v0.4.0.md" \
  >"$fixture/docs/releases/v0.4.0.md.tmp"
mv "$fixture/docs/releases/v0.4.0.md.tmp" \
  "$fixture/docs/releases/v0.4.0.md"
printf '\nThis body mentions # v0.4.0: but describes another release.\n' \
  >>"$fixture/docs/releases/v0.4.0.md"
commit=$(commit_fixture)
expect_failure 'release notes do not describe v0.4.0' "$commit"

echo 'Release preflight tests passed'
