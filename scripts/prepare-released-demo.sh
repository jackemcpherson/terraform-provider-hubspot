#!/bin/sh
set -eu

root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
version=${1:?release version is required}
demo_repo=${2:?exact demo repository is required}
demo_commit=${3:?exact demo commit is required}
destination=${4:?prepared demo destination is required}

test "$version" = v0.4.0 || { echo "released demo preparation requires v0.4.0" >&2; exit 1; }
test ! -e "$destination" || { echo "prepared demo destination already exists" >&2; exit 1; }
GOTOOLCHAIN=local go run "$root/cmd/validate-checkout" "$demo_repo" "$demo_commit"

mkdir "$destination"
git -C "$demo_repo" archive "$demo_commit" | tar -x -C "$destination"
test -x "$destination/scripts/demo" || { echo "exact demo archive omitted its lifecycle script" >&2; exit 1; }

# Resolve new locks from the published registries instead of trusting candidate locks.
rm -f "$destination/locks/terraform/.terraform.lock.hcl" "$destination/locks/tofu/.terraform.lock.hcl"

HUBSPOT_REQUIRE_CLEAN_PROVENANCE=0 ENGINE=terraform "$destination/scripts/demo" registry init >/dev/null
HUBSPOT_REQUIRE_CLEAN_PROVENANCE=0 ENGINE=tofu "$destination/scripts/demo" registry init >/dev/null

release_version=${version#v}
for engine in terraform tofu; do
  lock="$destination/locks/$engine/.terraform.lock.hcl"
  test -f "$lock" || { echo "prepared $engine Registry lock is missing" >&2; exit 1; }
  grep -Eq "version[[:space:]]*=[[:space:]]*\"$release_version\"" "$lock" || { echo "prepared $engine Registry lock selected the wrong version" >&2; exit 1; }
  grep -Eq '"(h1:|zh:)' "$lock" || { echo "prepared $engine Registry lock omitted package hashes" >&2; exit 1; }
done

printf '%s\n' "$destination/scripts/demo"
