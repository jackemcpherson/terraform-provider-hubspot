#!/bin/sh
set -eu

root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
log="$tmp/calls"

# The single-quoted string is the body of a generated test double.
# shellcheck disable=SC2016
printf '%s\n' '#!/bin/sh' 'printf "%s|%s\n" "$HUBSPOT_ACCEPTANCE_PREFIX" "$*" >>"$CALL_LOG"' >"$tmp/go"
chmod +x "$tmp/go"

PATH="$tmp:$PATH" CALL_LOG="$log" HUBSPOT_ACCESS_TOKEN=test \
	"$root/scripts/acceptance-cleanup.sh" report free_properties
PATH="$tmp:$PATH" CALL_LOG="$log" HUBSPOT_ACCESS_TOKEN=test \
	"$root/scripts/acceptance-cleanup.sh" archive free_properties tf_acc_owned_ archive-prefixed-crm-configuration

grep -Fq 'tf_acc_|test -tags=acceptance ./internal/acceptance -run ^TestAcc_JanitorReport$ -count=1 -timeout=10m' "$log"
grep -Fq 'tf_acc_owned_|test -tags=acceptance ./internal/acceptance -run ^TestAcc_ManualPrefixCleanup$ -count=1 -timeout=20m' "$log"

if PATH="$tmp:$PATH" CALL_LOG="$log" HUBSPOT_ACCESS_TOKEN=test \
	"$root/scripts/acceptance-cleanup.sh" archive free_properties tf_acc_owned_ delete-prefixed-configuration; then
	echo 'legacy delete confirmation must be rejected' >&2
	exit 1
fi
if PATH="$tmp:$PATH" CALL_LOG="$log" HUBSPOT_ACCESS_TOKEN=test \
	"$root/scripts/acceptance-cleanup.sh" archive free_properties unsafe archive-prefixed-crm-configuration; then
	echo 'unsafe archive prefix must be rejected' >&2
	exit 1
fi
