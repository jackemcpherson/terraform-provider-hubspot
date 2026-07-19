#!/bin/sh
set -eu

first=${1:?first build directory is required}
second=${2:?second build directory is required}
root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)

command -v jq >/dev/null 2>&1 || {
	echo "jq is required to compare SPDX SBOMs" >&2
	exit 1
}

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM

list_files() {
	directory=$1
	kind=$2
	output=$3

	case "$kind" in
	release)
		(cd "$directory" && find . -type f \( -name '*.zip' -o -name '*_manifest.json' \) -print | LC_ALL=C sort) >"$output"
		;;
	checksums)
		(cd "$directory" && find . -type f -name '*_SHA256SUMS' -print | LC_ALL=C sort) >"$output"
		;;
	sboms)
		(cd "$directory" && find . -type f -name '*.spdx.sbom' -print | LC_ALL=C sort) >"$output"
		;;
	*)
		echo "unknown artifact kind: $kind" >&2
		exit 1
		;;
	esac
}

compare_file_lists() {
	kind=$1
	first_list=$2
	second_list=$3

	if ! diff -u "$first_list" "$second_list"; then
		echo "$kind file lists differ" >&2
		exit 1
	fi
}

require_expected_files() {
	kind=$1
	file_list=$2

	if ! test -s "$file_list"; then
		echo "release build contains no $kind files" >&2
		exit 1
	fi

	if test "$kind" = release; then
		grep -q '[.]zip$' "$file_list" || {
			echo "release build contains no provider archives" >&2
			exit 1
		}
		grep -q '/terraform-provider-hubspot_.*_manifest[.]json$' "$file_list" || {
			echo "release build contains no registry manifest" >&2
			exit 1
		}
	fi
}

hash_release_files() {
	directory=$1
	file_list=$2
	output=$3

	: >"$output"
	while IFS= read -r artifact; do
		hash=$(shasum -a 256 "$directory/$artifact" | awk '{print $1}')
		printf '%s  %s\n' "$hash" "$artifact" >>"$output"
	done <"$file_list"
}

hash_checksum_inventory() {
	directory=$1
	file_list=$2
	output=$3

	: >"$output"
	while IFS= read -r artifact; do
		hash=$(shasum -a 256 "$directory/$artifact" | awk '{print $1}')
		printf '%s  %s\n' "$hash" "$artifact" >>"$output"
	done <"$file_list"
}

"$root/scripts/verify-registry-checksums.sh" "$first"
"$root/scripts/verify-registry-checksums.sh" "$second"

hash_normalized_sboms() {
	directory=$1
	file_list=$2
	output=$3
	prefix=$4
	index=0

	: >"$output"
	while IFS= read -r artifact; do
		index=$((index + 1))
		normalized="$tmp/$prefix-sbom-$index.json"
		jq -S 'del(.documentNamespace, .creationInfo.created)' "$directory/$artifact" >"$normalized"
		hash=$(shasum -a 256 "$normalized" | awk '{print $1}')
		printf '%s  %s\n' "$hash" "$artifact" >>"$output"
	done <"$file_list"
}

compare_hashes() {
	kind=$1
	first_hashes=$2
	second_hashes=$3

	if ! diff -u "$first_hashes" "$second_hashes"; then
		echo "$kind differ between release builds" >&2
		exit 1
	fi
}

for kind in release checksums sboms; do
	list_files "$first" "$kind" "$tmp/first-$kind-files"
	list_files "$second" "$kind" "$tmp/second-$kind-files"
	require_expected_files "$kind" "$tmp/first-$kind-files"
	require_expected_files "$kind" "$tmp/second-$kind-files"
	compare_file_lists "$kind" "$tmp/first-$kind-files" "$tmp/second-$kind-files"
done

hash_release_files "$first" "$tmp/first-release-files" "$tmp/first-release-hashes"
hash_release_files "$second" "$tmp/second-release-files" "$tmp/second-release-hashes"
compare_hashes "release archives or manifest" "$tmp/first-release-hashes" "$tmp/second-release-hashes"

# Terraform Registry checksums cover only archives and the manifest, so they are
# reproducible byte-for-byte. Standalone SBOM documents remain release assets and
# are compared separately after removing only Syft's volatile document metadata.
hash_checksum_inventory "$first" "$tmp/first-checksums-files" "$tmp/first-checksum-hashes"
hash_checksum_inventory "$second" "$tmp/second-checksums-files" "$tmp/second-checksum-hashes"
compare_hashes "Registry checksum inventories" "$tmp/first-checksum-hashes" "$tmp/second-checksum-hashes"

hash_normalized_sboms "$first" "$tmp/first-sboms-files" "$tmp/first-sbom-hashes" first
hash_normalized_sboms "$second" "$tmp/second-sboms-files" "$tmp/second-sbom-hashes" second
compare_hashes "normalized SPDX SBOM inventories" "$tmp/first-sbom-hashes" "$tmp/second-sbom-hashes"
