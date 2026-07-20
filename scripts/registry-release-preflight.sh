#!/bin/sh
set -eu

root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
version=${1:-v0.0.0-preflight}

"$root/scripts/validate-release-version.sh" "$version"

cd "$root"
bundle=$(mktemp -d)
trap 'rm -rf "$bundle"' EXIT HUP INT TERM
RELEASE_PREFLIGHT=1 "$root/scripts/build-release-bundle.sh" "$version" "$bundle"

echo "Registry release pre-flight passed for $version with OpenTofu and Terraform"
