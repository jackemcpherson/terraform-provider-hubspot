#!/bin/sh
set -eu

root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM

expect_failure() {
	description=$1
	if "$root/scripts/verify-registry-manifest.sh" "$tmp/manifest.json" >"$tmp/output" 2>&1; then
		echo "expected manifest validation failure: $description" >&2
		exit 1
	fi
	grep -q 'Registry manifest must set numeric version 1' "$tmp/output"
}

printf '%s\n' '{"version":1,"metadata":{"protocol_versions":["6.0"]}}' >"$tmp/manifest.json"
"$root/scripts/verify-registry-manifest.sh" "$tmp/manifest.json"

printf '%s\n' '{"format_version":1,"metadata":{"protocol_versions":["6.0"]}}' >"$tmp/manifest.json"
expect_failure 'the required version field is absent'

printf '%s\n' '{"version":0,"metadata":{"protocol_versions":["6.0"]}}' >"$tmp/manifest.json"
expect_failure 'the manifest version is unknown'

printf '%s\n' '{"version":1,"metadata":{"protocol_versions":["5.0"]}}' >"$tmp/manifest.json"
expect_failure 'the provider protocol is wrong'

printf '%s\n' '{"version":1,"metadata":{"protocol_versions":["6.0"]}' >"$tmp/manifest.json"
expect_failure 'the JSON is malformed'

echo 'Registry manifest contract tests passed'
