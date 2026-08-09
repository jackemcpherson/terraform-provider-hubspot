#!/bin/sh
set -eu

root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
log="$tmp/calls"
assets="$tmp/assets"
demo="$tmp/demo-repo"
evidence="$tmp/evidence"
provider="$tmp/provider-repo"
mkdir -p "$assets" "$demo" "$provider"
git -C "$demo" init -q
git -C "$demo" config user.name test
git -C "$demo" config user.email test@example.com
touch "$demo/README.md"
git -C "$demo" add README.md
git -C "$demo" commit -qm fixture
demo_commit=$(git -C "$demo" rev-parse HEAD)
git -C "$provider" init -q
git -C "$provider" config user.name test
git -C "$provider" config user.email test@example.com
touch "$provider/go.mod"
git -C "$provider" add go.mod
git -C "$provider" commit -qm fixture
provider_commit=$(git -C "$provider" rev-parse HEAD)
os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$(uname -m)" in arm64|aarch64) arch=arm64 ;; *) arch=amd64 ;; esac
touch "$assets/terraform-provider-hubspot_0.4.0_${os}_${arch}.zip"
printf '%s  %s\n' e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855 "terraform-provider-hubspot_0.4.0_${os}_${arch}.zip" >"$assets/terraform-provider-hubspot_0.4.0_SHA256SUMS"

# The single-quoted strings are the bodies of generated test doubles.
# shellcheck disable=SC2016
printf '%s\n' '#!/bin/sh' 'printf "live:%s\n" "$*" >>"$CALL_LOG"' >"$tmp/live"
# shellcheck disable=SC2016
cat >"$tmp/migration" <<'EOF'
#!/bin/sh
printf 'migration:%s\n' "$*" >>"$CALL_LOG"
test "${SKIP_MIGRATION_EVIDENCE:-}" = 1 && exit 0
mkdir -p "$(dirname "$FORM_MIGRATION_EVIDENCE_FILE")"
printf '%s\n' '{"version":"v0.4.0","engines":["terraform","tofu"],"registry_sources":["registry.terraform.io/jackemcpherson/hubspot","registry.opentofu.org/jackemcpherson/hubspot"],"state_migration":"passed","identity_preserved":true,"terminal_record":{"generated_identity_hash":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","terminal":"archived","active_owned_forms":0,"cleanup":"passed"},"completed_at":"2026-08-03T00:00:00Z","cleanup":"passed","status":"passed"}' >"$FORM_MIGRATION_EVIDENCE_FILE"
EOF
# shellcheck disable=SC2016
cat >"$tmp/files-migration" <<'EOF'
#!/bin/sh
printf 'files-migration:%s\n' "$*" >>"$CALL_LOG"
test "${SKIP_FILES_EVIDENCE:-}" = 1 && exit 0
mkdir -p "$(dirname "$FILES_MIGRATION_EVIDENCE_FILE")"
printf '%s\n' '{"version":"v0.4.0","engines":["terraform","tofu"],"registry_sources":["registry.terraform.io/jackemcpherson/hubspot","registry.opentofu.org/jackemcpherson/hubspot"],"state_migration":"passed","identity_preserved":true,"metadata_updates":"passed","byte_replacements":"passed","drift_repair":"passed","terminal_record":{"generated_identity_hash":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","active_owned_files":0,"active_owned_folders":0,"cleanup":"passed"},"completed_at":"2026-08-03T00:00:00Z","cleanup":"passed","status":"passed"}' >"$FILES_MIGRATION_EVIDENCE_FILE"
EOF
# shellcheck disable=SC2016
printf '%s\n' '#!/bin/sh' 'printf "northstar:%s\n" "$*" >>"$CALL_LOG"' >"$tmp/northstar"
# shellcheck disable=SC2016
printf '%s\n' '#!/bin/sh' 'printf "verify:%s\n" "$*" >>"$CALL_LOG"' >"$tmp/verify"
# shellcheck disable=SC2016
printf '%s\n' '#!/bin/sh' 'printf "registry:%s\n" "$*" >>"$CALL_LOG"' >"$tmp/registry"
# shellcheck disable=SC2016
printf '%s\n' '#!/bin/sh' 'printf "prepare:%s:%s:%s:%s\n" "$1" "$2" "$3" "$5" >>"$CALL_LOG"' 'mkdir -p "$4/scripts"' 'printf "#!/bin/sh\n" >"$4/scripts/demo"' 'chmod +x "$4/scripts/demo"' 'printf "%s\n" "$4/scripts/demo"' >"$tmp/prepare"
chmod +x "$tmp/live" "$tmp/migration" "$tmp/files-migration" "$tmp/northstar" "$tmp/verify" "$tmp/registry" "$tmp/prepare"

