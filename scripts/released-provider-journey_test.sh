#!/bin/sh
set -eu

root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
log="$tmp/calls"
assets="$tmp/assets"
demo="$tmp/demo-repo"
evidence="$tmp/evidence"
mkdir -p "$assets" "$demo"
git -C "$demo" init -q
git -C "$demo" config user.name test
git -C "$demo" config user.email test@example.com
touch "$demo/README.md"
git -C "$demo" add README.md
git -C "$demo" commit -qm fixture
os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$(uname -m)" in arm64|aarch64) arch=arm64 ;; *) arch=amd64 ;; esac
touch "$assets/terraform-provider-hubspot_0.3.0_${os}_${arch}.zip"

# The single-quoted strings are the bodies of generated test doubles.
# shellcheck disable=SC2016
printf '%s\n' '#!/bin/sh' 'printf "live:%s\n" "$*" >>"$CALL_LOG"' >"$tmp/live"
# shellcheck disable=SC2016
cat >"$tmp/migration" <<'EOF'
#!/bin/sh
printf 'migration:%s\n' "$*" >>"$CALL_LOG"
test "${SKIP_MIGRATION_EVIDENCE:-}" = 1 && exit 0
mkdir -p "$(dirname "$FORM_MIGRATION_EVIDENCE_FILE")"
printf '%s\n' '{"version":"v0.3.0","engines":["terraform","tofu"],"registry_sources":["registry.terraform.io/jackemcpherson/hubspot","registry.opentofu.org/jackemcpherson/hubspot"],"state_migration":"passed","identity_preserved":true,"terminal_record":{"generated_identity_hash":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","terminal":"archived","active_owned_forms":0,"cleanup":"passed"},"completed_at":"2026-08-03T00:00:00Z","cleanup":"passed","status":"passed"}' >"$FORM_MIGRATION_EVIDENCE_FILE"
EOF
# shellcheck disable=SC2016
printf '%s\n' '#!/bin/sh' 'printf "northstar:%s\n" "$*" >>"$CALL_LOG"' >"$tmp/northstar"
# shellcheck disable=SC2016
printf '%s\n' '#!/bin/sh' 'printf "verify:%s\n" "$*" >>"$CALL_LOG"' >"$tmp/verify"
chmod +x "$tmp/live" "$tmp/migration" "$tmp/northstar" "$tmp/verify"

CALL_LOG="$log" \
	RELEASED_LIVE_SHARD_SCRIPT="$tmp/live" \
	RELEASED_FORM_MIGRATION_SCRIPT="$tmp/migration" \
	RELEASED_NORTHSTAR_SCRIPT="$tmp/northstar" \
	VERIFY_RELEASED_PROVIDER_SCRIPT="$tmp/verify" \
	HUBSPOT_DEMO_REPO="$demo" \
	HUBSPOT_ONE_PORTAL_LOCK_DIR="$tmp/portal-lock" \
	RELEASE_EVIDENCE_DIR="$evidence" \
	"$root/scripts/released-provider-journey.sh" v0.3.0 "$assets"

test "$(cat "$log")" = "verify:terraform registry.terraform.io/jackemcpherson/hubspot v0.3.0 $assets
verify:tofu registry.opentofu.org/jackemcpherson/hubspot v0.3.0 $assets
live:free_properties terraform registry.terraform.io/jackemcpherson/hubspot v0.3.0
live:free_properties tofu registry.opentofu.org/jackemcpherson/hubspot v0.3.0
migration:v0.3.0
northstar:v0.3.0"

report="$evidence/released-provider-0.3.0.json"
test -s "$report"
grep -q '"status":"passed"' "$report"
grep -q '"archive_sha256":"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"' "$report"
grep -q '"demo_commit":"[0-9a-f]\{40\}"' "$report"
grep -q '"state_migration":"passed"' "$report"
grep -q '"form_identity_preserved":true' "$report"
grep -q '"northstar":"passed"' "$report"

if CALL_LOG="$tmp/incomplete.log" SKIP_MIGRATION_EVIDENCE=1 \
	RELEASED_LIVE_SHARD_SCRIPT="$tmp/live" \
	RELEASED_FORM_MIGRATION_SCRIPT="$tmp/migration" \
	RELEASED_NORTHSTAR_SCRIPT="$tmp/northstar" \
	VERIFY_RELEASED_PROVIDER_SCRIPT="$tmp/verify" \
	HUBSPOT_DEMO_REPO="$demo" HUBSPOT_ONE_PORTAL_LOCK_DIR="$tmp/portal-lock" RELEASE_EVIDENCE_DIR="$tmp/incomplete-evidence" \
	"$root/scripts/released-provider-journey.sh" v0.3.0 "$assets" >/dev/null 2>&1; then
	echo "expected incomplete migration evidence rejection" >&2
	exit 1
fi
! grep -q '^northstar:' "$tmp/incomplete.log" || { echo "Northstar ran after incomplete Form evidence" >&2; exit 1; }
test ! -e "$tmp/portal-lock"

mkdir "$tmp/portal-lock"
if CALL_LOG="$tmp/locked.log" RELEASED_LIVE_SHARD_SCRIPT="$tmp/live" RELEASED_FORM_MIGRATION_SCRIPT="$tmp/migration" \
	RELEASED_NORTHSTAR_SCRIPT="$tmp/northstar" VERIFY_RELEASED_PROVIDER_SCRIPT="$tmp/verify" HUBSPOT_DEMO_REPO="$demo" \
	HUBSPOT_ONE_PORTAL_LOCK_DIR="$tmp/portal-lock" RELEASE_EVIDENCE_DIR="$tmp/locked-evidence" \
	"$root/scripts/released-provider-journey.sh" v0.3.0 "$assets" >/dev/null 2>&1; then
	echo "expected shared portal lock rejection" >&2
	exit 1
fi
! grep -q '^live:' "$tmp/locked.log" || { echo "live mutation ran while portal lock was held" >&2; exit 1; }
rmdir "$tmp/portal-lock"

if "$root/scripts/released-provider-journey.sh" v0.2.0 "$assets" >/dev/null 2>&1; then
	echo "expected wrong released-journey version rejection" >&2
	exit 1
fi
