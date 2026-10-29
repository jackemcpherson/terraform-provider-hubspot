#!/bin/sh
set -eu

mode=${1:?mode is required}
shard=${2:?shard is required}
prefix=${3:-}
confirm=${4:-}
generated_ids=${5:-}
lock_dir=${HUBSPOT_ONE_PORTAL_LOCK_DIR:-"${TMPDIR:-/tmp}/hubspot-free-portal-${HUBSPOT_PORTAL_LOCK_ID:-free-configuration}.lock"}
lock_acquired=false

case "$shard" in
  free_properties|form_definitions|files_configuration|account_memberships|product_definitions|contact_segments) ;;
  *) echo "cleanup shard must be free_properties, form_definitions, files_configuration, account_memberships, product_definitions, or contact_segments" >&2; exit 1 ;;
esac

if [ "$shard" = contact_segments ]; then
  printf '%s\n' "$generated_ids" | grep -Eq '^[1-9][0-9]*(,[1-9][0-9]*)*$' || {
    echo "contact_segments cleanup requires exact comma-separated generated IDs" >&2
    exit 1
  }
  HUBSPOT_CONTACT_SEGMENT_IDS=$generated_ids
  export HUBSPOT_CONTACT_SEGMENT_IDS
elif [ -n "$generated_ids" ]; then
  echo "generated IDs are accepted only for contact_segments cleanup" >&2
  exit 1
fi

: "${HUBSPOT_ACCESS_TOKEN:?HUBSPOT_ACCESS_TOKEN is required}"
export CAPABILITY_SHARD="$shard"

finish() {
  code=$?
  if [ "$lock_acquired" = true ]; then rmdir "$lock_dir" || code=1; fi
  exit "$code"
}
trap finish EXIT HUP INT TERM
mkdir "$lock_dir" 2>/dev/null || { echo "HubSpot configuration maintenance is already running: $lock_dir" >&2; exit 1; }
lock_acquired=true

case "$mode" in
  report)
    export HUBSPOT_ACCEPTANCE_PREFIX=tf_acc_
    go test -tags=acceptance ./internal/acceptance -run '^TestAcc_JanitorReport$' -count=1 -timeout=10m
    ;;
  archive)
    case "$shard:$confirm" in
      free_properties:archive-prefixed-crm-configuration|form_definitions:archive-prefixed-form-definitions|files_configuration:delete-prefixed-files-configuration|account_memberships:delete-prefixed-account-memberships|product_definitions:archive-prefixed-product-definitions|contact_segments:delete-exact-contact-segments) ;;
      *) echo "archive confirmation did not match the selected shard" >&2; exit 1 ;;
    esac
    printf '%s\n' "$prefix" | grep -Eq '^tf_acc_[A-Za-z0-9_]+_$' || { echo "refusing to archive outside exact tf_acc_ prefix" >&2; exit 1; }
    export HUBSPOT_ACCEPTANCE_PREFIX="$prefix"
    go test -tags=acceptance ./internal/acceptance -run '^TestAcc_ManualPrefixCleanup$' -count=1 -timeout=20m
    ;;
  *) echo "mode must be report or archive" >&2; exit 1 ;;
esac
