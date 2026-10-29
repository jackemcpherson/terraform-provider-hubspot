#!/bin/sh
set -eu

root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM

# fixture copies the files check-workflows.sh reads into a fresh tree.
fixture() {
	dir="$tmp/$1"
	mkdir "$dir"
	cp "$root/Makefile" "$dir/Makefile"
	cp -R "$root/.github" "$dir/.github"
	echo "$dir"
}

# mutate rewrites one fixture file with a sed expression. It avoids sed -i,
# whose syntax differs between BSD and GNU sed.
mutate() {
	file=$1
	expression=$2
	sed "$expression" "$file" >"$file.new"
	! cmp -s "$file" "$file.new" || {
		echo "mutation $expression did not change $file" >&2
		exit 1
	}
	mv "$file.new" "$file"
}

# expect_rejection runs the check in a fixture and requires a failure whose
# message contains the given text.
expect_rejection() {
	dir=$1
	message=$2
	if (cd "$dir" && "$root/scripts/check-workflows.sh") >"$dir/out" 2>&1; then
		echo "check-workflows accepted a fixture that should fail: $message" >&2
		exit 1
	fi
	grep -Fq "$message" "$dir/out" || {
		echo "check-workflows failed without '$message':" >&2
		cat "$dir/out" >&2
		exit 1
	}
}

pristine=$(fixture pristine)
(cd "$pristine" && "$root/scripts/check-workflows.sh")

dir=$(fixture validate-tofu)
mutate "$dir/.github/workflows/validate-provider.yml" "s/tofu-version: '[^']*'/tofu-version: '0.0.1'/"
expect_rejection "$dir" ".github/workflows/validate-provider.yml pins OpenTofu 0.0.1"

dir=$(fixture makefile-terraform)
mutate "$dir/Makefile" 's/^TERRAFORM_CURRENT_VERSION := .*/TERRAFORM_CURRENT_VERSION := 9.9.9/'
expect_rejection "$dir" "pins Terraform"

dir=$(fixture maintenance-missing)
mutate "$dir/.github/workflows/provider-maintenance.yml" "/terraform-version: /d"
expect_rejection "$dir" ".github/workflows/provider-maintenance.yml must pin Terraform"
