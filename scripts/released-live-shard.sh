#!/bin/sh
set -eu

shard=${1:?capability shard is required}
engine=${2:?engine is required}
address=${3:?provider address is required}
version=${4:?release version is required}
fixture="acceptance/released/$shard"

test "$shard" = free_properties || { echo "v0.1 supports only the free_properties capability shard" >&2; exit 1; }
case "$engine:$address" in
  terraform:registry.terraform.io/jackemcpherson/hubspot|tofu:registry.opentofu.org/jackemcpherson/hubspot) ;;
  *) echo "engine and registry address do not match" >&2; exit 1 ;;
esac
: "${HUBSPOT_ACCESS_TOKEN:?HUBSPOT_ACCESS_TOKEN is required}"
test -f "$fixture/main.tf.tmpl" || { echo "released-artifact fixture is missing for $shard" >&2; exit 1; }

tmp=$(mktemp -d)
active=false
state_backup=
cleanup() {
  code=$?
  if [ "$active" = true ]; then
    if [ -n "$state_backup" ] && test -s "$state_backup"; then
      "$engine" -chdir="$tmp" state push -force "$state_backup" >/dev/null 2>&1 || code=1
    fi
    "$engine" -chdir="$tmp" destroy -auto-approve -input=false >/dev/null 2>&1 || code=1
  fi
  rm -rf "$tmp"
  exit "$code"
}
trap cleanup EXIT
trap 'exit 1' HUP INT TERM
cp -R "$fixture/." "$tmp/"
release_version=${version#v}
sed -e "s|__PROVIDER_SOURCE__|$address|g" -e "s|__PROVIDER_VERSION__|$release_version|g" "$tmp/main.tf.tmpl" >"$tmp/main.tf"
rm "$tmp/main.tf.tmpl"
export TF_VAR_hubspot_access_token=$HUBSPOT_ACCESS_TOKEN
export TF_VAR_acceptance_prefix=${HUBSPOT_ACCEPTANCE_PREFIX:?acceptance prefix is required}

export CAPABILITY_SHARD=$shard
export HUBSPOT_ACCEPTANCE=1
go test -tags=acceptance ./internal/acceptance -run '^TestAcc_free_properties_QuotaPreflight$' -count=1 -timeout=5m

"$engine" -chdir="$tmp" init -input=false >/dev/null
active=true
"$engine" -chdir="$tmp" apply -auto-approve -input=false >/dev/null
"$engine" -chdir="$tmp" plan -detailed-exitcode -input=false >/dev/null

state_backup="$tmp/pre-import.tfstate"
"$engine" -chdir="$tmp" state pull >"$state_backup"
"$engine" -chdir="$tmp" state rm hubspot_property_group.released hubspot_property.scalar hubspot_property.enumeration >/dev/null
"$engine" -chdir="$tmp" import -input=false hubspot_property_group.released "contacts/${HUBSPOT_ACCEPTANCE_PREFIX}released_group" >/dev/null
"$engine" -chdir="$tmp" import -input=false hubspot_property.scalar "contacts/${HUBSPOT_ACCEPTANCE_PREFIX}released_scalar" >/dev/null
"$engine" -chdir="$tmp" import -input=false hubspot_property.enumeration "contacts/${HUBSPOT_ACCEPTANCE_PREFIX}released_enumeration" >/dev/null
"$engine" -chdir="$tmp" plan -detailed-exitcode -input=false >/dev/null

go test -tags=acceptance ./internal/acceptance -run '^TestReleasedFreePropertiesDrift$' -count=1 -timeout=5m
set +e
"$engine" -chdir="$tmp" plan -detailed-exitcode -input=false >/dev/null 2>&1
drift_code=$?
set -e
test "$drift_code" -eq 2 || { echo "released-provider drift phase did not detect a change" >&2; exit 1; }
"$engine" -chdir="$tmp" apply -auto-approve -input=false >/dev/null
"$engine" -chdir="$tmp" plan -detailed-exitcode -input=false >/dev/null

"$engine" -chdir="$tmp" destroy -auto-approve -input=false >/dev/null
go test -tags=acceptance ./internal/acceptance -run '^TestReleasedFreePropertiesAbsence$' -count=1 -timeout=5m
active=false
