#!/bin/sh
set -eu

root=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
version=${1:?v-prefixed candidate version is required}
demo=${2:?cumulative demo checkout is required}
release_preflight=${REGISTRY_RELEASE_PREFLIGHT_SCRIPT:-"$root/scripts/registry-release-preflight.sh"}

"$root/scripts/validate-candidate-compatibility.sh" "$version" "$demo"
"$release_preflight" "$version"
