#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM

mkdir -p "$tmp/provider" "$tmp/bin"
cp "$root/examples/provider/main.tf" "$tmp/provider/main.tf"
CGO_ENABLED=0 GOTOOLCHAIN=local go build -trimpath -o "$tmp/bin/terraform-provider-hubspot" "$root"

cat >"$tmp/tofu.tfrc" <<EOF
provider_installation {
  dev_overrides {
    "registry.terraform.io/jackemcpherson/hubspot" = "$tmp/bin"
  }
  direct {}
}
EOF

if command -v tofu >/dev/null 2>&1; then
  tofu version | grep -F 'OpenTofu v1.12.3' >/dev/null
  TF_CLI_CONFIG_FILE="$tmp/tofu.tfrc" tofu -chdir="$tmp/provider" validate
else
  echo "OpenTofu is required for engine-smoke" >&2
  exit 1
fi

if command -v terraform >/dev/null 2>&1; then
  terraform version | grep -F 'Terraform v1.15.8' >/dev/null
  cat >"$tmp/terraform.tfrc" <<EOF
provider_installation {
  dev_overrides {
    "registry.terraform.io/jackemcpherson/hubspot" = "$tmp/bin"
  }
  direct {}
}
EOF
  TF_CLI_CONFIG_FILE="$tmp/terraform.tfrc" terraform -chdir="$tmp/provider" validate
else
  echo "Terraform is required for engine-smoke" >&2
  exit 1
fi
