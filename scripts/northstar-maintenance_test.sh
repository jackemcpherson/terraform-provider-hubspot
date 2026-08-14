#!/bin/sh
set -eu

root=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
log="$tmp/calls"

cat >"$tmp/demo" <<'EOF'
#!/bin/sh
set -eu
printf '%s:%s:%s\n' "$ENGINE" "$1" "$2" >>"$CALL_LOG"
test "${DEMO_RESULT:-passed}" = passed
EOF
chmod +x "$tmp/demo"

CALL_LOG="$log" HUBSPOT_DEMO_SCRIPT="$tmp/demo" \
  HUBSPOT_ONE_PORTAL_LOCK_DIR="$tmp/lock" \
  "$root/scripts/northstar-maintenance.sh"

for engine in tofu terraform; do
  for phase in plan apply verify drift repair refresh adopt verify destroy-plan destroy-apply; do
    grep -Fqx "$engine:local:$phase" "$log"
  done
done
test "$(wc -l <"$log" | tr -d ' ')" -eq 20

mkdir "$tmp/busy-lock"
if CALL_LOG="$log" HUBSPOT_DEMO_SCRIPT="$tmp/demo" \
  HUBSPOT_ONE_PORTAL_LOCK_DIR="$tmp/busy-lock" \
  "$root/scripts/northstar-maintenance.sh"; then
  echo 'maintenance accepted a concurrent portal lifecycle' >&2
  exit 1
fi
rmdir "$tmp/busy-lock"

if CALL_LOG="$log" DEMO_RESULT=failed HUBSPOT_DEMO_SCRIPT="$tmp/demo" \
  HUBSPOT_ONE_PORTAL_LOCK_DIR="$tmp/failure-lock" \
  "$root/scripts/northstar-maintenance.sh"; then
  echo 'maintenance ignored a failed lifecycle phase' >&2
  exit 1
fi
test ! -e "$tmp/failure-lock" || {
  echo 'failed maintenance retained the portal lock' >&2
  exit 1
}