CALL_LOG="$log" \
	RELEASED_LIVE_SHARD_SCRIPT="$tmp/live" \
	RELEASED_FORM_MIGRATION_SCRIPT="$tmp/migration" \
	RELEASED_FILES_MIGRATION_SCRIPT="$tmp/files-migration" \
	RELEASED_NORTHSTAR_SCRIPT="$tmp/northstar" \
	VERIFY_RELEASED_PROVIDER_SCRIPT="$tmp/verify" \
	REGISTRY_INGESTION_VERIFIER="$tmp/registry" \
	PREPARE_RELEASED_DEMO_SCRIPT="$tmp/prepare" \
	HUBSPOT_DEMO_REPO="$demo" \
	HUBSPOT_PROVIDER_REPO="$provider" \
	HUBSPOT_PROVIDER_EXPECTED_COMMIT="$provider_commit" \
	HUBSPOT_DEMO_EXPECTED_COMMIT="$demo_commit" \
	HUBSPOT_ONE_PORTAL_LOCK_DIR="$tmp/portal-lock" \
	RELEASE_EVIDENCE_DIR="$evidence" \
	"$root/scripts/released-provider-journey.sh" v0.4.0 "$assets"

test "$(cat "$log")" = "registry:v0.4.0
verify:terraform registry.terraform.io/jackemcpherson/hubspot v0.4.0 $assets
verify:tofu registry.opentofu.org/jackemcpherson/hubspot v0.4.0 $assets
prepare:v0.4.0:$demo:$demo_commit:$assets
live:free_properties terraform registry.terraform.io/jackemcpherson/hubspot v0.4.0
live:free_properties tofu registry.opentofu.org/jackemcpherson/hubspot v0.4.0
migration:v0.4.0
files-migration:v0.4.0
northstar:v0.4.0"

report="$evidence/released-provider-0.4.0.json"
test -s "$report"
grep -q '"status":"passed"' "$report"
grep -q '"archive_sha256":"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"' "$report"
grep -q '"demo_commit":"[0-9a-f]\{40\}"' "$report"
grep -q '"state_migration":"passed"' "$report"
grep -q '"form_identity_preserved":true' "$report"
grep -q '"files_identity_preserved":true' "$report"
grep -q '"northstar":"passed"' "$report"

if CALL_LOG="$tmp/incomplete.log" SKIP_MIGRATION_EVIDENCE=1 \
	RELEASED_LIVE_SHARD_SCRIPT="$tmp/live" \
	RELEASED_FORM_MIGRATION_SCRIPT="$tmp/migration" \
	RELEASED_FILES_MIGRATION_SCRIPT="$tmp/files-migration" \
	RELEASED_NORTHSTAR_SCRIPT="$tmp/northstar" \
	VERIFY_RELEASED_PROVIDER_SCRIPT="$tmp/verify" \
	REGISTRY_INGESTION_VERIFIER="$tmp/registry" \
	PREPARE_RELEASED_DEMO_SCRIPT="$tmp/prepare" \
	HUBSPOT_DEMO_REPO="$demo" HUBSPOT_PROVIDER_REPO="$provider" HUBSPOT_PROVIDER_EXPECTED_COMMIT="$provider_commit" \
	HUBSPOT_DEMO_EXPECTED_COMMIT="$demo_commit" HUBSPOT_ONE_PORTAL_LOCK_DIR="$tmp/portal-lock" RELEASE_EVIDENCE_DIR="$tmp/incomplete-evidence" \
	"$root/scripts/released-provider-journey.sh" v0.4.0 "$assets" >/dev/null 2>&1; then
	echo "expected incomplete migration evidence rejection" >&2
	exit 1
