#!/bin/sh
set -eu

root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM

assets="$tmp/assets"
mkdir -p "$assets"

platforms='darwin_amd64 darwin_arm64 freebsd_386 freebsd_amd64 freebsd_arm64 freebsd_armv6 freebsd_armv7 linux_386 linux_amd64 linux_arm64 linux_armv6 linux_armv7 windows_386 windows_amd64 windows_arm64'
for platform in $platforms; do
	printf '%s\n' "provider archive $platform" >"$assets/terraform-provider-hubspot_0.1.1_${platform}.zip"
done
printf '%s\n' '{"format_version":1,"protocol_versions":["6.0"]}' >"$assets/terraform-registry-manifest.json"
printf '%s\n' '{"spdxVersion":"SPDX-2.3"}' >"$assets/terraform-provider-hubspot_0.1.1_linux_amd64.zip.spdx.sbom"

(
	cd "$assets"
	shasum -a 256 \
		terraform-provider-hubspot_0.1.1_*.zip \
		terraform-provider-hubspot_0.1.1_linux_amd64.zip.spdx.sbom \
		terraform-registry-manifest.json \
		>terraform-provider-hubspot_0.1.1_SHA256SUMS
)

if "$root/scripts/verify-registry-checksums.sh" "$assets" >"$tmp/failure-output" 2>&1; then
	echo 'expected Registry checksum contract to reject an SBOM entry' >&2
	exit 1
fi

grep -q 'checksum inventory contains files outside the Terraform Registry contract' "$tmp/failure-output"

(
	cd "$assets"
	shasum -a 256 \
		terraform-provider-hubspot_0.1.1_*.zip \
		terraform-registry-manifest.json \
		>terraform-provider-hubspot_0.1.1_SHA256SUMS
)

"$root/scripts/verify-registry-checksums.sh" "$assets"

rm "$assets/terraform-provider-hubspot_0.1.1_windows_arm64.zip"
(
	cd "$assets"
	shasum -a 256 \
		terraform-provider-hubspot_0.1.1_*.zip \
		terraform-registry-manifest.json \
		>terraform-provider-hubspot_0.1.1_SHA256SUMS
)
if "$root/scripts/verify-registry-checksums.sh" "$assets" >"$tmp/missing-output" 2>&1; then
	echo 'expected Registry checksum contract to reject a missing platform archive' >&2
	exit 1
fi
grep -q 'release package assets do not match the supported platform set' "$tmp/missing-output"

printf '%s\n' 'provider archive windows_arm64' >"$assets/terraform-provider-hubspot_0.1.1_windows_arm64.zip"
printf '%s\n' 'unsupported archive' >"$assets/terraform-provider-hubspot_0.1.1_solaris_amd64.zip"
(
	cd "$assets"
	shasum -a 256 \
		terraform-provider-hubspot_0.1.1_*.zip \
		terraform-registry-manifest.json \
		>terraform-provider-hubspot_0.1.1_SHA256SUMS
)
if "$root/scripts/verify-registry-checksums.sh" "$assets" >"$tmp/extra-output" 2>&1; then
	echo 'expected Registry checksum contract to reject an unsupported platform archive' >&2
	exit 1
fi
grep -q 'release package assets do not match the supported platform set' "$tmp/extra-output"

echo 'Registry checksum contract tests passed'
