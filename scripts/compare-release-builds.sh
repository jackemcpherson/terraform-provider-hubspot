#!/bin/sh
set -eu

first=${1:?first build directory is required}
second=${2:?second build directory is required}
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM

(cd "$first" && find . -type f \( -name '*.zip' -o -name '*_SHA256SUMS' -o -name 'terraform-registry-manifest.json' \) -print | sort | xargs shasum -a 256) >"$tmp/first"
(cd "$second" && find . -type f \( -name '*.zip' -o -name '*_SHA256SUMS' -o -name 'terraform-registry-manifest.json' \) -print | sort | xargs shasum -a 256) >"$tmp/second"
diff -u "$tmp/first" "$tmp/second"
