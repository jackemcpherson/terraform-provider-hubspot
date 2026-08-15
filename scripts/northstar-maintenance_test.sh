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

cat >"$tmp/helper" <<'EOF'
#!/bin/sh
set -eu
printf 'helper:%s\n' "$*" >>"$CALL_LOG"
test "${HELPER_RESULT:-passed}" = passed
EOF
chmod +x "$tmp/helper"

CALL_LOG="$log" HUBSPOT_DEMO_SCRIPT="$tmp/demo" \
	HUBSPOT_NORTHSTAR_HELPER_SCRIPT="$tmp/helper" \
	HUBSPOT_ONE_PORTAL_LOCK_DIR="$tmp/lock" \
	"$root/scripts/northstar-maintenance.sh"

test "$(sed -n '1p' "$log")" = 'helper:cleanup'
for engine in tofu terraform; do
  for phase in plan apply verify drift repair refresh adopt verify destroy-plan destroy-apply; do
    grep -Fqx "$engine:local:$phase" "$log"
  done
done
test "$(wc -l <"$log" | tr -d ' ')" -eq 21

mkdir "$tmp/busy-lock"
call_count=$(wc -l <"$log" | tr -d ' ')
if CALL_LOG="$log" HUBSPOT_DEMO_SCRIPT="$tmp/demo" \
	HUBSPOT_NORTHSTAR_HELPER_SCRIPT="$tmp/helper" \
	HUBSPOT_ONE_PORTAL_LOCK_DIR="$tmp/busy-lock" \
  "$root/scripts/northstar-maintenance.sh"; then
  echo 'maintenance accepted a concurrent portal lifecycle' >&2
  exit 1
fi
test "$(wc -l <"$log" | tr -d ' ')" -eq "$call_count"
rmdir "$tmp/busy-lock"

call_count=$(wc -l <"$log" | tr -d ' ')
if CALL_LOG="$log" HELPER_RESULT=failed HUBSPOT_DEMO_SCRIPT="$tmp/demo" \
	HUBSPOT_NORTHSTAR_HELPER_SCRIPT="$tmp/helper" \
	HUBSPOT_ONE_PORTAL_LOCK_DIR="$tmp/cleanup-failure-lock" \
	"$root/scripts/northstar-maintenance.sh"; then
	echo 'maintenance ignored a failed pre-run cleanup' >&2
	exit 1
fi
test "$(wc -l <"$log" | tr -d ' ')" -eq "$((call_count + 1))"
test ! -e "$tmp/cleanup-failure-lock" || {
	echo 'failed pre-run cleanup retained the portal lock' >&2
	exit 1
}

if CALL_LOG="$log" DEMO_RESULT=failed HUBSPOT_DEMO_SCRIPT="$tmp/demo" \
	HUBSPOT_NORTHSTAR_HELPER_SCRIPT="$tmp/helper" \
	HUBSPOT_ONE_PORTAL_LOCK_DIR="$tmp/failure-lock" \
  "$root/scripts/northstar-maintenance.sh"; then
  echo 'maintenance ignored a failed lifecycle phase' >&2
  exit 1
fi
test ! -e "$tmp/failure-lock" || {
  echo 'failed maintenance retained the portal lock' >&2
  exit 1
}
