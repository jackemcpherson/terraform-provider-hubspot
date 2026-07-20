#!/bin/sh
set -eu

version=${1:?v-prefixed release version is required}
printf '%s\n' "$version" | grep -Eq '^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$' || {
	echo 'version must be v-prefixed SemVer' >&2
	exit 1
}
