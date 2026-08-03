#!/bin/sh
set -eu

root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
dir=${1:?asset directory is required}
public_key=${2:?armored public key is required}
registered_fingerprint=${3:?registered signing fingerprint is required}
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
checksum=$(find "$dir" -name '*_SHA256SUMS' -type f -print -quit)
test -n "$checksum"
test -f "$checksum.sig"
if grep -q '^-----BEGIN PGP SIGNATURE-----$' "$checksum.sig"; then
	echo 'Registry checksum signature must be binary, not ASCII-armored' >&2
	exit 1
fi
printf '%s' "$public_key" | gpg --batch --import
if ! gpg --batch --status-fd 1 --verify "$checksum.sig" "$checksum" >"$tmp/checksum-signature-status"; then
	echo 'Registry checksum signature verification failed' >&2
	exit 1
fi
"$root/scripts/verify-gpg-signing-identity.sh" "$registered_fingerprint" "$tmp/checksum-signature-status" 'Registry checksum'
"$root/scripts/verify-registry-checksums.sh" "$dir"
checksum_name=$(basename "$checksum")
release_prefix=${checksum_name%_SHA256SUMS}
manifest="$dir/${release_prefix}_manifest.json"
test -f "$manifest"
find "$dir" -name '*.zip' -type f -print -quit | grep -q .
"$root/scripts/verify-registry-manifest.sh" "$manifest"
