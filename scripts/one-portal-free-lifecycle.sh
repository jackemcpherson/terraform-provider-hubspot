#!/bin/sh
set -eu

root=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
demo_script=${HUBSPOT_DEMO_SCRIPT:-"$root/../terraform-hubspot-demo/scripts/demo"}
if [ "$#" -gt 0 ]; then
  acceptance_script=$1
  shift
else
  acceptance_script=${HUBSPOT_ACCEPTANCE_SCRIPT:-"$root/scripts/acceptance-shard.sh"}
fi
lock_dir=${HUBSPOT_ONE_PORTAL_LOCK_DIR:-"${TMPDIR:-/tmp}/hubspot-free-portal-${HUBSPOT_PORTAL_LOCK_ID:-default}.lock"}
demo_torn_down=false
lock_acquired=false

test -x "$demo_script" || { echo "demo script is not executable: $demo_script" >&2; exit 1; }
test -x "$acceptance_script" || { echo "acceptance script is not executable: $acceptance_script" >&2; exit 1; }
test "${CAPABILITY_SHARD:-}" = free_properties || { echo "one-portal lifecycle requires CAPABILITY_SHARD=free_properties" >&2; exit 1; }
mkdir "$lock_dir" 2>/dev/null || { echo "one-portal lifecycle is already running: $lock_dir" >&2; exit 1; }
lock_acquired=true

restore_demo() {
  code=$?
  if [ "$demo_torn_down" = true ]; then
    ENGINE=tofu HUBSPOT_PORTAL_LOCK_HELD=1 "$demo_script" local plan >&2 || code=1
    ENGINE=tofu HUBSPOT_PORTAL_LOCK_HELD=1 "$demo_script" local apply >&2 || code=1
    ENGINE=tofu HUBSPOT_PORTAL_LOCK_HELD=1 "$demo_script" local verify >&2 || code=1
  fi
  if [ "$lock_acquired" = true ]; then rmdir "$lock_dir" || code=1; fi
  exit "$code"
}
trap restore_demo EXIT HUP INT TERM

run_demo() {
  ENGINE=$1 HUBSPOT_PORTAL_LOCK_HELD=1 "$demo_script" local "$2"
}

run_demo tofu adopt
run_demo tofu verify
run_demo tofu drift
run_demo tofu repair
run_demo tofu refresh
run_demo tofu adopt
run_demo tofu verify
run_demo tofu destroy-plan
demo_torn_down=true
run_demo tofu destroy-apply

run_demo terraform plan
run_demo terraform apply
run_demo terraform verify
run_demo terraform drift
run_demo terraform repair
run_demo terraform refresh
run_demo terraform adopt
run_demo terraform verify
run_demo terraform destroy-plan
run_demo terraform destroy-apply

HUBSPOT_PORTAL_LOCK_HELD=1 "$acceptance_script" "$@"
