#!/bin/sh
set -eu

destination=${1:?destination directory is required}
version=${2:?zizmor version is required}

case "$version:$(uname -s):$(uname -m)" in
1.27.0:Darwin:arm64|1.27.0:Darwin:aarch64)
	target=aarch64-apple-darwin
	digest=81336423d1b280c5dd0cdd8644a1e5f3238ab3ceb8d6e4334dfd05dab95a8a86
	;;
1.27.0:Darwin:x86_64)
	target=x86_64-apple-darwin
	digest=51cd82d1f6914cbb7f4402dbdc19bd989a7599078e5ddeaf837d1ab901c97328
	;;
1.27.0:Linux:aarch64|1.27.0:Linux:arm64)
	target=aarch64-unknown-linux-gnu
	digest=46fceee9a8262dca0e61f8463204e1f0f3a63bf6c20fa3ef9a5c1b3cff7b17b0
	;;
1.27.0:Linux:x86_64)
	target=x86_64-unknown-linux-gnu
	digest=277f2bd8fd37cf60c42ab7afca6faa884e65440fa31e02b44bdaae60f62a358f
	;;
*)
	echo "unsupported zizmor release platform: $version $(uname -s) $(uname -m)" >&2
	exit 1
	;;
esac

mkdir -p "$destination"
if test -x "$destination/zizmor" && "$destination/zizmor" --version | grep -Fq "zizmor $version"; then
	exit 0
fi

tmp=$(mktemp -d)
trap 'rm -r "$tmp"' EXIT HUP INT TERM
archive="$tmp/zizmor.tar.gz"
curl -fsSL "https://github.com/zizmorcore/zizmor/releases/download/v$version/zizmor-$target.tar.gz" -o "$archive"
printf '%s  %s\n' "$digest" "$archive" | shasum -a 256 -c -
tar -xzf "$archive" -C "$tmp"
test -x "$tmp/zizmor"
cp "$tmp/zizmor" "$destination/zizmor"
chmod +x "$destination/zizmor"
