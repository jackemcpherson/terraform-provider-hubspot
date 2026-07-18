#!/bin/sh
set -eu

root=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
log="$tmp/calls"

cat >"$tmp/demo" <<'EOF'
#!/bin/sh
printf 'demo:%s:%s\n' "$1" "$2" >>"$CALL_LOG"
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

run
test "$(cat "$log")" = 'demo:local:adopt
demo:local:verify
demo:local:destroy-plan
demo:local:destroy-apply
acceptance
demo:local:plan
demo:local:apply
demo:local:verify'

: >"$log"
if CALL_LOG="$log" CAPABILITY_SHARD=free_properties ACCEPTANCE_RESULT=failed HUBSPOT_ONE_PORTAL_LOCK_DIR="$tmp/lock" HUBSPOT_DEMO_SCRIPT="$tmp/demo" HUBSPOT_ACCEPTANCE_SCRIPT="$tmp/acceptance" "$root/scripts/one-portal-free-lifecycle.sh"; then
  echo "expected acceptance failure" >&2
  exit 1
fi
test "$(cat "$log")" = 'demo:local:adopt
demo:local:verify
demo:local:destroy-plan
demo:local:destroy-apply
acceptance
demo:local:plan
demo:local:apply
demo:local:verify'

: >"$log"
CALL_LOG="$log" CAPABILITY_SHARD=free_properties HUBSPOT_ONE_PORTAL_LOCK_DIR="$tmp/lock" HUBSPOT_DEMO_SCRIPT="$tmp/demo" \
  "$root/scripts/one-portal-free-lifecycle.sh" "$tmp/acceptance" free_properties tofu
test "$(cat "$log")" = 'demo:local:adopt
demo:local:verify
demo:local:destroy-plan
demo:local:destroy-apply
acceptance:free_properties tofu
demo:local:plan
demo:local:apply
demo:local:verify'

: >"$log"
if CALL_LOG="$log" CAPABILITY_SHARD=free_properties DEMO_DESTROY_RESULT=failed HUBSPOT_ONE_PORTAL_LOCK_DIR="$tmp/lock" HUBSPOT_DEMO_SCRIPT="$tmp/demo" HUBSPOT_ACCEPTANCE_SCRIPT="$tmp/acceptance" "$root/scripts/one-portal-free-lifecycle.sh"; then
  echo "expected demo destroy failure" >&2
  exit 1
fi
test "$(cat "$log")" = 'demo:local:adopt
demo:local:verify
demo:local:destroy-plan
demo:local:destroy-apply
demo:local:plan
demo:local:apply
demo:local:verify'

mkdir "$tmp/lock"
if CALL_LOG="$log" CAPABILITY_SHARD=free_properties HUBSPOT_ONE_PORTAL_LOCK_DIR="$tmp/lock" HUBSPOT_DEMO_SCRIPT="$tmp/demo" HUBSPOT_ACCEPTANCE_SCRIPT="$tmp/acceptance" "$root/scripts/one-portal-free-lifecycle.sh"; then
  echo "expected concurrent lifecycle rejection" >&2
  exit 1
fi
