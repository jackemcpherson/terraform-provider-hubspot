#!/bin/sh
set -eu

root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
assets=${1:?release asset directory is required}
version=${2:?v-prefixed release version is required}

printf '%s\n' "$version" | grep -Eq '^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$' || {
	echo 'version must be v-prefixed SemVer' >&2
	exit 1
}

"$root/scripts/verify-registry-checksums.sh" "$assets"

release_version=${version#v}
checksum="$assets/terraform-provider-hubspot_${release_version}_SHA256SUMS"
test -f "$checksum" || {
	echo 'checksum filename does not match the requested release version' >&2
	exit 1
}

archive_count=$(find "$assets" -maxdepth 1 -type f -name '*.zip' | wc -l | tr -d ' ')
sbom_count=$(find "$assets" -maxdepth 1 -type f -name '*.zip.spdx.sbom' | wc -l | tr -d ' ')
test "$archive_count" -eq 13 || {
	echo 'release bundle must contain 13 standard provider archives' >&2
	exit 1
}
test "$sbom_count" -eq "$archive_count" || {
	echo 'release bundle must contain one SPDX SBOM per provider archive' >&2
	exit 1
}

for sbom in "$assets"/*.zip.spdx.sbom; do
	jq -e '.spdxVersion | type == "string"' "$sbom" >/dev/null || {
		echo "invalid SPDX SBOM: $(basename "$sbom")" >&2
		exit 1
	}
done

for archive in "$assets"/*.zip; do
	expected_binary="terraform-provider-hubspot_v${release_version}"
	case "$archive" in *_windows_*.zip) expected_binary=${expected_binary}.exe ;; esac
	provider_binary_count=$(unzip -Z1 "$archive" | grep -Ec '^terraform-provider-hubspot_v[^/]+([.]exe)?$' || true)
	test "$provider_binary_count" -eq 1 || {
		echo "provider archive must contain exactly one provider binary: $(basename "$archive")" >&2
		exit 1
	}
	if ! unzip -Z1 "$archive" | grep -Fxq "$expected_binary"; then
		echo "provider archive has no correctly versioned binary: $(basename "$archive")" >&2
		exit 1
	fi
done

"$root/scripts/smoke-release-archive.sh" "$assets" "$version"
