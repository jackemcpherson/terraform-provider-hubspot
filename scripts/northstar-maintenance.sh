#!/bin/sh
set -eu

demo=${HUBSPOT_DEMO_SCRIPT:?HUBSPOT_DEMO_SCRIPT is required}
provider_repo=${HUBSPOT_PROVIDER_REPO:-$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)}
lock_dir=${HUBSPOT_ONE_PORTAL_LOCK_DIR:-"${TMPDIR:-/tmp}/hubspot-free-portal-${HUBSPOT_PORTAL_LOCK_ID:-free-configuration}.lock"}
test -x "$demo" || {
  echo 'Northstar demo script is not executable' >&2
  exit 1
}

mkdir "$lock_dir" 2>/dev/null || {
  echo "Northstar maintenance is already running: $lock_dir" >&2
  exit 1
}
trap 'rmdir "$lock_dir"' EXIT HUP INT TERM

run_demo_phase() {
  ENGINE=$1 HUBSPOT_PORTAL_LOCK_HELD=1 "$demo" local "$2"
}

run_northstar_helper() {
  if [ -n "${HUBSPOT_NORTHSTAR_HELPER_SCRIPT:-}" ]; then
    test -x "$HUBSPOT_NORTHSTAR_HELPER_SCRIPT" || {
      echo 'Northstar helper script is not executable' >&2
      exit 1
    }
    "$HUBSPOT_NORTHSTAR_HELPER_SCRIPT" "$@"
    return
  fi
  (
    cd "$provider_repo"
    GOTOOLCHAIN=local go run ./cmd/northstar-lifecycle "$@"
  )
}

run_northstar_helper cleanup

for engine in tofu terraform; do
  run_demo_phase "$engine" plan
  run_demo_phase "$engine" apply
  run_demo_phase "$engine" verify
  run_demo_phase "$engine" drift
  run_demo_phase "$engine" repair
  run_demo_phase "$engine" refresh
  run_demo_phase "$engine" adopt
  run_demo_phase "$engine" verify
  run_demo_phase "$engine" destroy-plan
  run_demo_phase "$engine" destroy-apply
done
