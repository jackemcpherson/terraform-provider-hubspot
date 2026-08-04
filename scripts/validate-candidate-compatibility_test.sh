#!/bin/sh
set -eu

root=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
valid_hash='h1:0WnQNSWX/OwYe1pbEN8Kla8ej2VV90oP7vLcBj6SZ7A='

write_constraint() {
	file=$1
	constraint=$2
	mkdir -p "$(dirname -- "$file")"
	cat >"$file" <<EOF
terraform {
  required_providers {
    hubspot = {
      source  = "jackemcpherson/hubspot"
      version = "$constraint"
    }
  }
}
EOF
}

write_lock() {
	demo=$1
	engine=$2
	selected_version=$3
	constraint=$4
	hash=$5
	case "$engine" in
		tofu) address=registry.opentofu.org/jackemcpherson/hubspot ;;
		terraform) address=registry.terraform.io/jackemcpherson/hubspot ;;
		*) echo "unknown fixture engine: $engine" >&2; exit 1 ;;
	esac
	lock="$demo/locks/$engine/.terraform.lock.hcl"
	mkdir -p "$(dirname -- "$lock")"
	cat >"$lock" <<EOF
provider "$address" {
  version     = "$selected_version"
  constraints = "$constraint"
  hashes = [
    "$hash",
  ]
}
EOF
}

new_fixture() {
	name=$1
	selected_version=$2
	demo="$tmp/$name"
	mkdir -p "$demo/modules/first" "$demo/modules/nested"
	write_constraint "$demo/versions.tf" '>= 0.4.0, < 0.5.0'
	cat >"$demo/main.tf" <<'EOF'
module "first" {
  source = "./modules/first"
}
EOF
	write_constraint "$demo/modules/first/versions.tf" '~> 0.4.0'
	cat >"$demo/modules/first/main.tf" <<'EOF'
module "nested" {
  source = "../nested"
}
EOF
	write_constraint "$demo/modules/nested/versions.tf" '>= 0.3.0, < 0.6.0'
	write_lock "$demo" tofu "$selected_version" '>= 0.4.0, < 0.5.0' "$valid_hash"
	write_lock "$demo" terraform "$selected_version" '>= 0.4.0, < 0.5.0' "$valid_hash"
}

expect_success() {
	description=$1
	version=$2
	demo=$3
	if ! "$root/scripts/validate-candidate-compatibility.sh" "$version" "$demo" >"$tmp/output" 2>&1; then
		echo "candidate compatibility rejected $description" >&2
		cat "$tmp/output" >&2
		exit 1
	fi
	grep -Fq "Candidate compatibility passed for $version" "$tmp/output"
}

expect_failure() {
	description=$1
	version=$2
	demo=$3
	shift 3
	if "$root/scripts/validate-candidate-compatibility.sh" "$version" "$demo" >"$tmp/output" 2>&1; then
		echo "candidate compatibility accepted $description" >&2
		exit 1
	fi
	for expected in "$@"; do
		grep -Fq "$expected" "$tmp/output" || {
			echo "$description did not identify: $expected" >&2
			cat "$tmp/output" >&2
			exit 1
		}
	done
}

new_fixture exact-success 0.4.0
expect_success 'an exact candidate and recursively discovered modules' v0.4.0 "$demo"

new_fixture later-patch 0.4.7
expect_success 'a later compatible patch candidate' v0.4.7 "$demo"

new_fixture stale-v030-constraint 0.4.0
write_constraint "$demo/modules/nested/versions.tf" '>= 0.3.0, < 0.4.0'
expect_failure 'the stale v0.3.0-era constraint' v0.4.0 "$demo" \
	'modules/nested/versions.tf' '>= 0.3.0, < 0.4.0' 'v0.4.0'

new_fixture malformed-constraint 0.4.0
write_constraint "$demo/modules/first/versions.tf" '>= definitely-not-a-version'
expect_failure 'a malformed provider constraint' v0.4.0 "$demo" \
	'modules/first/versions.tf' '>= definitely-not-a-version' 'v0.4.0'

new_fixture missing-module 0.4.0
cat >"$demo/main.tf" <<'EOF'
module "missing" {
  source = "./modules/not-present"
}
EOF
expect_failure 'a referenced module that is missing' v0.4.0 "$demo" \
	'main.tf' './modules/not-present' 'modules/not-present'

new_fixture missing-constraint 0.4.0
cat >"$demo/modules/nested/versions.tf" <<'EOF'
terraform {
  required_version = ">= 1.8"
}
EOF
expect_failure 'a consumer module with no HubSpot constraint' v0.4.0 "$demo" \
	'module modules/nested' 'HubSpot provider constraint is missing'

new_fixture stale-tofu-lock 0.4.0
write_lock "$demo" tofu 0.3.0 '= 0.3.0' "$valid_hash"
expect_failure 'a stale OpenTofu lock' v0.4.0 "$demo" \
	'locks/tofu/.terraform.lock.hcl' 'OpenTofu' '0.3.0' 'v0.4.0'

new_fixture different-terraform-lock 0.4.0
write_lock "$demo" terraform 0.4.1 '~> 0.4.0' "$valid_hash"
expect_failure 'a differently selected Terraform lock' v0.4.0 "$demo" \
	'locks/terraform/.terraform.lock.hcl' 'Terraform' '0.4.1' 'v0.4.0'

new_fixture missing-lock 0.4.0
rm "$demo/locks/tofu/.terraform.lock.hcl"
expect_failure 'a missing committed OpenTofu lock' v0.4.0 "$demo" \
	'locks/tofu/.terraform.lock.hcl' 'committed OpenTofu lock is missing'

new_fixture malformed-lock 0.4.0
printf '%s\n' 'provider "registry.terraform.io/jackemcpherson/hubspot" {' >"$demo/locks/terraform/.terraform.lock.hcl"
expect_failure 'a malformed committed Terraform lock' v0.4.0 "$demo" \
	'locks/terraform/.terraform.lock.hcl' 'malformed Terraform lock'

new_fixture incompatible-lock-constraint 0.4.0
write_lock "$demo" tofu 0.4.0 '< 0.4.0' "$valid_hash"
expect_failure 'an incompatible OpenTofu lock constraint' v0.4.0 "$demo" \
	'locks/tofu/.terraform.lock.hcl' '< 0.4.0' 'v0.4.0'

new_fixture missing-lock-hashes 0.4.0
cat >"$demo/locks/tofu/.terraform.lock.hcl" <<'EOF'
provider "registry.opentofu.org/jackemcpherson/hubspot" {
  version     = "0.4.0"
  constraints = ">= 0.4.0, < 0.5.0"
  hashes      = []
}
EOF
expect_failure 'an OpenTofu lock with no package hashes' v0.4.0 "$demo" \
	'locks/tofu/.terraform.lock.hcl' 'no HubSpot provider package hashes'

new_fixture malformed-lock-hash 0.4.0
write_lock "$demo" terraform 0.4.0 '>= 0.4.0, < 0.5.0' 'h1:not-a-registry-package-hash'
expect_failure 'a malformed Terraform package hash' v0.4.0 "$demo" \
	'locks/terraform/.terraform.lock.hcl' 'malformed Terraform HubSpot provider package hash'

new_fixture invalid-version 0.4.0
expect_failure 'an unprefixed candidate version' 0.4.0 "$demo" 'version must be v-prefixed SemVer'
