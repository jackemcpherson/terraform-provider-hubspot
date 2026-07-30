#!/bin/sh
set -eu

license=${1:-LICENSE}
expected=3f3d9e0024b1921b067d6f7f88deb4a60cbe7a78e76c64e3f1d7fc3b779b9d04

if [ ! -f "$license" ]; then
	echo "canonical MPL-2.0 license missing: $license" >&2
	exit 1
fi

actual=$(shasum -a 256 "$license" | awk '{print $1}')
if [ "$actual" != "$expected" ]; then
	echo "LICENSE must contain the canonical MPL-2.0 text" >&2
	exit 1
fi
