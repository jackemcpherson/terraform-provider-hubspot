#!/bin/sh
set -eu

root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
log="$tmp/calls"
assets="$tmp/assets"
demo="$tmp/demo-repo"
evidence="$tmp/evidence"
mkdir -p "$assets" "$demo"
git -C "$demo" init -q
git -C "$demo" config user.name test
git -C "$demo" config user.email test@example.com
touch "$demo/README.md"
git -C "$demo" add README.md
git -C "$demo" commit -qm fixture
os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$(uname -m)" in arm64|aarch64) arch=arm64 ;; *) arch=amd64 ;; esac
touch "$assets/terraform-provider-hubspot_1.2.3_${os}_${arch}.zip"

# The single-quoted strings are the bodies of generated test doubles.
# shellcheck disable=SC2016
printf '%s\n' '#!/bin/sh' 'printf "live:%s\n" "$*" >>"$CALL_LOG"' >"$tmp/live"
# shellcheck disable=SC2016
printf '%s\n' '#!/bin/sh' 'printf "migration:%s\n" "$*" >>"$CALL_LOG"' >"$tmp/migration"
# shellcheck disable=SC2016
printf '%s\n' '#!/bin/sh' 'printf "northstar:%s\n" "$*" >>"$CALL_LOG"' >"$tmp/northstar"
# shellcheck disable=SC2016
printf '%s\n' '#!/bin/sh' 'printf "verify:%s\n" "$*" >>"$CALL_LOG"' >"$tmp/verify"
chmod +x "$tmp/live" "$tmp/migration" "$tmp/northstar" "$tmp/verify"

CALL_LOG="$log" \
	RELEASED_LIVE_SHARD_SCRIPT="$tmp/live" \
	STATE_MIGRATION_SCRIPT="$tmp/migration" \
	RELEASED_NORTHSTAR_SCRIPT="$tmp/northstar" \
	VERIFY_RELEASED_PROVIDER_SCRIPT="$tmp/verify" \
	HUBSPOT_DEMO_REPO="$demo" \
	RELEASE_EVIDENCE_DIR="$evidence" \
	"$root/scripts/released-provider-journey.sh" v1.2.3 "$assets"

test "$(cat "$log")" = "verify:terraform registry.terraform.io/jackemcpherson/hubspot v1.2.3 $assets
verify:tofu registry.opentofu.org/jackemcpherson/hubspot v1.2.3 $assets
live:free_properties terraform registry.terraform.io/jackemcpherson/hubspot v1.2.3
live:free_properties tofu registry.opentofu.org/jackemcpherson/hubspot v1.2.3
migration:v1.2.3
northstar:v1.2.3"

report="$evidence/released-provider-1.2.3.json"
test -s "$report"
grep -q '"status":"passed"' "$report"
grep -q '"archive_sha256":"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"' "$report"
grep -q '"demo_commit":"[0-9a-f]\{40\}"' "$report"
