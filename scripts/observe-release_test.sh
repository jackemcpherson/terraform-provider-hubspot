#!/bin/sh
set -eu

root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
candidate=1111111111111111111111111111111111111111
tag_commit=2222222222222222222222222222222222222222
registered_fingerprint=AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
other_fingerprint=BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB
fixture_root="$tmp/release-assets"
valid_assets="$fixture_root/valid"
mkdir -p "$valid_assets"

for platform in \
	darwin_amd64 \
	darwin_arm64 \
	freebsd_386 \
	freebsd_amd64 \
	freebsd_arm \
	freebsd_arm64 \
	linux_386 \
	linux_amd64 \
	linux_arm \
	linux_arm64 \
	windows_386 \
	windows_amd64 \
	windows_arm64
do
	archive_source="$tmp/archive-$platform"
	mkdir "$archive_source"
	binary=terraform-provider-hubspot_v1.2.3
	case "$platform" in windows_*) binary=${binary}.exe ;; esac
	printf '#!/bin/sh\nexit 0\n' >"$archive_source/$binary"
	chmod +x "$archive_source/$binary"
	(cd "$archive_source" && zip -q "$valid_assets/terraform-provider-hubspot_1.2.3_${platform}.zip" "$binary")
done
printf '%s\n' '{"version":1,"metadata":{"protocol_versions":["6.0"]}}' >"$valid_assets/terraform-provider-hubspot_1.2.3_manifest.json"

write_checksums() {
	assets=$1
	(
		cd "$assets"
		for asset in terraform-provider-hubspot_1.2.3_*.zip terraform-provider-hubspot_1.2.3_manifest.json; do
			shasum -a 256 "$asset"
		done | LC_ALL=C sort -k 2
	) >"$assets/terraform-provider-hubspot_1.2.3_SHA256SUMS"
	printf 'binary signature fixture\n' >"$assets/terraform-provider-hubspot_1.2.3_SHA256SUMS.sig"
}
write_checksums "$valid_assets"

for state in bad-signature bad-manifest bad-closure; do
	mkdir "$fixture_root/$state"
	cp "$valid_assets"/* "$fixture_root/$state/"
done
printf '%s\n' '{"version":0,"metadata":{"protocol_versions":["6.0"]}}' >"$fixture_root/bad-manifest/terraform-provider-hubspot_1.2.3_manifest.json"
write_checksums "$fixture_root/bad-manifest"
rm "$fixture_root/bad-closure/terraform-provider-hubspot_1.2.3_windows_arm64.zip"
write_checksums "$fixture_root/bad-closure"

# These generated commands isolate the release-observation contract from GitHub
# and Git while exercising the production asset verification scripts.
# shellcheck disable=SC2016
printf '%s\n' '#!/bin/sh' '
case "$1 $2" in
  "rev-parse HEAD") printf "%s\n" "$CANDIDATE_COMMIT" ;;
  "ls-remote --exit-code") test "$TAG_EXISTS" = true && printf "%s\trefs/tags/%s\n" "$TAG_COMMIT" "$VERSION" ;;
  "fetch --quiet") ;;
  "rev-list -n") printf "%s\n" "$TAG_COMMIT" ;;
  "merge-base --is-ancestor") test "${TAG_ON_MAIN:-true}" = true ;;
  "verify-tag --raw")
    test "${TAG_SIGNED:-true}" = true || exit 1
    signer=${TAG_SIGNER_FINGERPRINT:-$REGISTERED_FINGERPRINT}
    printf "[GNUPG:] VALIDSIG %s 2026-08-03 0 0 4 0 1 10 00 %s\n" "$signer" "$signer" >&2
    ;;
  *) echo "unexpected git call: $*" >&2; exit 1 ;;
esac' >"$tmp/git"
# shellcheck disable=SC2016
printf '%s\n' '#!/bin/sh' '
assets="$FIXTURE_ROOT/${EVIDENCE_STATE:-valid}"
if test "$1" = api && printf "%s" "$2" | grep -q /check-runs; then
  test "${REQUIRED_RESULT:-success}" = success && printf "true\n" || printf "false\n"
elif test "$1" = api; then
  test "$RELEASE_STATE" != none || exit 1
  test "$RELEASE_STATE" = draft && draft=true || draft=false
  printf "{\"draft\":%s,\"assets\":[" "$draft"
  separator=
  for asset in "$assets"/*; do
    name=${asset##*/}
    digest=$(shasum -a 256 "$asset")
    digest=${digest%% *}
    if test "${DIGEST_VALID:-true}" != true && test -z "$separator"; then
      digest=ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff
    fi
    printf "%s{\"name\":\"%s\",\"digest\":\"sha256:%s\"}" "$separator" "$name" "$digest"
    separator=,
  done
  printf "]}\n"
