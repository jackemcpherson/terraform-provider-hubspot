#!/bin/sh
set -eu

shard=${CAPABILITY_SHARD:?CAPABILITY_SHARD is required}
token=${HUBSPOT_ACCESS_TOKEN:?HUBSPOT_ACCESS_TOKEN is required}
prefix=${HUBSPOT_ACCEPTANCE_PREFIX:?HUBSPOT_ACCEPTANCE_PREFIX is required}
manifest="acceptance/capabilities/$shard.json"
report_dir=${ACCEPTANCE_REPORT_DIR:-acceptance-report}
status=failed
cleanup=passed
ledger=
binary_dir=
commit=$(git rev-parse HEAD)
manifest_sha=
provider_sha=
suite_sha=
tofu_version=
terraform_version=
mkdir -p "$report_dir"

finish() {
  code=$?
  if [ -n "$ledger" ] && test -s "$ledger"; then
    echo "acceptance cleanup ledger is not empty" >&2
    status=failed
    cleanup=failed
    code=1
  fi
  printf '{"commit":"%s","shard":"%s","manifest_sha256":"%s","provider_sha256":"%s","suite_sha256":"%s","engines":["tofu-%s","terraform-%s"],"cleanup":"%s","status":"%s"}\n' \
    "$commit" "$shard" "$manifest_sha" "$provider_sha" "$suite_sha" "$tofu_version" "$terraform_version" "$cleanup" "$status" >"$report_dir/$shard.json"
  if [ -n "$ledger" ]; then rm -f "$ledger"; fi
  if [ -n "$binary_dir" ]; then rm -rf "$binary_dir"; fi
  exit "$code"
}
trap finish EXIT
trap 'exit 1' HUP INT TERM

test "$shard" = free_properties || { echo "v0.1 supports only the free_properties capability shard" >&2; exit 1; }

printf '%s\n' "$prefix" | grep -Eq '^tf_acc_[A-Za-z0-9_]+_$' || { echo "acceptance prefix must use tf_acc_ and end with an underscore" >&2; exit 1; }

test -s "$manifest"
manifest_sha=$(shasum -a 256 "$manifest" | awk '{print $1}')
grep -q "\"shard\":\"$shard\"" "$manifest"
grep -q '"quota_preflight":true' "$manifest"
if grep -Eqi 'hub[_-]?id|app[_-]?id|record[_-]?id|access[_-]?token|pat-' "$manifest"; then
  echo "capability manifest contains a forbidden identifier or credential marker" >&2
  exit 1
fi

test -z "$(git status --porcelain --untracked-files=all)" || { echo "acceptance source tree is not the exact clean commit" >&2; exit 1; }
tofu_version=$(tofu version | sed -n '1s/^OpenTofu v//p')
terraform_version=$(terraform version | sed -n '1s/^Terraform v//p')
test "$tofu_version" = "1.12.3" || { echo "unexpected OpenTofu acceptance version" >&2; exit 1; }
test "$terraform_version" = "1.15.8" || { echo "unexpected Terraform acceptance version" >&2; exit 1; }

binary_dir=$(mktemp -d)
provider_binary="$binary_dir/terraform-provider-hubspot"
CGO_ENABLED=0 GOTOOLCHAIN=local go build -trimpath -o "$provider_binary" .
provider_sha=$(shasum -a 256 "$provider_binary" | awk '{print $1}')
export HUBSPOT_ACCEPTANCE_PROVIDER_BINARY=$provider_binary

ledger=$(mktemp)
export HUBSPOT_ACCEPTANCE_CLEANUP_LEDGER=$ledger
export HUBSPOT_ACCEPTANCE=1

regex="^TestAcc_${shard}_"
tests=$(go test -tags=acceptance ./internal/acceptance -list "$regex")
suite_sha=$(find internal/acceptance -type f -name '*.go' -print | LC_ALL=C sort | while IFS= read -r source; do shasum -a 256 "$source"; done | shasum -a 256 | awk '{print $1}')
printf '%s\n' "$tests" | grep -q "TestAcc_${shard}_" || {
  echo "no acceptance tests registered for required shard $shard" >&2
  exit 1
}
printf '%s\n' "$tests" | grep -qx "TestAcc_${shard}_QuotaPreflight" || {
  echo "no quota preflight registered for required shard $shard" >&2
  exit 1
}
go test -tags=acceptance ./internal/acceptance -run "^TestAcc_${shard}_QuotaPreflight$" -count=1 -timeout=5m
go test -tags=acceptance ./internal/acceptance -run "$regex" -count=1 -timeout=20m
status=passed

# Keep the secret referenced so shell linters do not mistake it for optional.
test -n "$token"
