#!/bin/sh
set -eu

shard=${CAPABILITY_SHARD:?CAPABILITY_SHARD is required}
token=${HUBSPOT_ACCESS_TOKEN:?HUBSPOT_ACCESS_TOKEN is required}
prefix=${HUBSPOT_ACCEPTANCE_PREFIX:?HUBSPOT_ACCEPTANCE_PREFIX is required}
manifest="acceptance/capabilities/$shard.json"
report_dir=${ACCEPTANCE_REPORT_DIR:-acceptance-report}
status=failed
ledger=
mkdir -p "$report_dir"

finish() {
  code=$?
  if [ -n "$ledger" ] && test -s "$ledger"; then
    echo "acceptance cleanup ledger is not empty" >&2
    status=failed
    code=1
  fi
  printf '{"shard":"%s","engine":"tofu-1.12.3","status":"%s"}\n' "$shard" "$status" >"$report_dir/$shard.json"
  if [ -n "$ledger" ]; then rm -f "$ledger"; fi
  exit "$code"
}
trap finish EXIT
trap 'exit 1' HUP INT TERM

case "$shard" in
  free_properties|deal_pipelines|ticket_pipelines|custom_schemas|sensitive_properties|custom_pipelines) ;;
  *) echo "unknown capability shard" >&2; exit 1 ;;
esac

printf '%s\n' "$prefix" | grep -Eq '^tf_acc_[A-Za-z0-9_]+_$' || { echo "acceptance prefix must use tf_acc_ and end with an underscore" >&2; exit 1; }

test -s "$manifest"
grep -q "\"shard\":\"$shard\"" "$manifest"
grep -q '"quota_preflight":true' "$manifest"
if grep -Eqi 'hub[_-]?id|app[_-]?id|record[_-]?id|access[_-]?token|pat-' "$manifest"; then
  echo "capability manifest contains a forbidden identifier or credential marker" >&2
  exit 1
fi

ledger=$(mktemp)
export HUBSPOT_ACCEPTANCE_CLEANUP_LEDGER=$ledger
export HUBSPOT_ACCEPTANCE=1

regex="^TestAcc_${shard}_"
tests=$(go test -tags=acceptance ./internal/provider -list "$regex")
printf '%s\n' "$tests" | grep -q "TestAcc_${shard}_" || {
  echo "no acceptance tests registered for required shard $shard" >&2
  exit 1
}
go test -tags=acceptance ./internal/provider -run "$regex" -count=1 -timeout=20m
status=passed

# Keep the secret referenced so shell linters do not mistake it for optional.
test -n "$token"
