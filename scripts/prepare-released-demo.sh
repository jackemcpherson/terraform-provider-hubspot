#!/bin/sh
set -eu

root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
version=${1:?release version is required}
demo_repo=${2:?exact demo repository is required}
demo_commit=${3:?exact demo commit is required}
destination=${4:?prepared demo destination is required}
assets=${5:?GitHub release assets directory is required}

test "$version" = v0.4.0 || { echo "released demo preparation requires v0.4.0" >&2; exit 1; }
test ! -e "$destination" || { echo "prepared demo destination already exists" >&2; exit 1; }
test -d "$assets" || { echo "GitHub release assets directory is missing" >&2; exit 1; }
GOTOOLCHAIN=local go run "$root/cmd/validate-checkout" "$demo_repo" "$demo_commit"

mkdir "$destination"
git -C "$demo_repo" archive "$demo_commit" | tar -x -C "$destination"
test -x "$destination/scripts/demo" || { echo "exact demo archive omitted its lifecycle script" >&2; exit 1; }

# Resolve new locks from the published registries instead of trusting candidate locks.
rm -f "$destination/locks/terraform/.terraform.lock.hcl" "$destination/locks/tofu/.terraform.lock.hcl"

HUBSPOT_REQUIRE_CLEAN_PROVENANCE=0 ENGINE=terraform "$destination/scripts/demo" registry init >/dev/null
HUBSPOT_REQUIRE_CLEAN_PROVENANCE=0 ENGINE=tofu "$destination/scripts/demo" registry init >/dev/null

release_version=${version#v}
os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$(uname -m)" in arm64|aarch64) arch=arm64 ;; x86_64|amd64) arch=amd64 ;; *) echo "unsupported prepared demo architecture" >&2; exit 1 ;; esac
archive=$(find "$assets" -type f -name "terraform-provider-hubspot_${release_version}_${os}_${arch}.zip" -print -quit)
test -n "$archive" || { echo "prepared demo release archive is missing" >&2; exit 1; }
archive_digest=$(shasum -a 256 "$archive" | awk '{print $1}')
checksum_inventory="$assets/terraform-provider-hubspot_${release_version}_SHA256SUMS"
test -f "$checksum_inventory" || { echo "prepared demo checksum inventory is missing" >&2; exit 1; }
awk -v digest="$archive_digest" -v archive_name="$(basename "$archive")" \
  '$1 == digest && $2 == archive_name { matches++ } END { exit matches == 1 ? 0 : 1 }' \
  "$checksum_inventory" || { echo "prepared demo checksum inventory does not bind the selected archive" >&2; exit 1; }
for engine in terraform tofu; do
  lock="$destination/locks/$engine/.terraform.lock.hcl"
  case "$engine" in
    terraform) address=registry.terraform.io/jackemcpherson/hubspot ;;
    tofu) address=registry.opentofu.org/jackemcpherson/hubspot ;;
  esac
  test -f "$lock" || { echo "prepared $engine Registry lock is missing" >&2; exit 1; }
  grep -Fq "provider \"$address\"" "$lock" || { echo "prepared $engine Registry lock selected the wrong source" >&2; exit 1; }
  grep -Eq "version[[:space:]]*=[[:space:]]*\"$release_version\"" "$lock" || { echo "prepared $engine Registry lock selected the wrong version" >&2; exit 1; }
  grep -Fq "zh:$archive_digest" "$lock" || { echo "prepared $engine Registry lock digest does not match the GitHub release" >&2; exit 1; }
done

printf '%s\n' "$destination/scripts/demo"