fi
! grep -q '^northstar:' "$tmp/incomplete.log" || { echo "Northstar ran after incomplete Form evidence" >&2; exit 1; }
test ! -e "$tmp/portal-lock"

if CALL_LOG="$tmp/incomplete-files.log" SKIP_FILES_EVIDENCE=1 \
	RELEASED_LIVE_SHARD_SCRIPT="$tmp/live" RELEASED_FORM_MIGRATION_SCRIPT="$tmp/migration" RELEASED_FILES_MIGRATION_SCRIPT="$tmp/files-migration" \
	RELEASED_NORTHSTAR_SCRIPT="$tmp/northstar" VERIFY_RELEASED_PROVIDER_SCRIPT="$tmp/verify" REGISTRY_INGESTION_VERIFIER="$tmp/registry" \
	PREPARE_RELEASED_DEMO_SCRIPT="$tmp/prepare" HUBSPOT_DEMO_REPO="$demo" HUBSPOT_PROVIDER_REPO="$provider" \
	HUBSPOT_PROVIDER_EXPECTED_COMMIT="$provider_commit" HUBSPOT_DEMO_EXPECTED_COMMIT="$demo_commit" \
	HUBSPOT_ONE_PORTAL_LOCK_DIR="$tmp/portal-lock" RELEASE_EVIDENCE_DIR="$tmp/incomplete-files-evidence" \
	"$root/scripts/released-provider-journey.sh" v0.4.0 "$assets" >/dev/null 2>&1; then
	echo "expected incomplete Files migration evidence rejection" >&2
	exit 1
fi
grep -q '^files-migration:v0.4.0$' "$tmp/incomplete-files.log"
! grep -q '^northstar:' "$tmp/incomplete-files.log" || { echo "Northstar ran after incomplete Files evidence" >&2; exit 1; }
test ! -e "$tmp/portal-lock"

mkdir "$tmp/portal-lock"
if CALL_LOG="$tmp/locked.log" RELEASED_LIVE_SHARD_SCRIPT="$tmp/live" RELEASED_FORM_MIGRATION_SCRIPT="$tmp/migration" \
	RELEASED_FILES_MIGRATION_SCRIPT="$tmp/files-migration" RELEASED_NORTHSTAR_SCRIPT="$tmp/northstar" VERIFY_RELEASED_PROVIDER_SCRIPT="$tmp/verify" \
	REGISTRY_INGESTION_VERIFIER="$tmp/registry" PREPARE_RELEASED_DEMO_SCRIPT="$tmp/prepare" HUBSPOT_DEMO_REPO="$demo" \
	HUBSPOT_PROVIDER_REPO="$provider" HUBSPOT_PROVIDER_EXPECTED_COMMIT="$provider_commit" HUBSPOT_DEMO_EXPECTED_COMMIT="$demo_commit" \
	HUBSPOT_ONE_PORTAL_LOCK_DIR="$tmp/portal-lock" RELEASE_EVIDENCE_DIR="$tmp/locked-evidence" \
	"$root/scripts/released-provider-journey.sh" v0.4.0 "$assets" >/dev/null 2>&1; then
	echo "expected shared portal lock rejection" >&2
	exit 1
fi
! grep -q '^live:' "$tmp/locked.log" || { echo "live mutation ran while portal lock was held" >&2; exit 1; }
rmdir "$tmp/portal-lock"

if "$root/scripts/released-provider-journey.sh" v0.3.0 "$assets" >/dev/null 2>&1; then
	echo "expected wrong released-journey version rejection" >&2
	exit 1
fi
