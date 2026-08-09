#!/bin/sh
set -eu

version=${1:?release version is required}
prefix=${HUBSPOT_ACCEPTANCE_PREFIX:?HUBSPOT_ACCEPTANCE_PREFIX is required}
fixture=${RELEASED_FILES_FIXTURE_DIR:-acceptance/released/files_configuration}
evidence=${FILES_MIGRATION_EVIDENCE_FILE:-acceptance-report/released-files-migration.json}
: "${HUBSPOT_ACCESS_TOKEN:?HUBSPOT_ACCESS_TOKEN is required}"

test "$version" = v0.4.0 || { echo "released Files migration requires v0.4.0" >&2; exit 1; }
printf '%s\n' "$prefix" | grep -Eq '^tf_acc_[A-Za-z0-9_]+_$' || { echo "unsafe released Files prefix" >&2; exit 1; }
test -f "$fixture/main.tf.tmpl" || { echo "released Files fixture is missing" >&2; exit 1; }
test -f "$fixture/before.txt" && test -f "$fixture/after.txt" || { echo "released Files byte fixtures are missing" >&2; exit 1; }
fixture=$(CDPATH='' cd -- "$fixture" && pwd)

before_sha=12cb397ead6584ef487cbfb0e9663d5d125537bf689b0c69c716ef49ed718890
after_sha=335fdaad53f1eca8f4fe0a94f706aa753eabcc60ced61786f43b542176979b71
test "$(shasum -a 256 "$fixture/before.txt" | awk '{print $1}')" = "$before_sha" || { echo "released Files before fixture digest changed" >&2; exit 1; }
test "$(shasum -a 256 "$fixture/after.txt" | awk '{print $1}')" = "$after_sha" || { echo "released Files after fixture digest changed" >&2; exit 1; }

