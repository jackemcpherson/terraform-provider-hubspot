#!/bin/sh
set -eu

version=v0.4.0
release_notes=docs/releases/$version.md

grep -q '^## \[0.4.0\] - 2026-08-09$' CHANGELOG.md || { echo "CHANGELOG is not finalized for $version" >&2; exit 1; }
grep -q 'v0.4.0 release is a public beta' README.md || { echo "README does not name the exact candidate version" >&2; exit 1; }
test -s "$release_notes" || { echo "reviewed $version release notes are missing" >&2; exit 1; }
for phrase in hubspot_file_folder hubspot_file 'generated folder and file IDs' 'exact `files` scope' 'does not manage CMS Developer File System content' 'OpenTofu Registry'; do
  grep -Fqi "$phrase" "$release_notes" || { echo "release notes are missing: $phrase" >&2; exit 1; }
done
grep -A1 '^changelog:$' .goreleaser.yml | grep -q 'disable: false' || { echo "GoReleaser would discard reviewed release notes" >&2; exit 1; }
grep -q 'args: release --clean --release-notes docs/releases/v0.4.0.md' .github/workflows/release.yml || {
	echo "protected publication does not use the reviewed v0.4.0 notes" >&2
  exit 1
}
! grep -Eqi 'provenance attestation|SPDX attestation' "$release_notes" || {
  echo "release notes claim evidence excluded by ADR 0002" >&2
  exit 1
}
