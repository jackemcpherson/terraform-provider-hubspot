#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM

mkdir -p "$tmp/bin" "$tmp/examples"
for example in provider property-definition property-group property aliases; do
  cp -R "$root/examples/$example" "$tmp/examples/$example"
done
for example in hubspot_property_group hubspot_property; do
  mkdir -p "$tmp/examples/reference-$example"
  cp "$root/examples/resources/$example/resource.tf" "$tmp/examples/reference-$example/resource.tf"
  cp "$root/examples/provider/provider.tf" "$tmp/examples/reference-$example/provider.tf"
done
for example in hubspot_property_definition hubspot_property_definitions; do
  mkdir -p "$tmp/examples/reference-$example"
  cp "$root/examples/data-sources/$example/data-source.tf" "$tmp/examples/reference-$example/data-source.tf"
  cp "$root/examples/provider/provider.tf" "$tmp/examples/reference-$example/provider.tf"
done
CGO_ENABLED=0 GOTOOLCHAIN=local go build -trimpath -o "$tmp/bin/terraform-provider-hubspot" "$root"

cat >"$tmp/tofu.tfrc" <<EOF
provider_installation {
  dev_overrides {
    "registry.terraform.io/jackemcpherson/hubspot" = "$tmp/bin"
    "registry.opentofu.org/jackemcpherson/hubspot" = "$tmp/bin"
  }
  direct {}
}
EOF

examples="provider property-definition property-group property aliases reference-hubspot_property_group reference-hubspot_property reference-hubspot_property_definition reference-hubspot_property_definitions"

check_examples() {
  engine=$1
  cli_config=$2

  for example in $examples; do
    if [ "$example" = aliases ]; then
      TF_CLI_CONFIG_FILE="$cli_config" "$engine" -chdir="$tmp/examples/$example" get >/dev/null
    fi
    TF_CLI_CONFIG_FILE="$cli_config" "$engine" -chdir="$tmp/examples/$example" validate
    case "$example" in
      property-group|property|aliases|reference-hubspot_property_group|reference-hubspot_property)
        HUBSPOT_ACCESS_TOKEN=example TF_VAR_sandbox_hubspot_access_token=example TF_CLI_CONFIG_FILE="$cli_config" "$engine" -chdir="$tmp/examples/$example" plan -refresh=false -input=false -lock=false -out="$tmp/$engine-$example.plan" >/dev/null
        ;;
    esac
  done
}

engine=${ENGINE:-both}
tofu_expected=${TOFU_EXPECTED_VERSION:-1.12.3}
terraform_expected=${TERRAFORM_EXPECTED_VERSION:-1.15.8}
case "$engine" in both|tofu|terraform) ;; *) echo "ENGINE must be both, tofu, or terraform" >&2; exit 1 ;; esac

if [ "$engine" = both ] || [ "$engine" = tofu ]; then
  command -v tofu >/dev/null 2>&1 || { echo "OpenTofu is required for engine-smoke" >&2; exit 1; }
  tofu version | grep -F "OpenTofu v$tofu_expected" >/dev/null
  check_examples tofu "$tmp/tofu.tfrc"
fi

if [ "$engine" = both ] || [ "$engine" = terraform ]; then
  command -v terraform >/dev/null 2>&1 || { echo "Terraform is required for engine-smoke" >&2; exit 1; }
  terraform version | grep -F "Terraform v$terraform_expected" >/dev/null
  cat >"$tmp/terraform.tfrc" <<EOF
provider_installation {
  dev_overrides {
    "registry.terraform.io/jackemcpherson/hubspot" = "$tmp/bin"
    "registry.opentofu.org/jackemcpherson/hubspot" = "$tmp/bin"
  }
  direct {}
}
EOF
  check_examples terraform "$tmp/terraform.tfrc"
fi
