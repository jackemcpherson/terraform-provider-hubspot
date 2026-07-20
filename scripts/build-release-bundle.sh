#!/bin/sh
set -eu

root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
version=${1:?v-prefixed release version is required}
output=${2:?output directory is required}
release_version=${version#v}
case "$output" in
	/*) ;;
	*) output="$(pwd)/$output" ;;
esac

"$root/scripts/validate-release-version.sh" "$version"

for tool in goreleaser jq syft; do
	command -v "$tool" >/dev/null 2>&1 || {
		echo "$tool is required; run make tools" >&2
		exit 1
	}
done

if test -e "$output" && test -n "$(find "$output" -mindepth 1 -maxdepth 1 -print -quit)"; then
	echo "output directory must not exist or must be empty: $output" >&2
	exit 1
fi

mkdir -p "$output/assets" "$output/metadata"
cd "$root"
goreleaser check
goreleaser healthcheck
"$root/scripts/verify-registry-manifest.sh" "$root/terraform-registry-manifest.json"

GORELEASER_CURRENT_TAG="$version" goreleaser release \
	--clean \
	--parallelism=2 \
	--skip=announce,publish,sign,validate

test "$(jq '[.[] | select(.type == "Archive")] | length' dist/artifacts.json)" -eq 13 || {
	echo 'GoReleaser artifact catalog must contain 13 provider archives' >&2
	exit 1
}
test "$(jq '[.[] | select(.type == "SBOM")] | length' dist/artifacts.json)" -eq 13 || {
	echo 'GoReleaser artifact catalog must contain one SBOM per archive' >&2
	exit 1
}
test "$(jq '[.[] | select(.type == "Checksum")] | length' dist/artifacts.json)" -eq 1 || {
	echo 'GoReleaser artifact catalog must contain one checksum inventory' >&2
	exit 1
}

manifest_asset="terraform-provider-hubspot_${release_version}_manifest.json"
cp "$root/terraform-registry-manifest.json" "$root/dist/$manifest_asset"
find "$root/dist" -maxdepth 1 -type f \( -name '*.zip' -o -name '*.spdx.sbom' -o -name '*_SHA256SUMS' -o -name '*_manifest.json' \) \
	-exec cp {} "$output/assets/" \;

"$root/scripts/verify-release-bundle.sh" "$output/assets" "$version"
syft scan "dir:$output/assets" --output "spdx-json=$output/metadata/release.spdx.json"
if ! "$root/scripts/release-notes.sh" "$version" >"$output/metadata/release-notes.md"; then
	if test "${RELEASE_PREFLIGHT:-}" != 1; then
		echo "CHANGELOG.md has no release notes for $version" >&2
		exit 1
	fi
	printf '# Pre-flight %s\n' "$version" >"$output/metadata/release-notes.md"
fi
