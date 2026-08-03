#!/bin/sh
set -eu

version=${1:?release version is required}
assets=${2:?GitHub release assets directory is required}
released_live_shard=${RELEASED_LIVE_SHARD_SCRIPT:-./scripts/released-live-shard.sh}
form_migration=${RELEASED_FORM_MIGRATION_SCRIPT:-./scripts/released-form-migration.sh}
released_northstar=${RELEASED_NORTHSTAR_SCRIPT:-./scripts/released-northstar-journey.sh}
verify_released=${VERIFY_RELEASED_PROVIDER_SCRIPT:-./scripts/verify-released-provider.sh}
demo_repo=${HUBSPOT_DEMO_REPO:-../terraform-hubspot-demo}
evidence_dir=${RELEASE_EVIDENCE_DIR:-acceptance-report}
form_evidence="$evidence_dir/released-form-migration.json"
lock_dir=${HUBSPOT_ONE_PORTAL_LOCK_DIR:-"${TMPDIR:-/tmp}/hubspot-free-portal-${HUBSPOT_PORTAL_LOCK_ID:-free-configuration}.lock"}

test "$version" = v0.3.0 || { echo "released provider journey requires v0.3.0" >&2; exit 1; }
test -d "$assets" || { echo "GitHub release assets directory is missing: $assets" >&2; exit 1; }
test -d "$demo_repo/.git" || { echo "Northstar demo checkout is missing: $demo_repo" >&2; exit 1; }

started_at=$(date -u '+%Y-%m-%dT%H:%M:%SZ')
mkdir -p "$evidence_dir"
rm -f "$form_evidence"

"$verify_released" terraform registry.terraform.io/jackemcpherson/hubspot "$version" "$assets"
"$verify_released" tofu registry.opentofu.org/jackemcpherson/hubspot "$version" "$assets"

mkdir "$lock_dir" 2>/dev/null || { echo "released provider portal lifecycle is already running: $lock_dir" >&2; exit 1; }
export HUBSPOT_PORTAL_LOCK_HELD=1
release_lock() {
  code=$?
  rmdir "$lock_dir" || code=1
  exit "$code"
}
trap release_lock EXIT HUP INT TERM

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
FORM_MIGRATION_EVIDENCE_FILE="$form_evidence" "$form_migration" "$version"
test -s "$form_evidence" || { echo "released Form migration evidence is incomplete" >&2; exit 1; }
grep -q '"version":"v0.3.0"' "$form_evidence"
grep -q '"engines":\["terraform","tofu"\]' "$form_evidence"
grep -q '"registry_sources":\["registry.terraform.io/jackemcpherson/hubspot","registry.opentofu.org/jackemcpherson/hubspot"\]' "$form_evidence"
grep -q '"state_migration":"passed"' "$form_evidence"
grep -q '"identity_preserved":true' "$form_evidence"
grep -q '"terminal":"archived"' "$form_evidence"
grep -q '"completed_at":"[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9:]*Z"' "$form_evidence"
grep -q '"cleanup":"passed"' "$form_evidence"
grep -q '"status":"passed"' "$form_evidence"
! grep -Eq '[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}' "$form_evidence" || {
  echo "released Form migration evidence contains a raw identity" >&2
  exit 1
}
"$released_northstar" "$version"

release_version=${version#v}
os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$(uname -m)" in arm64|aarch64) arch=arm64 ;; x86_64|amd64) arch=amd64 ;; *) echo "unsupported evidence architecture" >&2; exit 1 ;; esac
archive=$(find "$assets" -type f -name "terraform-provider-hubspot_${release_version}_${os}_${arch}.zip" -print -quit)
test -n "$archive" || { echo "matching GitHub archive is missing from release evidence" >&2; exit 1; }
archive_digest=$(shasum -a 256 "$archive" | awk '{print $1}')
provider_commit=$(git rev-parse HEAD)
demo_commit=$(git -C "$demo_repo" rev-parse HEAD)
completed_at=$(date -u '+%Y-%m-%dT%H:%M:%SZ')
printf '{"version":"%s","provider_commit":"%s","demo_commit":"%s","archive_sha256":"%s","registry_archives":{"registry.terraform.io/jackemcpherson/hubspot":"%s","registry.opentofu.org/jackemcpherson/hubspot":"%s"},"engines":["terraform","tofu"],"registry_sources":["registry.terraform.io/jackemcpherson/hubspot","registry.opentofu.org/jackemcpherson/hubspot"],"started_at":"%s","completed_at":"%s","state_migration":"passed","form_identity_preserved":true,"northstar":"passed","cleanup":"passed","status":"passed"}\n' \
	"$version" "$provider_commit" "$demo_commit" "$archive_digest" "$archive_digest" "$archive_digest" "$started_at" "$completed_at" >"$evidence_dir/released-provider-$release_version.json"
