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

CALL_LOG="$log" HUBSPOT_DEMO_SCRIPT="$tmp/demo" "$root/scripts/released-northstar-journey.sh" v0.3.0
test "$(cat "$log")" = 'terraform:registry:plan
terraform:registry:apply
terraform:registry:verify
terraform:registry:drift
terraform:registry:repair
terraform:registry:refresh
terraform:registry:adopt
terraform:registry:verify
terraform:registry:destroy-plan
terraform:registry:destroy-apply
tofu:registry:plan
tofu:registry:apply
tofu:registry:verify
tofu:registry:drift
tofu:registry:repair
tofu:registry:refresh
tofu:registry:adopt
tofu:registry:verify
tofu:registry:destroy-plan
tofu:registry:destroy-apply'

if CALL_LOG="$log" HUBSPOT_DEMO_SCRIPT="$tmp/demo" "$root/scripts/released-northstar-journey.sh" v0.2.0; then
	echo "expected release-version rejection" >&2
	exit 1
fi
