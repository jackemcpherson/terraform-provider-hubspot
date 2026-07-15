#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM

tool=$(go env GOPATH)/bin/tfplugindocs
test -x "$tool" || {
  echo "tfplugindocs is required; run make tools" >&2
  exit 1
}

mkdir -p "$tmp/docs"
cp "$root/docs/index.md" "$tmp/docs/index.md"
cp -R "$root/docs/resources" "$tmp/docs/resources"
cp -R "$root/docs/data-sources" "$tmp/docs/data-sources"

"$tool" generate --provider-dir "$root" --provider-name hubspot >/dev/null

for path in index.md resources data-sources; do
  diff -ru "$tmp/docs/$path" "$root/docs/$path"
done
