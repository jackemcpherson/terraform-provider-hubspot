#!/bin/sh
set -eu

root=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
demo_script=${HUBSPOT_DEMO_SCRIPT:-"$root/../terraform-hubspot-demo/scripts/demo"}
acceptance_script=${HUBSPOT_ACCEPTANCE_SCRIPT:-"$root/scripts/acceptance-shard.sh"}
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
    HUBSPOT_PORTAL_LOCK_HELD=1 "$demo_script" local plan >&2 || code=1
    HUBSPOT_PORTAL_LOCK_HELD=1 "$demo_script" local apply >&2 || code=1
    HUBSPOT_PORTAL_LOCK_HELD=1 "$demo_script" local verify >&2 || code=1
  fi
  if [ "$lock_acquired" = true ]; then rmdir "$lock_dir" || code=1; fi
  exit "$code"
}
trap restore_demo EXIT HUP INT TERM

HUBSPOT_PORTAL_LOCK_HELD=1 "$demo_script" local destroy-plan
HUBSPOT_PORTAL_LOCK_HELD=1 "$demo_script" local destroy-apply
demo_torn_down=true
HUBSPOT_PORTAL_LOCK_HELD=1 "$acceptance_script"
