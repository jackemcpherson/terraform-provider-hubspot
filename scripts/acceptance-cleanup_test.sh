#!/bin/sh
set -eu

root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
log="$tmp/calls"

# The single-quoted string is the body of a generated test double.
# shellcheck disable=SC2016
printf '%s\n' '#!/bin/sh' \
	'printf "%s|%s|%s\n" "$CAPABILITY_SHARD" "$HUBSPOT_ACCEPTANCE_PREFIX" "$*" >>"$CALL_LOG"' \
	'test "${GO_RESULT:-passed}" = passed' >"$tmp/go"
chmod +x "$tmp/go"

PATH="$tmp:$PATH" CALL_LOG="$log" HUBSPOT_ACCESS_TOKEN=test HUBSPOT_ONE_PORTAL_LOCK_DIR="$tmp/lock" \
	"$root/scripts/acceptance-cleanup.sh" report free_properties
PATH="$tmp:$PATH" CALL_LOG="$log" HUBSPOT_ACCESS_TOKEN=test HUBSPOT_ONE_PORTAL_LOCK_DIR="$tmp/lock" \
	"$root/scripts/acceptance-cleanup.sh" report form_definitions
PATH="$tmp:$PATH" CALL_LOG="$log" HUBSPOT_ACCESS_TOKEN=test HUBSPOT_ONE_PORTAL_LOCK_DIR="$tmp/lock" \
	"$root/scripts/acceptance-cleanup.sh" archive free_properties tf_acc_owned_ archive-prefixed-crm-configuration
PATH="$tmp:$PATH" CALL_LOG="$log" HUBSPOT_ACCESS_TOKEN=test HUBSPOT_ONE_PORTAL_LOCK_DIR="$tmp/lock" \
	"$root/scripts/acceptance-cleanup.sh" archive form_definitions tf_acc_forms_ archive-prefixed-form-definitions

grep -Fq 'free_properties|tf_acc_|test -tags=acceptance ./internal/acceptance -run ^TestAcc_JanitorReport$ -count=1 -timeout=10m' "$log"
grep -Fq 'form_definitions|tf_acc_|test -tags=acceptance ./internal/acceptance -run ^TestAcc_JanitorReport$ -count=1 -timeout=10m' "$log"
grep -Fq 'free_properties|tf_acc_owned_|test -tags=acceptance ./internal/acceptance -run ^TestAcc_ManualPrefixCleanup$ -count=1 -timeout=20m' "$log"
grep -Fq 'form_definitions|tf_acc_forms_|test -tags=acceptance ./internal/acceptance -run ^TestAcc_ManualPrefixCleanup$ -count=1 -timeout=20m' "$log"

if PATH="$tmp:$PATH" CALL_LOG="$log" HUBSPOT_ACCESS_TOKEN=test HUBSPOT_ONE_PORTAL_LOCK_DIR="$tmp/lock" \
	"$root/scripts/acceptance-cleanup.sh" archive free_properties tf_acc_owned_ delete-prefixed-configuration; then
	echo 'legacy delete confirmation must be rejected' >&2
	exit 1
fi
if PATH="$tmp:$PATH" CALL_LOG="$log" HUBSPOT_ACCESS_TOKEN=test HUBSPOT_ONE_PORTAL_LOCK_DIR="$tmp/lock" \
	"$root/scripts/acceptance-cleanup.sh" archive free_properties unsafe archive-prefixed-crm-configuration; then
	echo 'unsafe archive prefix must be rejected' >&2
	exit 1
fi
if PATH="$tmp:$PATH" CALL_LOG="$log" HUBSPOT_ACCESS_TOKEN=test HUBSPOT_ONE_PORTAL_LOCK_DIR="$tmp/lock" \
	"$root/scripts/acceptance-cleanup.sh" archive form_definitions tf_acc_forms_ archive-prefixed-crm-configuration; then
	echo 'CRM confirmation must not authorize Forms cleanup' >&2
	exit 1
fi
if PATH="$tmp:$PATH" CALL_LOG="$log" HUBSPOT_ACCESS_TOKEN=test HUBSPOT_ONE_PORTAL_LOCK_DIR="$tmp/lock" \
	"$root/scripts/acceptance-cleanup.sh" report unknown_shard; then
	echo 'unknown cleanup shard must be rejected' >&2
	exit 1
fi

mkdir "$tmp/busy-lock"
if PATH="$tmp:$PATH" CALL_LOG="$log" HUBSPOT_ACCESS_TOKEN=test HUBSPOT_ONE_PORTAL_LOCK_DIR="$tmp/busy-lock" \
	"$root/scripts/acceptance-cleanup.sh" report form_definitions; then
	echo 'shared portal lock collision must fail closed' >&2
	exit 1
fi
rmdir "$tmp/busy-lock"

if PATH="$tmp:$PATH" CALL_LOG="$log" GO_RESULT=failed HUBSPOT_ACCESS_TOKEN=test HUBSPOT_ONE_PORTAL_LOCK_DIR="$tmp/guard-lock" \
	"$root/scripts/acceptance-cleanup.sh" archive form_definitions tf_acc_forms_ archive-prefixed-form-definitions; then
	echo 'portal guard or janitor failure must fail cleanup' >&2
	exit 1
fi
test ! -e "$tmp/guard-lock" || { echo 'failed cleanup retained the shared portal lock' >&2; exit 1; }