elif test "$1 $2" = "release download"; then
  while test "$#" -gt 0; do
    if test "$1" = --dir; then shift; mkdir -p "$1"; cp "$assets"/* "$1/"; break; fi
    shift
  done
else
  echo "unexpected gh call: $*" >&2; exit 1
fi' >"$tmp/gh"
# shellcheck disable=SC2016
printf '%s\n' '#!/bin/sh' '
if test "$1 $2" = "--batch --import"; then exit 0; fi
if test "$1 $2 $3 $4" = "--batch --status-fd 1 --verify"; then
  test "${EVIDENCE_STATE:-valid}" != bad-signature || exit 1
  printf "[GNUPG:] VALIDSIG %s 2026-08-03 0 0 4 0 1 10 00 %s\n" "$REGISTERED_FINGERPRINT" "$REGISTERED_FINGERPRINT"
  exit 0
fi
echo "unexpected gpg call: $*" >&2
exit 1' >"$tmp/gpg"
# shellcheck disable=SC2016
printf '%s\n' '#!/bin/sh' '
case "$*" in
  *"providers schema -json"*) printf "{\"hubspot_property_group\":{}}\n" ;;
esac' >"$tmp/terraform"
cp "$tmp/terraform" "$tmp/tofu"
chmod +x "$tmp/git" "$tmp/gh" "$tmp/gpg" "$tmp/terraform" "$tmp/tofu"

observe() {
	PATH="$tmp:$PATH" GH_TOKEN=test GPG_PUBLIC_KEY=test \
		GPG_FINGERPRINT="$registered_fingerprint" REGISTERED_FINGERPRINT="$registered_fingerprint" \
		CANDIDATE_COMMIT="$candidate" TAG_COMMIT="$tag_commit" VERSION=v1.2.3 \
		TAG_EXISTS="$1" RELEASE_STATE="$2" FIXTURE_ROOT="$fixture_root" \
		"$root/scripts/observe-release.sh" v1.2.3 "$candidate" owner/repository
}

expect_failure() {
	description=$1
	expected=$2
	shift 2
	if "$@" >"$tmp/output" 2>&1; then
		echo "expected release observation failure: $description" >&2
		exit 1
	fi
	grep -q "$expected" "$tmp/output" || {
		echo "release observation failed at the wrong boundary: $description" >&2
		sed -n '1,120p' "$tmp/output" >&2
		exit 1
	}
}

expect_success() {
	expected=$1
	shift
	if ! "$@" >"$tmp/output" 2>&1; then
		echo "expected release observation success: $expected" >&2
		sed -n '1,160p' "$tmp/output" >&2
		exit 1
	fi
	test "$(tail -n 1 "$tmp/output")" = "$expected"
}

expect_success "new $candidate" observe false none
expect_success "draft $tag_commit" observe true draft
expect_success "published $tag_commit" observe true published

expect_failure 'tag without release' 'immutable conflict' observe true none
expect_failure 'release without tag' 'immutable conflict' observe false draft
TAG_SIGNED=false expect_failure 'bad signed tag' 'release tag signature verification failed' observe true draft
unset TAG_SIGNED
REQUIRED_RESULT=failed expect_failure 'failed Required check' 'no successful Required check' observe false none
unset REQUIRED_RESULT
TAG_ON_MAIN=false expect_failure 'release commit outside main' 'existing release commit is not on main' observe true published
unset TAG_ON_MAIN
TAG_SIGNER_FINGERPRINT="$other_fingerprint" expect_failure 'unregistered tag signing identity' 'release tag was not signed by the registered signing identity' observe true published
unset TAG_SIGNER_FINGERPRINT
EVIDENCE_STATE=bad-signature expect_failure 'bad checksum signature' 'Registry checksum signature verification failed' observe true published
unset EVIDENCE_STATE
EVIDENCE_STATE=bad-manifest expect_failure 'bad Registry manifest' 'Registry manifest must set numeric version 1' observe true published
unset EVIDENCE_STATE
EVIDENCE_STATE=bad-closure expect_failure 'bad archive and checksum closure' 'release package assets do not match the supported platform set' observe true published
unset EVIDENCE_STATE
DIGEST_VALID=false expect_failure 'bad immutable asset digest' 'downloaded release assets do not match GitHub immutable asset digests' observe true published
unset DIGEST_VALID

echo 'Release observer contract tests passed'
