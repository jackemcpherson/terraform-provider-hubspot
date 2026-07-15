#!/bin/sh
set -eu

dir=${1:?asset directory is required}
public_key=${2:?armored public key is required}
checksum=$(find "$dir" -name '*_SHA256SUMS' -type f -print -quit)
test -n "$checksum"
test -f "$checksum.sig"
printf '%s' "$public_key" | gpg --batch --import
gpg --batch --verify "$checksum.sig" "$checksum"
(cd "$dir" && shasum -a 256 -c "$(basename "$checksum")")
find "$dir" -name '*.spdx.sbom' -type f -print -quit | grep -q .
test -f "$dir/terraform-registry-manifest.json"
find "$dir" -name '*.zip' -type f -print -quit | grep -q .
grep -q '"protocol_versions": \["6.0"\]' "$dir/terraform-registry-manifest.json"
