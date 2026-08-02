#!/bin/sh
set -eu

version=${1:?release version is required}
assets=${2:?GitHub release assets directory is required}
released_live_shard=${RELEASED_LIVE_SHARD_SCRIPT:-./scripts/released-live-shard.sh}
state_migration=${STATE_MIGRATION_SCRIPT:-./scripts/verify-state-migration.sh}
released_northstar=${RELEASED_NORTHSTAR_SCRIPT:-./scripts/released-northstar-journey.sh}
verify_released=${VERIFY_RELEASED_PROVIDER_SCRIPT:-./scripts/verify-released-provider.sh}
demo_repo=${HUBSPOT_DEMO_REPO:-../terraform-hubspot-demo}
evidence_dir=${RELEASE_EVIDENCE_DIR:-acceptance-report}

test -d "$assets" || { echo "GitHub release assets directory is missing: $assets" >&2; exit 1; }
test -d "$demo_repo/.git" || { echo "Northstar demo checkout is missing: $demo_repo" >&2; exit 1; }

"$verify_released" terraform registry.terraform.io/jackemcpherson/hubspot "$version" "$assets"
"$verify_released" tofu registry.opentofu.org/jackemcpherson/hubspot "$version" "$assets"

"$released_live_shard" \
	free_properties \
	terraform \
	registry.terraform.io/jackemcpherson/hubspot \
	"$version"
"$released_live_shard" \
	free_properties \
	tofu \
	registry.opentofu.org/jackemcpherson/hubspot \
	"$version"
"$state_migration" "$version"
"$released_northstar" "$version"

release_version=${version#v}
os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$(uname -m)" in arm64|aarch64) arch=arm64 ;; x86_64|amd64) arch=amd64 ;; *) echo "unsupported evidence architecture" >&2; exit 1 ;; esac
archive=$(find "$assets" -type f -name "terraform-provider-hubspot_${release_version}_${os}_${arch}.zip" -print -quit)
test -n "$archive" || { echo "matching GitHub archive is missing from release evidence" >&2; exit 1; }
archive_digest=$(shasum -a 256 "$archive" | awk '{print $1}')
provider_commit=$(git rev-parse HEAD)
demo_commit=$(git -C "$demo_repo" rev-parse HEAD)
generated_at=$(date -u '+%Y-%m-%dT%H:%M:%SZ')
mkdir -p "$evidence_dir"
printf '{"version":"%s","provider_commit":"%s","demo_commit":"%s","archive_sha256":"%s","engines":["terraform","tofu"],"registry_sources":["registry.terraform.io/jackemcpherson/hubspot","registry.opentofu.org/jackemcpherson/hubspot"],"generated_at":"%s","cleanup":"passed","status":"passed"}\n' \
	"$version" "$provider_commit" "$demo_commit" "$archive_digest" "$generated_at" >"$evidence_dir/released-provider-$release_version.json"
