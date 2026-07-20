#!/bin/sh
set -eu

root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
version=${1:-v0.0.0-preflight}
release_version=${version#v}

printf '%s\n' "$version" | grep -Eq '^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$' || {
	echo 'version must be v-prefixed SemVer' >&2
	exit 1
}

command -v goreleaser >/dev/null 2>&1 || {
	echo 'goreleaser is required; run make tools' >&2
	exit 1
}

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
"$root/scripts/verify-release-bundle.sh" "$root/dist" "$version"

echo "Registry release pre-flight passed for $version with OpenTofu and Terraform"
