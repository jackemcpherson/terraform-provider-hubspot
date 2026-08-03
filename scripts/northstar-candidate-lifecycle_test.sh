#!/bin/sh
set -eu

root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
log="$tmp/calls"

printf '%s\n' '#!/bin/sh' \
  'printf "%s:%s:%s\n" "$ENGINE" "$1" "$2" >>"$CALL_LOG"' \
  'test "${FAIL_PHASE:-none}" != "$ENGINE:$2"' >"$tmp/demo"
chmod +x "$tmp/demo"

run() {
  CALL_LOG="$log" HUBSPOT_DEMO_SCRIPT="$tmp/demo" HUBSPOT_ONE_PORTAL_LOCK_DIR="$tmp/lock" \
    "$root/scripts/northstar-candidate-lifecycle.sh"
}

run
for engine in tofu terraform; do
  for phase in plan apply verify drift repair refresh adopt verify destroy-plan destroy-apply; do
    printf '%s:%s:%s\n' "$engine" local "$phase"
  done
done >"$tmp/expected"
cmp "$log" "$tmp/expected"
test ! -e "$tmp/lock"

: >"$log"
if CALL_LOG="$log" FAIL_PHASE=tofu:repair HUBSPOT_DEMO_SCRIPT="$tmp/demo" HUBSPOT_ONE_PORTAL_LOCK_DIR="$tmp/lock" \
  "$root/scripts/northstar-candidate-lifecycle.sh"; then
  echo "Northstar lifecycle accepted a skipped or failed Forms phase" >&2
  exit 1
fi
test "$(tail -1 "$log")" = 'tofu:local:repair'
test ! -e "$tmp/lock"

mkdir "$tmp/lock"
if run; then
  echo "Northstar lifecycle accepted a shared portal lock collision" >&2
  exit 1
fi
