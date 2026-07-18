#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM

first="$tmp/first"
second="$tmp/second"
mkdir -p "$first" "$second"

write_sbom() {
	destination=$1
	namespace=$2
	created=$3
	version=$4
	printf '%s\n' "{\"spdxVersion\":\"SPDX-2.3\",\"documentNamespace\":\"$namespace\",\"creationInfo\":{\"created\":\"$created\",\"creators\":[\"Tool: syft-1.33.0\"]},\"packages\":[{\"name\":\"provider\",\"versionInfo\":\"$version\"}]}" >"$destination"
}

write_checksums() {
	directory=$1
	(
		cd "$directory"
		shasum -a 256 \
			terraform-provider-hubspot_0.1.0_linux_amd64.zip \
			terraform-provider-hubspot_0.1.0_linux_amd64.zip.spdx.sbom \
			terraform-registry-manifest.json \
			>terraform-provider-hubspot_0.1.0_SHA256SUMS
	)
}

expect_failure() {
	description=$1
	if "$root/scripts/compare-release-builds.sh" "$first" "$second" >"$tmp/failure-output" 2>&1; then
		echo "expected comparison failure: $description" >&2
		exit 1
	fi
}

printf '%s\n' 'provider archive' >"$first/terraform-provider-hubspot_0.1.0_linux_amd64.zip"
printf '%s\n' '{"format_version":1,"protocol_versions":["6.0"]}' >"$first/terraform-registry-manifest.json"
write_sbom "$first/terraform-provider-hubspot_0.1.0_linux_amd64.zip.spdx.sbom" \
	'https://spdx.org/spdxdocs/provider-first' '2026-07-18T00:00:00Z' '0.1.0'

cp "$first/terraform-provider-hubspot_0.1.0_linux_amd64.zip" "$second/"
cp "$first/terraform-registry-manifest.json" "$second/"
write_sbom "$second/terraform-provider-hubspot_0.1.0_linux_amd64.zip.spdx.sbom" \
	'https://spdx.org/spdxdocs/provider-second' '2026-07-18T00:01:00Z' '0.1.0'
write_checksums "$first"
write_checksums "$second"

"$root/scripts/compare-release-builds.sh" "$first" "$second"

write_sbom "$second/terraform-provider-hubspot_0.1.0_linux_amd64.zip.spdx.sbom" \
	'https://spdx.org/spdxdocs/provider-second' '2026-07-18T00:01:00Z' '0.2.0'
write_checksums "$second"
expect_failure 'SBOM inventory changed'

write_sbom "$second/terraform-provider-hubspot_0.1.0_linux_amd64.zip.spdx.sbom" \
	'https://spdx.org/spdxdocs/provider-second' '2026-07-18T00:01:00Z' '0.1.0'
write_checksums "$second"
printf '%s\n' 'changed provider archive' >"$second/terraform-provider-hubspot_0.1.0_linux_amd64.zip"
expect_failure 'release archive changed'

cp "$first/terraform-provider-hubspot_0.1.0_linux_amd64.zip" "$second/"
write_checksums "$second"
printf '%s  %s\n' \
	'ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff' \
	'terraform-provider-hubspot_0.1.0_linux_amd64.zip' \
	>>"$second/terraform-provider-hubspot_0.1.0_SHA256SUMS"
expect_failure 'non-SBOM checksum inventory changed'

echo 'release build comparison tests passed'
