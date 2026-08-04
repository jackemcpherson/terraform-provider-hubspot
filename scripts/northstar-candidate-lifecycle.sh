#!/bin/sh
set -eu

root=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
version=${1:?v-prefixed candidate version is required}
demo=${HUBSPOT_DEMO_SCRIPT:?HUBSPOT_DEMO_SCRIPT is required}
lock_dir=${HUBSPOT_ONE_PORTAL_LOCK_DIR:-"${TMPDIR:-/tmp}/hubspot-free-portal-${HUBSPOT_PORTAL_LOCK_ID:-free-configuration}.lock"}
test -x "$demo" || { echo "Northstar demo script is not executable" >&2; exit 1; }
demo_root=${HUBSPOT_DEMO_REPO:-"$(CDPATH='' cd -- "$(dirname -- "$demo")/.." && pwd)"}
"$root/scripts/validate-candidate-compatibility.sh" "$version" "$demo_root"
mkdir "$lock_dir" 2>/dev/null || { echo "Northstar portal lifecycle is already running: $lock_dir" >&2; exit 1; }
trap 'rmdir "$lock_dir"' EXIT HUP INT TERM

run() {
  ENGINE=$1 HUBSPOT_PORTAL_LOCK_HELD=1 "$demo" local "$2"
}

for engine in tofu terraform; do
  run "$engine" plan
  run "$engine" apply
  run "$engine" verify
  run "$engine" drift
  run "$engine" repair
  run "$engine" refresh
  run "$engine" adopt
  run "$engine" verify
  run "$engine" destroy-plan
  run "$engine" destroy-apply
done
