#!/bin/sh
set -eu

version=${1:?release version is required}
demo_script=${HUBSPOT_DEMO_SCRIPT:-../terraform-hubspot-demo/scripts/demo}

test "$version" = v0.2.0 || { echo "Northstar release journey requires v0.2.0" >&2; exit 1; }
test -x "$demo_script" || { echo "demo script is not executable: $demo_script" >&2; exit 1; }

run_engine() {
	engine=$1
	ENGINE=$engine "$demo_script" registry plan
	ENGINE=$engine "$demo_script" registry apply
	ENGINE=$engine "$demo_script" registry verify
}

run_engine terraform
run_engine tofu
