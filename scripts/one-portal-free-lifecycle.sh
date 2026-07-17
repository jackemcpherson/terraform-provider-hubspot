#!/bin/sh
set -eu

root=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
demo_script=${HUBSPOT_DEMO_SCRIPT:-"$root/../terraform-hubspot-demo/scripts/demo"}
acceptance_script=${HUBSPOT_ACCEPTANCE_SCRIPT:-"$root/scripts/acceptance-shard.sh"}
demo_torn_down=false

test -x "$demo_script" || { echo "demo script is not executable: $demo_script" >&2; exit 1; }
test -x "$acceptance_script" || { echo "acceptance script is not executable: $acceptance_script" >&2; exit 1; }
test "${CAPABILITY_SHARD:-}" = free_properties || { echo "one-portal lifecycle requires CAPABILITY_SHARD=free_properties" >&2; exit 1; }

restore_demo() {
  code=$?
  if [ "$demo_torn_down" = true ]; then
    "$demo_script" local plan >&2 || code=1
    "$demo_script" local apply >&2 || code=1
  fi
  exit "$code"
}
trap restore_demo EXIT HUP INT TERM

"$demo_script" local destroy-plan
"$demo_script" local destroy-apply
demo_torn_down=true
"$acceptance_script"
