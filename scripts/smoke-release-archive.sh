#!/bin/sh
set -eu

assets=${1:?release assets directory is required}
version=${2:?release version is required}
case "$version" in v[0-9]*.[0-9]*.[0-9]*) ;; *) echo "version must be v-prefixed SemVer" >&2; exit 1 ;; esac

release_version=${version#v}
os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$(uname -m)" in arm64|aarch64) arch=arm64 ;; x86_64|amd64) arch=amd64 ;; *) echo "unsupported smoke architecture" >&2; exit 1 ;; esac
platform="${os}_${arch}"
archive=$(find "$assets" -type f -name "terraform-provider-hubspot_${release_version}_${platform}.zip" -print -quit)
test -n "$archive" || { echo "$platform release archive is missing" >&2; exit 1; }

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
unzip -q "$archive" -d "$tmp/unpacked"
binary=$(find "$tmp/unpacked" -type f -name 'terraform-provider-hubspot_v*' -print -quit)
test -n "$binary" || { echo "provider binary is missing from archive" >&2; exit 1; }

for address in registry.terraform.io/jackemcpherson/hubspot registry.opentofu.org/jackemcpherson/hubspot; do
  host=${address%%/*}
  mirror="$tmp/mirror/$host/jackemcpherson/hubspot/$release_version/$platform"
  mkdir -p "$mirror"
  cp "$binary" "$mirror/"
done

cat >"$tmp/cli.tfrc" <<EOF
provider_installation {
  filesystem_mirror {
    path    = "$tmp/mirror"
    include = ["registry.terraform.io/jackemcpherson/hubspot", "registry.opentofu.org/jackemcpherson/hubspot"]
  }
}
EOF

smoke() {
  engine=$1
  address=$2
  work="$tmp/$engine"
  mkdir -p "$work"
  cat >"$work/main.tf" <<EOF
terraform {
  required_providers {
    hubspot = {
      source  = "$address"
      version = "$release_version"
    }
  }
}
provider "hubspot" { access_token = "smoke-only" }
EOF
  TF_CLI_CONFIG_FILE="$tmp/cli.tfrc" "$engine" -chdir="$work" init -backend=false -input=false >/dev/null
  TF_CLI_CONFIG_FILE="$tmp/cli.tfrc" "$engine" -chdir="$work" providers schema -json | grep -q 'hubspot_property_group'
}

smoke terraform registry.terraform.io/jackemcpherson/hubspot
smoke tofu registry.opentofu.org/jackemcpherson/hubspot
