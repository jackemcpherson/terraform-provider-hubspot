#!/bin/sh
set -eu

root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
dir=${1:?asset directory is required}
public_key=${2:?armored public key is required}
checksum=$(find "$dir" -name '*_SHA256SUMS' -type f -print -quit)
test -n "$checksum"
test -f "$checksum.sig"
printf '%s' "$public_key" | gpg --batch --import
gpg --batch --verify "$checksum.sig" "$checksum"
"$root/scripts/verify-registry-checksums.sh" "$dir"
find "$dir" -name '*.spdx.sbom' -type f -print -quit | grep -q .
checksum_name=$(basename "$checksum")
release_prefix=${checksum_name%_SHA256SUMS}
manifest="$dir/${release_prefix}_manifest.json"
test -f "$manifest"
find "$dir" -name '*.zip' -type f -print -quit | grep -q .
grep -q '"protocol_versions": \["6.0"\]' "$manifest"
