#!/bin/sh
set -eu

root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
log="$tmp/calls"

# The single-quoted strings are the bodies of generated test doubles.
# shellcheck disable=SC2016
printf '%s\n' '#!/bin/sh' 'printf "live:%s\n" "$*" >>"$CALL_LOG"' >"$tmp/live"
# shellcheck disable=SC2016
printf '%s\n' '#!/bin/sh' 'printf "migration:%s\n" "$*" >>"$CALL_LOG"' >"$tmp/migration"
chmod +x "$tmp/live" "$tmp/migration"

CALL_LOG="$log" \
	RELEASED_LIVE_SHARD_SCRIPT="$tmp/live" \
	STATE_MIGRATION_SCRIPT="$tmp/migration" \
	"$root/scripts/released-provider-journey.sh" v1.2.3

test "$(cat "$log")" = 'live:free_properties terraform registry.terraform.io/jackemcpherson/hubspot v1.2.3
live:free_properties tofu registry.opentofu.org/jackemcpherson/hubspot v1.2.3
migration:v1.2.3'
