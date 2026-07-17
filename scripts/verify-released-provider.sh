#!/bin/sh
set -eu

engine=${1:?engine is required}
address=${2:?provider address is required}
version=${3:?release version is required}
assets=${4:?GitHub release assets directory is required}
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
schema=$("$engine" -chdir="$tmp" providers schema -json)
printf '%s' "$schema" | grep -q 'hubspot_property_group'
printf '%s' "$schema" | grep -q 'hubspot_property_definition'
if printf '%s' "$schema" | grep -Eq 'hubspot_pipeline|hubspot_custom_object_schema'; then
  echo "released provider exposes a deferred resource" >&2
  exit 1
fi
grep -q "version = \"$release_version\"" "$tmp/.terraform.lock.hcl"
grep -q 'zh:' "$tmp/.terraform.lock.hcl"

os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$(uname -m)" in arm64|aarch64) arch=arm64 ;; x86_64|amd64) arch=amd64 ;; *) echo "unsupported verification architecture" >&2; exit 1 ;; esac
archive=$(find "$assets" -type f -name "terraform-provider-hubspot_${release_version}_${os}_${arch}.zip" -print -quit)
test -n "$archive" || { echo "matching GitHub archive is missing" >&2; exit 1; }
digest=$(shasum -a 256 "$archive" | awk '{print $1}')
grep -q "zh:$digest" "$tmp/.terraform.lock.hcl" || { echo "registry digest does not match the GitHub release" >&2; exit 1; }
