#!/bin/sh
set -eu

root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
log="$tmp/calls"
demo_root="$tmp/demo-root"
mkdir -p "$demo_root/scripts" "$demo_root/locks/tofu" "$demo_root/locks/terraform"
short_prefix() {
  seed=$1
  engine=$2
  code=$3
  namespace=$(printf '%s' "${seed}_${engine}" | shasum -a 256 | cut -c1-8)
  printf 'ns_%s_%s_\n' "$namespace" "$code"
}
seed=test_123
tofu_prefix=$(short_prefix "$seed" tofu o)
terraform_prefix=$(short_prefix "$seed" terraform t)

# The fixture expands these variables when it runs.
# shellcheck disable=SC2016
printf '%s\n' '#!/bin/sh' \
  'case "$ENGINE" in tofu) expected=$EXPECTED_TOFU_PREFIX ;; terraform) expected=$EXPECTED_TERRAFORM_PREFIX ;; esac' \
  'test "$HUBSPOT_NORTHSTAR_FILES_PREFIX" = "$expected"' \
  'printf "%s:%s:%s:%s\n" "$ENGINE" "$1" "$2" "$HUBSPOT_NORTHSTAR_FILES_PREFIX" >>"$CALL_LOG"' \
  'test "${FAIL_PHASE:-none}" != "$ENGINE:$2"' >"$demo_root/scripts/demo"
chmod +x "$demo_root/scripts/demo"
# The fixture expands these variables when it runs.
# shellcheck disable=SC2016
printf '%s\n' '#!/bin/sh' \
  'test "$HUBSPOT_NORTHSTAR_FILES_PREFIX" = "$EXPECTED_TOFU_PREFIX"' \
  'printf "%s\n" cleanup >>"$CALL_LOG"' >"$tmp/cleanup"
chmod +x "$tmp/cleanup"
printf '%s\n' '#!/bin/sh' \
  'case "$1" in tofu) expected=$EXPECTED_TOFU_PREFIX ;; terraform) expected=$EXPECTED_TERRAFORM_PREFIX ;; esac' \
  'test "$HUBSPOT_NORTHSTAR_FILES_PREFIX" = "$expected"' \
  'printf "%s:stage:%s\n" "$1" "$HUBSPOT_NORTHSTAR_FILES_PREFIX" >>"$CALL_LOG"' >"$tmp/stage"
chmod +x "$tmp/stage"
cat >"$demo_root/versions.tf" <<'EOF'
terraform {
  required_providers {
    hubspot = {
      source  = "jackemcpherson/hubspot"
      version = ">= 0.4.0, < 0.5.0"
    }
  }
}
EOF
for engine in tofu terraform; do
  case "$engine" in
    tofu) address=registry.opentofu.org/jackemcpherson/hubspot ;;
    terraform) address=registry.terraform.io/jackemcpherson/hubspot ;;
  esac
  cat >"$demo_root/locks/$engine/.terraform.lock.hcl" <<EOF
provider "$address" {
  version     = "0.4.0"
  constraints = ">= 0.4.0, < 0.5.0"
  hashes      = ["h1:0WnQNSWX/OwYe1pbEN8Kla8ej2VV90oP7vLcBj6SZ7A="]
}
EOF
done

run() {
  CALL_LOG="$log" HUBSPOT_DEMO_REPO="$demo_root" HUBSPOT_DEMO_SCRIPT="$demo_root/scripts/demo" \
    HUBSPOT_NORTHSTAR_CLEANUP_SCRIPT="$tmp/cleanup" \
    HUBSPOT_NORTHSTAR_STAGE_SCRIPT="$tmp/stage" \
    HUBSPOT_NORTHSTAR_FILES_SEED="$seed" EXPECTED_TOFU_PREFIX="$tofu_prefix" EXPECTED_TERRAFORM_PREFIX="$terraform_prefix" \
    HUBSPOT_ONE_PORTAL_LOCK_DIR="$tmp/lock" \
    "$root/scripts/northstar-candidate-lifecycle.sh" v0.4.0
}

run
for engine in tofu terraform; do
  case "$engine" in tofu) prefix=$tofu_prefix ;; terraform) prefix=$terraform_prefix ;; esac
  for phase in plan apply verify drift repair; do
    printf '%s:%s:%s:%s\n' "$engine" local "$phase" "$prefix"
  done
  printf '%s:stage:%s\n' "$engine" "$prefix"
  for phase in refresh adopt verify destroy-plan destroy-apply; do
    printf '%s:%s:%s:%s\n' "$engine" local "$phase" "$prefix"
  done
done >"$tmp/expected"
cmp "$log" "$tmp/expected"
if test -e "$tmp/lock"; then
  echo "Northstar lifecycle retained the portal lock after a successful candidate lifecycle" >&2
  exit 1
fi

: >"$log"
if CALL_LOG="$log" FAIL_PHASE=tofu:repair HUBSPOT_DEMO_REPO="$demo_root" \
  HUBSPOT_DEMO_SCRIPT="$demo_root/scripts/demo" HUBSPOT_NORTHSTAR_CLEANUP_SCRIPT="$tmp/cleanup" \
  HUBSPOT_NORTHSTAR_STAGE_SCRIPT="$tmp/stage" \
  HUBSPOT_NORTHSTAR_FILES_SEED="$seed" EXPECTED_TOFU_PREFIX="$tofu_prefix" EXPECTED_TERRAFORM_PREFIX="$terraform_prefix" \
  HUBSPOT_ONE_PORTAL_LOCK_DIR="$tmp/lock" \
  "$root/scripts/northstar-candidate-lifecycle.sh" v0.4.0; then
  echo "Northstar lifecycle accepted a skipped or failed Forms phase" >&2
  exit 1
fi
test "$(tail -2 "$log" | head -1)" = "tofu:local:repair:$tofu_prefix"
test "$(tail -1 "$log")" = cleanup
if test -e "$tmp/lock"; then
  echo "Northstar lifecycle retained the portal lock after a failed phase" >&2
  exit 1
fi

mkdir "$tmp/lock"
if run; then
  echo "Northstar lifecycle accepted a shared portal lock collision" >&2
  exit 1
fi

rm -r "$tmp/lock"
: >"$log"
sed 's/< 0.5.0/< 0.4.0/' "$demo_root/versions.tf" >"$demo_root/stale.tf"
rm "$demo_root/versions.tf"
if run; then
  echo "Northstar lifecycle accepted an incompatible candidate" >&2
  exit 1
fi
test ! -s "$log"
test ! -e "$tmp/lock"
