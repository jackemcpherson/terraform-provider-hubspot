#!/bin/sh
set -eu

version=${1:?release version is required}
prefix=${HUBSPOT_ACCEPTANCE_PREFIX:?HUBSPOT_ACCEPTANCE_PREFIX is required}
fixture=${RELEASED_FORM_FIXTURE_DIR:-acceptance/released/form_definitions}
evidence=${FORM_MIGRATION_EVIDENCE_FILE:-acceptance-report/released-form-migration.json}
: "${HUBSPOT_ACCESS_TOKEN:?HUBSPOT_ACCESS_TOKEN is required}"

test "$version" = v0.4.0 || { echo "released Form migration requires v0.4.0" >&2; exit 1; }
printf '%s\n' "$prefix" | grep -Eq '^tf_acc_[A-Za-z0-9_]+_$' || { echo "unsafe released Form prefix" >&2; exit 1; }
test -f "$fixture/main.tf.tmpl" || { echo "released Form fixture is missing" >&2; exit 1; }

terraform_source=registry.terraform.io/jackemcpherson/hubspot
tofu_source=registry.opentofu.org/jackemcpherson/hubspot
release_version=${version#v}
tmp=$(mktemp -d)
form_id=
active=false
current_engine=
cleanup() {
  code=$?
  if [ "$active" = true ] && [ -n "$form_id" ]; then
    run_helper archive "$form_id" "$prefix" >/dev/null 2>&1 || code=1
    run_helper verify-terminal "$form_id" "$prefix" >/dev/null 2>&1 || code=1
  fi
  rm -rf "$tmp"
  exit "$code"
}
trap cleanup EXIT
trap 'exit 1' HUP INT TERM

run_helper() {
  if [ -n "${RELEASED_FORM_HELPER_SCRIPT:-}" ]; then
    "$RELEASED_FORM_HELPER_SCRIPT" "$@"
  else
    GOTOOLCHAIN=local go run ./cmd/released-form-lifecycle "$@"
  fi
}

write_config() {
  source=$1
  sed -e "s|__PROVIDER_SOURCE__|$source|g" -e "s|__PROVIDER_VERSION__|$release_version|g" \
    "$fixture/main.tf.tmpl" >"$tmp/main.tf"
  rm -rf "$tmp/.terraform" "$tmp/.terraform.lock.hcl"
}

write_presentation() {
  presentation=$1
  printf 'presentation = "%s"\n' "$presentation" >"$tmp/journey.auto.tfvars"
}

init_engine() {
  current_engine=$1
  "$current_engine" -chdir="$tmp" init -input=false >/dev/null
}

require_empty_plan() {
  "$current_engine" -chdir="$tmp" plan -detailed-exitcode -input=false >/dev/null
}

read_form_id() {
  observed=$("$current_engine" -chdir="$tmp" output -raw released_form_id)
  printf '%s\n' "$observed" | grep -Eq '^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$' || {
    echo "released Form output did not contain one canonical generated ID" >&2
    exit 1
  }
  if [ -z "$form_id" ]; then
    form_id=$observed
  else
    test "$observed" = "$form_id" || { echo "released Form identity changed during state migration" >&2; exit 1; }
  fi
  run_helper verify-active "$form_id" "$prefix"
}

apply_presentation() {
  write_presentation "$1"
  "$current_engine" -chdir="$tmp" apply -auto-approve -input=false >/dev/null
  read_form_id
  require_empty_plan
}

drift_and_repair() {
  run_helper drift "$form_id" "$prefix"
  set +e
  "$current_engine" -chdir="$tmp" plan -detailed-exitcode -input=false >/dev/null 2>&1
  drift_code=$?
  set -e
  test "$drift_code" -eq 2 || { echo "$current_engine released Form drift was not detected" >&2; exit 1; }
  "$current_engine" -chdir="$tmp" apply -auto-approve -input=false >/dev/null
  read_form_id
  require_empty_plan
}

rm -f "$evidence"
export TF_VAR_hubspot_access_token="$HUBSPOT_ACCESS_TOKEN"
export TF_VAR_acceptance_prefix="$prefix"

write_config "$terraform_source"
write_presentation initial
init_engine terraform
"$current_engine" -chdir="$tmp" apply -auto-approve -input=false >/dev/null
active=true
read_form_id
require_empty_plan
apply_presentation terraform
drift_and_repair

cp "$tmp/terraform.tfstate" "$tmp/terraform-source.pre-migration.tfstate"
test -s "$tmp/terraform-source.pre-migration.tfstate"
terraform -chdir="$tmp" state replace-provider -auto-approve "$terraform_source" "$tofu_source" >/dev/null
write_config "$tofu_source"
init_engine tofu
read_form_id
require_empty_plan
apply_presentation tofu
drift_and_repair

cp "$tmp/terraform.tfstate" "$tmp/opentofu-source.pre-migration.tfstate"
test -s "$tmp/opentofu-source.pre-migration.tfstate"
tofu -chdir="$tmp" state replace-provider -auto-approve "$tofu_source" "$terraform_source" >/dev/null
write_config "$terraform_source"
init_engine terraform
read_form_id
require_empty_plan
apply_presentation terraform_final
drift_and_repair

terraform -chdir="$tmp" destroy -auto-approve -input=false >/dev/null
terminal_record=$(run_helper verify-terminal "$form_id" "$prefix")
printf '%s\n' "$terminal_record" | grep -q '"terminal":"archived"'
printf '%s\n' "$terminal_record" | grep -q '"active_owned_forms":0'
printf '%s\n' "$terminal_record" | grep -q '"cleanup":"passed"'
! printf '%s\n' "$terminal_record" | grep -Fq "$form_id" || { echo "released Form evidence exposed a raw identity" >&2; exit 1; }
active=false

mkdir -p "$(dirname "$evidence")"
completed_at=$(date -u '+%Y-%m-%dT%H:%M:%SZ')
printf '{"version":"%s","engines":["terraform","tofu"],"registry_sources":["%s","%s"],"state_migration":"passed","identity_preserved":true,"terminal_record":%s,"completed_at":"%s","cleanup":"passed","status":"passed"}\n' \
  "$version" "$terraform_source" "$tofu_source" "$terminal_record" "$completed_at" >"$evidence"
