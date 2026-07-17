#!/bin/sh
set -eu

mode=${1:?mode is required}
shard=${2:?shard is required}
prefix=${3:-}
confirm=${4:-}

test "$shard" = free_properties || { echo "v0.1 supports only the free_properties capability shard" >&2; exit 1; }

: "${HUBSPOT_ACCESS_TOKEN:?HUBSPOT_ACCESS_TOKEN is required}"
export CAPABILITY_SHARD=$shard

case "$mode" in
  report)
    export HUBSPOT_ACCEPTANCE_PREFIX=tf_acc_
    go test -tags=acceptance ./internal/acceptance -run '^TestAcc_JanitorReport$' -count=1 -timeout=10m
    ;;
  delete)
    test "$confirm" = "delete-prefixed-configuration" || { echo "cleanup confirmation did not match" >&2; exit 1; }
    printf '%s\n' "$prefix" | grep -Eq '^tf_acc_[A-Za-z0-9_]+_$' || { echo "refusing cleanup outside exact tf_acc_ prefix" >&2; exit 1; }
    export HUBSPOT_ACCEPTANCE_PREFIX=$prefix
    go test -tags=acceptance ./internal/acceptance -run '^TestAcc_ManualPrefixCleanup$' -count=1 -timeout=20m
    ;;
  *) echo "mode must be report or delete" >&2; exit 1 ;;
esac
