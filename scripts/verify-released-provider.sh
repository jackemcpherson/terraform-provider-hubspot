#!/bin/sh
set -eu

engine=${1:?engine is required}
address=${2:?provider address is required}
version=${3:?release version is required}
assets=${4:?GitHub release assets directory is required}
command -v jq >/dev/null 2>&1 || { echo "jq is required" >&2; exit 1; }
case "$engine:$address" in
  terraform:registry.terraform.io/jackemcpherson/hubspot|tofu:registry.opentofu.org/jackemcpherson/hubspot) ;;
  *) echo "engine and registry address do not match" >&2; exit 1 ;;
esac
release_version=${version#v}

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
cat >"$tmp/main.tf" <<EOF
terraform {
  required_providers {
    hubspot = {
      source  = "$address"
      version = "$release_version"
    }
  }
}
provider "hubspot" { access_token = "schema-only" }
EOF

"$engine" -chdir="$tmp" init -backend=false -input=false >/dev/null
schema_json=$("$engine" -chdir="$tmp" providers schema -json)
printf '%s' "$schema_json" | jq -e '
  (.provider_schemas | length) == 1 and
  ((.provider_schemas | to_entries[0].value.resource_schemas | keys) == ["hubspot_property", "hubspot_property_group"]) and
  ((.provider_schemas | to_entries[0].value.data_source_schemas | keys) == ["hubspot_property_definition", "hubspot_property_definitions"])
' >/dev/null || { echo "released provider schema does not match the Free alpha surface" >&2; exit 1; }
grep -q "version = \"$release_version\"" "$tmp/.terraform.lock.hcl"
grep -q 'zh:' "$tmp/.terraform.lock.hcl"

os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$(uname -m)" in arm64|aarch64) arch=arm64 ;; x86_64|amd64) arch=amd64 ;; *) echo "unsupported verification architecture" >&2; exit 1 ;; esac
archive=$(find "$assets" -type f -name "terraform-provider-hubspot_${release_version}_${os}_${arch}.zip" -print -quit)
test -n "$archive" || { echo "matching GitHub archive is missing" >&2; exit 1; }
digest=$(shasum -a 256 "$archive" | awk '{print $1}')
grep -q "zh:$digest" "$tmp/.terraform.lock.hcl" || { echo "registry digest does not match the GitHub release" >&2; exit 1; }
