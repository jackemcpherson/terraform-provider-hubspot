#!/bin/sh
set -eu

for workflow in .github/workflows/*.yml; do
  grep -q '^permissions: {}' "$workflow" || {
    echo "workflow $workflow must start with empty permissions" >&2
    exit 1
  }
  grep -q 'timeout-minutes:' "$workflow" || {
    echo "workflow $workflow has no finite timeout" >&2
    exit 1
  }
  grep -Eq '^[[:space:]]*runs-on: ubuntu-(24|26)\.04[[:space:]]*$' "$workflow" || {
    echo "workflow $workflow must pin the hosted runner image" >&2
    exit 1
  }
  ! grep -q 'ubuntu-latest' "$workflow" || {
    echo "workflow $workflow must not use ubuntu-latest" >&2
    exit 1
  }
  if grep -E 'uses: [^.]' "$workflow" | grep -Ev 'uses: [^@]+@[0-9a-f]{40}([[:space:]]+#.*)?$' >/dev/null; then
    echo "external action is not pinned to a full commit in $workflow" >&2
    exit 1
  fi
  ! grep -Eq 'pull_request_target|workflow_run|secrets:[[:space:]]*inherit|self-hosted|vars\.RUNNER_LABEL' "$workflow" || {
    echo "unsafe workflow boundary in $workflow" >&2
    exit 1
  }
  ! grep -Eq 'run:.*\$\{\{[[:space:]]*github\.' "$workflow" || {
    echo "untrusted event interpolation in $workflow" >&2
    exit 1
  }
  if grep -Eq '^[[:space:]]+- uses:' "$workflow"; then
    echo "every action step must have a descriptive name in $workflow" >&2
    exit 1
  fi
done

grep -Fq "TestAcc_product_definitions_ContractPreflight" Makefile || {
  echo "maintenance must run the guarded Product contract preflight" >&2
  exit 1
}

grep -Eq '^[[:space:]]+ref: [0-9a-f]{40}$' .github/workflows/provider-maintenance.yml || {
  echo 'provider maintenance must pin the Northstar demo to one exact commit' >&2
  exit 1
}

# Local checks and hosted workflows must qualify the same current engines. The
# Makefile owns the current pins; every workflow engine input must match them.
makefile_pin() {
  value=$(sed -n "s/^$1 := //p" Makefile)
  test -n "$value" || {
    echo "Makefile must define $1" >&2
    exit 1
  }
  echo "$value"
}
tofu_current=$(makefile_pin TOFU_CURRENT_VERSION)
terraform_current=$(makefile_pin TERRAFORM_CURRENT_VERSION)

check_engine_pins() {
  workflow=$1
  input=$2
  engine=$3
  expected=$4
  pins=$(sed -n "s/^[[:space:]]*$input:[[:space:]]*'\([^']*\)'[[:space:]]*\$/\1/p" "$workflow")
  for pin in $pins; do
    test "$pin" = "$expected" || {
      echo "workflow $workflow pins $engine $pin but the Makefile current pin is $expected" >&2
      exit 1
    }
  done
}

for workflow in .github/workflows/*.yml; do
  check_engine_pins "$workflow" tofu-version OpenTofu "$tofu_current"
  check_engine_pins "$workflow" terraform-version Terraform "$terraform_current"
done

# Removing a pin would let the setup action skip that engine, so the workflows
# that qualify current engines must keep both.
for workflow in .github/workflows/validate-provider.yml .github/workflows/provider-maintenance.yml; do
  grep -Fq "tofu-version: '$tofu_current'" "$workflow" || {
    echo "workflow $workflow must pin OpenTofu $tofu_current" >&2
    exit 1
  }
  grep -Fq "terraform-version: '$terraform_current'" "$workflow" || {
    echo "workflow $workflow must pin Terraform $terraform_current" >&2
    exit 1
  }
done

for action in .github/actions/*/action.yml; do
  if grep -E 'uses: [^.]' "$action" | grep -Ev 'uses: [^@]+@[0-9a-f]{40}([[:space:]]+#.*)?$' >/dev/null; then
    echo "external action is not pinned to a full commit in $action" >&2
    exit 1
  fi
done
