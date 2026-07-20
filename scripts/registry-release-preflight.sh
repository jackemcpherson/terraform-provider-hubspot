#!/bin/sh
set -eu

root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
version=${1:-v0.0.0-preflight}

printf '%s\n' "$version" | grep -Eq '^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$' || {
	echo 'version must be v-prefixed SemVer' >&2
	exit 1
}

cd "$root"
bundle=$(mktemp -d)
trap 'rm -rf "$bundle"' EXIT HUP INT TERM
RELEASE_PREFLIGHT=1 "$root/scripts/build-release-bundle.sh" "$version" "$bundle"

echo "Registry release pre-flight passed for $version with OpenTofu and Terraform"
