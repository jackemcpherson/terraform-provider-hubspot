#!/bin/sh
set -eu

root=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
log="$tmp/calls"

cat >"$tmp/demo" <<'EOF'
#!/bin/sh
printf 'demo:%s:%s:%s\n' "$ENGINE" "$1" "$2" >>"$CALL_LOG"
if [ "$2" = destroy-apply ]; then
  test "${DEMO_DESTROY_RESULT:-success}" = success
fi
EOF
cat >"$tmp/acceptance" <<'EOF'
#!/bin/sh
printf 'acceptance%s\n' "${*:+:$*}" >>"$CALL_LOG"
test "${ACCEPTANCE_RESULT:-success}" = success
EOF
chmod +x "$tmp/demo" "$tmp/acceptance"

run() {
  CALL_LOG="$log" CAPABILITY_SHARD=free_properties HUBSPOT_ONE_PORTAL_LOCK_DIR="$tmp/lock" HUBSPOT_DEMO_SCRIPT="$tmp/demo" HUBSPOT_ACCEPTANCE_SCRIPT="$tmp/acceptance" "$root/scripts/one-portal-free-lifecycle.sh"
}

tofu_lifecycle_calls() {
  cat <<'EOF'
demo:tofu:local:adopt
demo:tofu:local:verify
demo:tofu:local:drift
demo:tofu:local:repair
demo:tofu:local:refresh
demo:tofu:local:adopt
demo:tofu:local:verify
demo:tofu:local:destroy-plan
demo:tofu:local:destroy-apply
EOF
}

terraform_lifecycle_calls() {
  cat <<'EOF'
demo:terraform:local:plan
demo:terraform:local:apply
demo:terraform:local:verify
demo:terraform:local:drift
demo:terraform:local:repair
demo:terraform:local:refresh
demo:terraform:local:adopt
demo:terraform:local:verify
demo:terraform:local:destroy-plan
demo:terraform:local:destroy-apply
EOF
}

restore_calls() {
  cat <<'EOF'
demo:tofu:local:plan
demo:tofu:local:apply
demo:tofu:local:verify
EOF
}

expected_lifecycle() {
  tofu_lifecycle_calls
  terraform_lifecycle_calls
  printf '%s\n' "$1"
  restore_calls
}

run
test "$(cat "$log")" = "$(expected_lifecycle acceptance)"

: >"$log"
if CALL_LOG="$log" CAPABILITY_SHARD=free_properties ACCEPTANCE_RESULT=failed HUBSPOT_ONE_PORTAL_LOCK_DIR="$tmp/lock" HUBSPOT_DEMO_SCRIPT="$tmp/demo" HUBSPOT_ACCEPTANCE_SCRIPT="$tmp/acceptance" "$root/scripts/one-portal-free-lifecycle.sh"; then
  echo "expected acceptance failure" >&2
  exit 1
fi
test "$(cat "$log")" = "$(expected_lifecycle acceptance)"

: >"$log"
CALL_LOG="$log" CAPABILITY_SHARD=free_properties HUBSPOT_ONE_PORTAL_LOCK_DIR="$tmp/lock" HUBSPOT_DEMO_SCRIPT="$tmp/demo" \
  "$root/scripts/one-portal-free-lifecycle.sh" "$tmp/acceptance" free_properties tofu
test "$(cat "$log")" = "$(expected_lifecycle 'acceptance:free_properties tofu')"

: >"$log"
if CALL_LOG="$log" CAPABILITY_SHARD=free_properties DEMO_DESTROY_RESULT=failed HUBSPOT_ONE_PORTAL_LOCK_DIR="$tmp/lock" HUBSPOT_DEMO_SCRIPT="$tmp/demo" HUBSPOT_ACCEPTANCE_SCRIPT="$tmp/acceptance" "$root/scripts/one-portal-free-lifecycle.sh"; then
  echo "expected demo destroy failure" >&2
  exit 1
fi
expected_destroy_failure=$(tofu_lifecycle_calls; restore_calls)
test "$(cat "$log")" = "$expected_destroy_failure"

mkdir "$tmp/lock"
if CALL_LOG="$log" CAPABILITY_SHARD=free_properties HUBSPOT_ONE_PORTAL_LOCK_DIR="$tmp/lock" HUBSPOT_DEMO_SCRIPT="$tmp/demo" HUBSPOT_ACCEPTANCE_SCRIPT="$tmp/acceptance" "$root/scripts/one-portal-free-lifecycle.sh"; then
  echo "expected concurrent lifecycle rejection" >&2
  exit 1
fi