terraform_source=registry.terraform.io/jackemcpherson/hubspot
tofu_source=registry.opentofu.org/jackemcpherson/hubspot
release_version=${version#v}
tmp=$(mktemp -d)
root_folder_id=
leaf_folder_id=
file_id=
active=false
current_engine=
desired_file_name=
desired_file_access=
desired_file_md5=
desired_file_size=

run_helper() {
  if [ -n "${RELEASED_FILES_HELPER_SCRIPT:-}" ]; then
    "$RELEASED_FILES_HELPER_SCRIPT" "$@"
  else
    GOTOOLCHAIN=local go run ./cmd/released-files-lifecycle "$@"
  fi
}

cleanup() {
  code=$?
  if [ "$active" = true ]; then
    if [ -n "$root_folder_id" ] && [ -n "$leaf_folder_id" ] && [ -n "$file_id" ]; then
      run_helper cleanup "$root_folder_id" "$leaf_folder_id" "$file_id" "$prefix" >/dev/null 2>&1 || code=1
      run_helper verify-terminal "$root_folder_id" "$leaf_folder_id" "$file_id" "$prefix" >/dev/null 2>&1 || code=1
    elif [ -n "$current_engine" ]; then
      "$current_engine" -chdir="$tmp" destroy -auto-approve -input=false >/dev/null 2>&1 || code=1
    else
      code=1
    fi
  fi
  rm -rf "$tmp"
  exit "$code"
}
trap cleanup EXIT
trap 'exit 1' HUP INT TERM

write_config() {
  source=$1
  sed -e "s|__PROVIDER_SOURCE__|$source|g" -e "s|__PROVIDER_VERSION__|$release_version|g" \
    "$fixture/main.tf.tmpl" >"$tmp/main.tf"
  rm -rf "$tmp/.terraform" "$tmp/.terraform.lock.hcl"
}

write_desired() {
  suffix=$1
  access=$2
  asset=$3
  sha=$4
  printf 'file_suffix = "%s"\nfile_access = "%s"\nsource_path = "%s"\nsource_sha256 = "%s"\n' \
    "$suffix" "$access" "$fixture/$asset" "$sha" >"$tmp/journey.auto.tfvars"
  desired_file_name="$prefix$suffix"
  desired_file_access=$access
  if command -v md5sum >/dev/null 2>&1; then
    desired_file_md5=$(md5sum "$fixture/$asset" | awk '{print $1}')
  else
    desired_file_md5=$(md5 -q "$fixture/$asset")
  fi
  desired_file_size=$(wc -c <"$fixture/$asset" | tr -d ' ')
}

init_engine() {
  current_engine=$1
  "$current_engine" -chdir="$tmp" init -input=false >/dev/null
}

require_empty_plan() {
  "$current_engine" -chdir="$tmp" plan -detailed-exitcode -input=false >/dev/null
}

read_ids() {
  observed_root=$("$current_engine" -chdir="$tmp" output -raw released_root_folder_id)
  observed_leaf=$("$current_engine" -chdir="$tmp" output -raw released_leaf_folder_id)
  observed_file=$("$current_engine" -chdir="$tmp" output -raw released_file_id)
  for observed in "$observed_root" "$observed_leaf" "$observed_file"; do
    printf '%s\n' "$observed" | grep -Eq '^[1-9][0-9]*$' || { echo "released Files output did not contain canonical generated IDs" >&2; exit 1; }
  done
  if [ -z "$root_folder_id" ]; then
    root_folder_id=$observed_root
    leaf_folder_id=$observed_leaf
    file_id=$observed_file
  else
    test "$observed_root" = "$root_folder_id" && test "$observed_leaf" = "$leaf_folder_id" && test "$observed_file" = "$file_id" || {
      echo "released Files identity changed during state migration" >&2
      exit 1
    }
  fi
  run_helper verify-active "$root_folder_id" "$leaf_folder_id" "$file_id" "$prefix" \
    "$desired_file_name" "$desired_file_access" "$desired_file_md5" "$desired_file_size"
}

apply_desired() {
  write_desired "$1" "$2" "$3" "$4"
  "$current_engine" -chdir="$tmp" plan -input=false -out=reviewed.tfplan >/dev/null
  active=true
  "$current_engine" -chdir="$tmp" apply -input=false reviewed.tfplan >/dev/null
  read_ids
  require_empty_plan
}

drift_and_repair() {
  run_helper drift "$root_folder_id" "$leaf_folder_id" "$file_id" "$prefix" \
    "$desired_file_name" "$desired_file_access" "$desired_file_md5" "$desired_file_size"
  set +e
  "$current_engine" -chdir="$tmp" plan -detailed-exitcode -input=false -out=reviewed.tfplan >/dev/null 2>&1
  drift_code=$?
  set -e
  test "$drift_code" -eq 2 || { echo "$current_engine released Files drift was not detected" >&2; exit 1; }
  "$current_engine" -chdir="$tmp" apply -input=false reviewed.tfplan >/dev/null
  read_ids
  require_empty_plan
}

rm -f "$evidence"
export TF_VAR_hubspot_access_token="$HUBSPOT_ACCESS_TOKEN"
export TF_VAR_acceptance_prefix="$prefix"

write_config "$terraform_source"
init_engine terraform
apply_desired released_file.txt PRIVATE before.txt "$before_sha"
apply_desired released_file_terraform.txt PUBLIC_NOT_INDEXABLE after.txt "$after_sha"
drift_and_repair

terraform -chdir="$tmp" state replace-provider -auto-approve "$terraform_source" "$tofu_source" >/dev/null
write_config "$tofu_source"
init_engine tofu
read_ids
require_empty_plan
apply_desired released_file_tofu.txt PRIVATE before.txt "$before_sha"
drift_and_repair

tofu -chdir="$tmp" state replace-provider -auto-approve "$tofu_source" "$terraform_source" >/dev/null
write_config "$terraform_source"
init_engine terraform
read_ids
require_empty_plan
apply_desired released_file_final.txt PUBLIC_NOT_INDEXABLE after.txt "$after_sha"
drift_and_repair

terraform -chdir="$tmp" plan -destroy -input=false -out=reviewed.tfplan >/dev/null
terraform -chdir="$tmp" apply -input=false reviewed.tfplan >/dev/null
terminal_record=$(run_helper verify-terminal "$root_folder_id" "$leaf_folder_id" "$file_id" "$prefix")
printf '%s\n' "$terminal_record" | grep -q '"active_owned_files":0'
printf '%s\n' "$terminal_record" | grep -q '"active_owned_folders":0'
printf '%s\n' "$terminal_record" | grep -q '"cleanup":"passed"'
! printf '%s\n' "$terminal_record" | grep -Eq "$root_folder_id|$leaf_folder_id|$file_id" || { echo "released Files evidence exposed a raw identity" >&2; exit 1; }
active=false

mkdir -p "$(dirname "$evidence")"
completed_at=$(date -u '+%Y-%m-%dT%H:%M:%SZ')
printf '{"version":"%s","engines":["terraform","tofu"],"registry_sources":["%s","%s"],"state_migration":"passed","identity_preserved":true,"metadata_updates":"passed","byte_replacements":"passed","drift_repair":"passed","terminal_record":%s,"completed_at":"%s","cleanup":"passed","status":"passed"}\n' \
  "$version" "$terraform_source" "$tofu_source" "$terminal_record" "$completed_at" >"$evidence"
