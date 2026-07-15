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

if [ "$shard" = free_properties ] || [ "$shard" = deal_pipelines ] || [ "$shard" = ticket_pipelines ]; then
  export CAPABILITY_SHARD=$shard
  export HUBSPOT_ACCEPTANCE=1
  go test -tags=acceptance ./internal/acceptance -run "^TestAcc_${shard}_QuotaPreflight$" -count=1 -timeout=5m
fi

"$engine" -chdir="$tmp" init -input=false >/dev/null
active=true
"$engine" -chdir="$tmp" apply -auto-approve -input=false >/dev/null
"$engine" -chdir="$tmp" plan -detailed-exitcode -input=false >/dev/null

if [ "$shard" = free_properties ]; then
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
fi

if [ "$shard" = deal_pipelines ] || [ "$shard" = ticket_pipelines ]; then
  command -v jq >/dev/null 2>&1 || { echo "jq is required for pipeline released verification" >&2; exit 1; }

  if [ "$shard" = deal_pipelines ]; then
    pipeline_object_type=deals
    drift_test=TestReleasedDealPipelineDrift
  else
    pipeline_object_type=tickets
    drift_test=TestReleasedTicketPipelineDrift
  fi
  state_backup="$tmp/pre-import.tfstate"
  "$engine" -chdir="$tmp" state pull >"$state_backup"
  state_json=$("$engine" -chdir="$tmp" show -json)
  pipeline_id=$(printf '%s' "$state_json" | jq -er '.values.root_module.resources[] | select(.address == "hubspot_pipeline.released") | .values.id')
  open_stage_id=$(printf '%s' "$state_json" | jq -er '.values.root_module.resources[] | select(.address == "hubspot_pipeline.released") | .values.stages.open.id')
  closed_stage_id=$(printf '%s' "$state_json" | jq -er '.values.root_module.resources[] | select(.address == "hubspot_pipeline.released") | .values.stages.closed.id')
  export TF_VAR_open_stage_key=$open_stage_id
  export TF_VAR_closed_stage_key=$closed_stage_id
  export HUBSPOT_RELEASED_PIPELINE_ID=${pipeline_id#"$pipeline_object_type"/}
  export HUBSPOT_RELEASED_STAGE_ID=$open_stage_id
  "$engine" -chdir="$tmp" state rm hubspot_pipeline.released >/dev/null
  "$engine" -chdir="$tmp" import -input=false hubspot_pipeline.released "$pipeline_id" >/dev/null
  "$engine" -chdir="$tmp" plan -detailed-exitcode -input=false >/dev/null

  go test -tags=acceptance ./internal/acceptance -run "^${drift_test}$" -count=1 -timeout=5m
  set +e
  "$engine" -chdir="$tmp" plan -detailed-exitcode -input=false >/dev/null 2>&1
  drift_code=$?
  set -e
  test "$drift_code" -eq 2 || { echo "released deal-pipeline drift phase did not detect a change" >&2; exit 1; }
  "$engine" -chdir="$tmp" apply -auto-approve -input=false >/dev/null
  "$engine" -chdir="$tmp" plan -detailed-exitcode -input=false >/dev/null
fi

"$engine" -chdir="$tmp" destroy -auto-approve -input=false >/dev/null
if [ "$shard" = free_properties ]; then
  go test -tags=acceptance ./internal/acceptance -run '^TestReleasedFreePropertiesAbsence$' -count=1 -timeout=5m
fi
if [ "$shard" = deal_pipelines ]; then
  go test -tags=acceptance ./internal/acceptance -run '^TestReleasedDealPipelineArchived$' -count=1 -timeout=5m
fi
if [ "$shard" = ticket_pipelines ]; then
  go test -tags=acceptance ./internal/acceptance -run '^TestReleasedTicketPipelineArchived$' -count=1 -timeout=5m
fi
active=false
