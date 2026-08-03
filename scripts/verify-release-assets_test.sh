#!/bin/sh
set -eu

root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
assets="$tmp/assets"
registered_fingerprint=AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
other_fingerprint=BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB
mkdir "$assets"
printf 'not needed for a rejected signature\n' >"$assets/terraform-provider-hubspot_1.2.3_SHA256SUMS"
printf 'binary signature fixture\n' >"$assets/terraform-provider-hubspot_1.2.3_SHA256SUMS.sig"

# shellcheck disable=SC2016
printf '%s\n' '#!/bin/sh' '
if test "$1 $2" = "--batch --import"; then
  exit 0
fi
if test "$1 $2 $3 $4" = "--batch --status-fd 1 --verify"; then
  test "${CHECKSUM_SIGNED:-true}" = true || exit 1
  signer=${CHECKSUM_SIGNER_FINGERPRINT:-$REGISTERED_FINGERPRINT}
  printf "[GNUPG:] VALIDSIG %s 2026-08-03 0 0 4 0 1 10 00 %s\n" "$signer" "$signer"
  exit 0
fi
echo "unexpected gpg call: $*" >&2
exit 1' >"$tmp/gpg"
chmod +x "$tmp/gpg"

verify() {
	PATH="$tmp:$PATH" REGISTERED_FINGERPRINT="$registered_fingerprint" \
		"$root/scripts/verify-release-assets.sh" "$assets" test "$registered_fingerprint"
}

expect_failure() {
	description=$1
	shift
	if "$@" >"$tmp/output" 2>&1; then
		echo "expected release asset verification failure: $description" >&2
		exit 1
	fi
}

CHECKSUM_SIGNED=false expect_failure 'bad checksum signature' verify
unset CHECKSUM_SIGNED
grep -q 'Registry checksum signature verification failed' "$tmp/output"

CHECKSUM_SIGNER_FINGERPRINT="$other_fingerprint" expect_failure 'unregistered checksum signing identity' verify
unset CHECKSUM_SIGNER_FINGERPRINT
grep -q 'Registry checksum was not signed by the registered signing identity' "$tmp/output"

echo 'Release asset signature contract tests passed'
