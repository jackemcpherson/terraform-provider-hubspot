#!/bin/sh
set -eu

shard=${1:?capability shard is required}
engine=${2:?engine is required}
address=${3:?provider address is required}
version=${4:?release version is required}
fixture="acceptance/released/$shard"

case "$shard" in free_properties|deal_pipelines|ticket_pipelines|custom_schemas|sensitive_properties|custom_pipelines) ;; *) echo "unknown capability shard" >&2; exit 1 ;; esac
case "$engine:$address" in
  terraform:registry.terraform.io/jackemcpherson/hubspot|tofu:registry.opentofu.org/jackemcpherson/hubspot) ;;
  *) echo "engine and registry address do not match" >&2; exit 1 ;;
esac
: "${HUBSPOT_ACCESS_TOKEN:?HUBSPOT_ACCESS_TOKEN is required}"
test -f "$fixture/main.tf.tmpl" || { echo "released-artifact fixture is missing for $shard" >&2; exit 1; }

tmp=$(mktemp -d)
active=false
cleanup() {
  code=$?
  if [ "$active" = true ]; then
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

"$engine" -chdir="$tmp" init -input=false >/dev/null
active=true
"$engine" -chdir="$tmp" apply -auto-approve -input=false >/dev/null
"$engine" -chdir="$tmp" plan -detailed-exitcode -input=false >/dev/null
"$engine" -chdir="$tmp" destroy -auto-approve -input=false >/dev/null
active=false
