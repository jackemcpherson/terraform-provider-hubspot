#!/bin/sh
set -eu

root=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
version=${1:?v-prefixed release version is required}
demo=${2:?cumulative demo checkout is required}

"$root/scripts/validate-release-version.sh" "$version"
(
	cd "$root"
	GOTOOLCHAIN=local go run ./cmd/candidate-compatibility "$version" "$demo"
)
