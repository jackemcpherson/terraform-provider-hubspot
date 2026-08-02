#!/bin/sh
set -eu

root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
log="$tmp/calls"

cat >"$tmp/demo" <<'EOF'
#!/bin/sh
printf '%s:%s:%s\n' "$ENGINE" "$1" "$2" >>"$CALL_LOG"
EOF
chmod +x "$tmp/demo"

CALL_LOG="$log" HUBSPOT_DEMO_SCRIPT="$tmp/demo" "$root/scripts/released-northstar-journey.sh" v0.2.0
test "$(cat "$log")" = 'terraform:registry:plan
terraform:registry:apply
terraform:registry:verify
tofu:registry:plan
tofu:registry:apply
tofu:registry:verify'

if CALL_LOG="$log" HUBSPOT_DEMO_SCRIPT="$tmp/demo" "$root/scripts/released-northstar-journey.sh" v0.2.1; then
	echo "expected release-version rejection" >&2
	exit 1
fi
