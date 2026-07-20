#!/bin/sh
set -eu

manifest=${1:?Registry manifest path is required}

command -v jq >/dev/null 2>&1 || {
	echo 'jq is required to validate the Registry manifest' >&2
	exit 1
}

if ! jq -e '
	type == "object" and
	(.version | type) == "number" and
	.version == 1 and
	(.metadata | type) == "object" and
	(.metadata.protocol_versions | type) == "array" and
	.metadata.protocol_versions == ["6.0"]
' "$manifest" >/dev/null; then
	echo 'Registry manifest must set numeric version 1 and metadata.protocol_versions to ["6.0"]' >&2
	exit 1
fi
