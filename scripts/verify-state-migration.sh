#!/bin/sh
set -eu

version=${1:?release version is required}
prefix=${HUBSPOT_ACCEPTANCE_PREFIX:?acceptance prefix is required}
: "${HUBSPOT_ACCESS_TOKEN:?HUBSPOT_ACCESS_TOKEN is required}"
release_version=${version#v}
printf '%s\n' "$prefix" | grep -Eq '^tf_acc_[A-Za-z0-9_]+_$' || { echo "unsafe migration prefix" >&2; exit 1; }

tmp=$(mktemp -d)
active_engine=
cleanup() {
  code=$?
  if [ -n "$active_engine" ] && [ -f "$tmp/main.tf" ]; then
    "$active_engine" -chdir="$tmp" destroy -auto-approve -input=false >/dev/null 2>&1 || code=1
  fi
  rm -rf "$tmp"
  exit "$code"
}
trap cleanup EXIT
trap 'exit 1' HUP INT TERM

write_config() {
  address=$1
  name=$2
  cat >"$tmp/main.tf" <<EOF
terraform {
  required_providers {
    hubspot = {
      source  = "$address"
      version = "$release_version"
    }
  }
}
provider "hubspot" {}
resource "hubspot_property_group" "migration" {
  object_type = "contacts"
  name        = "$name"
  label       = "$name"
}
EOF
  rm -rf "$tmp/.terraform" "$tmp/.terraform.lock.hcl"
}

migrate() {
  from_engine=$1
  from_address=$2
  to_engine=$3
  to_address=$4
  name=$5
  write_config "$from_address" "$name"
  active_engine=$from_engine
  "$from_engine" -chdir="$tmp" init -input=false >/dev/null
  "$from_engine" -chdir="$tmp" apply -auto-approve -input=false >/dev/null
  cp "$tmp/terraform.tfstate" "$tmp/${name}.pre-migration.tfstate"
  test -s "$tmp/${name}.pre-migration.tfstate"
  "$from_engine" -chdir="$tmp" state replace-provider -auto-approve "$from_address" "$to_address" >/dev/null
  write_config "$to_address" "$name"
  active_engine=$to_engine
  "$to_engine" -chdir="$tmp" init -input=false >/dev/null
  "$to_engine" -chdir="$tmp" plan -detailed-exitcode -input=false >/dev/null
  "$to_engine" -chdir="$tmp" destroy -auto-approve -input=false >/dev/null
  active_engine=
  rm -f "$tmp/terraform.tfstate" "$tmp/terraform.tfstate.backup"
}

migrate terraform registry.terraform.io/jackemcpherson/hubspot tofu registry.opentofu.org/jackemcpherson/hubspot "${prefix}tf_to_tofu"
migrate tofu registry.opentofu.org/jackemcpherson/hubspot terraform registry.terraform.io/jackemcpherson/hubspot "${prefix}tofu_to_tf"
