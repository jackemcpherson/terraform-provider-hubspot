#!/bin/sh
set -eu

first=${1:?first asset directory is required}
second=${2:?second asset directory is required}
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM

(cd "$first" && find . -type f -print | sort | xargs shasum -a 256) >"$tmp/first"
(cd "$second" && find . -type f -print | sort | xargs shasum -a 256) >"$tmp/second"
diff -u "$tmp/first" "$tmp/second"
