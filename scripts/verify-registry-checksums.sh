#!/bin/sh
set -eu

directory=${1:?release asset directory is required}
root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM

find "$directory" -maxdepth 1 -name '*_SHA256SUMS' -type f -print >"$tmp/checksum-files"
if test "$(wc -l <"$tmp/checksum-files" | tr -d ' ')" -ne 1; then
	echo 'release assets must contain exactly one checksum file' >&2
	exit 1
fi
checksum=$(sed -n '1p' "$tmp/checksum-files")
checksum_name=$(basename "$checksum")
case "$checksum_name" in
terraform-provider-hubspot_*_SHA256SUMS)
	release_prefix=${checksum_name%_SHA256SUMS}
	;;
*)
	echo 'checksum filename does not identify a HubSpot provider release' >&2
	exit 1
	;;
esac
manifest_name=${release_prefix}_manifest.json

(
	for platform in \
		darwin_amd64 \
		darwin_arm64 \
		freebsd_386 \
		freebsd_amd64 \
		freebsd_arm \
		freebsd_arm64 \
		linux_386 \
		linux_amd64 \
		linux_arm \
		linux_arm64 \
		windows_386 \
		windows_amd64 \
		windows_arm64
	do
		printf '%s_%s.zip\n' "$release_prefix" "$platform"
	done
	printf '%s\n' "$manifest_name"
) | LC_ALL=C sort >"$tmp/expected"

(
	cd "$directory"
	find . -maxdepth 1 -type f \( -name '*.zip' -o -name "$manifest_name" \) -print |
		sed 's|^[.]/||' | LC_ALL=C sort
) >"$tmp/package-assets"
if ! diff -u "$tmp/expected" "$tmp/package-assets"; then
	echo 'release package assets do not match the supported platform set' >&2
	exit 1
fi

if ! awk '
	NF != 2 || length($1) != 64 || $1 !~ /^[0-9a-f]+$/ { invalid=1; next }
	{ print $2 }
	END { if (invalid) exit 1 }
' "$checksum" | LC_ALL=C sort >"$tmp/actual"; then
	echo 'checksum file contains an invalid entry' >&2
	exit 1
fi

if ! diff -u "$tmp/expected" "$tmp/actual"; then
	echo 'checksum inventory contains files outside the Terraform Registry contract' >&2
	exit 1
fi

(cd "$directory" && shasum -a 256 -c "$(basename "$checksum")")
"$root/scripts/verify-registry-manifest.sh" "$directory/$manifest_name"
