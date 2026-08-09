#!/bin/sh
set -eu

root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
log="$tmp/calls"
demo_root="$tmp/demo-root"
mkdir -p "$demo_root/scripts" "$demo_root/locks/tofu" "$demo_root/locks/terraform"

printf '%s\n' '#!/bin/sh' \
  'printf "%s:%s:%s\n" "$ENGINE" "$1" "$2" >>"$CALL_LOG"' \
  'test "${FAIL_PHASE:-none}" != "$ENGINE:$2"' >"$demo_root/scripts/demo"
chmod +x "$demo_root/scripts/demo"
cat >"$demo_root/versions.tf" <<'EOF'
terraform {
  required_providers {
    hubspot = {
      source  = "jackemcpherson/hubspot"
      version = ">= 0.4.0, < 0.5.0"
    }
  }
}
EOF
for engine in tofu terraform; do
  case "$engine" in
    tofu) address=registry.opentofu.org/jackemcpherson/hubspot ;;
    terraform) address=registry.terraform.io/jackemcpherson/hubspot ;;
  esac
  cat >"$demo_root/locks/$engine/.terraform.lock.hcl" <<EOF
provider "$address" {
  version     = "0.4.0"
  constraints = ">= 0.4.0, < 0.5.0"
  hashes      = ["h1:0WnQNSWX/OwYe1pbEN8Kla8ej2VV90oP7vLcBj6SZ7A="]
}
EOF
done

git -C "$demo_root" init -q
git -C "$demo_root" config user.name test
git -C "$demo_root" config user.email test@example.com
git -C "$demo_root" add .
git -C "$demo_root" commit -qm fixture
demo_commit=$(git -C "$demo_root" rev-parse HEAD)

provider_root="$tmp/provider-root"
mkdir "$provider_root"
git -C "$provider_root" init -q
git -C "$provider_root" config user.name test
git -C "$provider_root" config user.email test@example.com
touch "$provider_root/go.mod"
git -C "$provider_root" add go.mod
git -C "$provider_root" commit -qm fixture
provider_commit=$(git -C "$provider_root" rev-parse HEAD)

run() {
  CALL_LOG="$log" HUBSPOT_DEMO_SCRIPT="$demo_root/scripts/demo" HUBSPOT_ONE_PORTAL_LOCK_DIR="$tmp/lock" \
    "$root/scripts/northstar-candidate-lifecycle.sh" v0.4.0
}

run
for engine in tofu terraform; do
  for phase in plan apply verify drift repair refresh adopt verify destroy-plan destroy-apply; do
    printf '%s:%s:%s\n' "$engine" local "$phase"
  done
done >"$tmp/expected"
cmp "$log" "$tmp/expected"
if test -e "$tmp/lock"; then
  echo "Northstar lifecycle retained the portal lock after a successful candidate lifecycle" >&2
  exit 1
fi

: >"$log"
(
  set +e
  CALL_LOG="$log" FAIL_PHASE=tofu:repair HUBSPOT_DEMO_SCRIPT="$demo_root/scripts/demo" HUBSPOT_ONE_PORTAL_LOCK_DIR="$tmp/lock" \
    "$root/scripts/northstar-candidate-lifecycle.sh" v0.4.0
  printf '%s\n' "$?" >"$tmp/failed-phase-status"
)
failed_phase_status=$(cat "$tmp/failed-phase-status")
if test "$failed_phase_status" -eq 0; then
  echo "Northstar lifecycle accepted a skipped or failed Forms phase" >&2
  exit 1
fi
test "$(tail -1 "$log")" = 'tofu:local:repair'
if test -e "$tmp/lock"; then
  echo "Northstar lifecycle retained the portal lock after a failed phase" >&2
  exit 1
fi

worktree="$tmp/demo-worktree"
git -C "$demo_root" worktree add --quiet --detach "$worktree" "$demo_commit"
: >"$log"
CALL_LOG="$log" HUBSPOT_DEMO_SCRIPT="$worktree/scripts/demo" HUBSPOT_ONE_PORTAL_LOCK_DIR="$tmp/lock" \
  HUBSPOT_REQUIRE_CLEAN_PROVENANCE=1 HUBSPOT_PROVIDER_REPO="$provider_root" \
  HUBSPOT_PROVIDER_EXPECTED_COMMIT="$provider_commit" HUBSPOT_DEMO_EXPECTED_COMMIT="$demo_commit" \
  "$root/scripts/northstar-candidate-lifecycle.sh" v0.4.0
test "$(wc -l <"$log" | tr -d ' ')" = 20

(
  set +e
  CALL_LOG="$log" HUBSPOT_DEMO_SCRIPT="$worktree/scripts/demo" HUBSPOT_ONE_PORTAL_LOCK_DIR="$tmp/lock" \
    HUBSPOT_REQUIRE_CLEAN_PROVENANCE=1 HUBSPOT_PROVIDER_REPO="$provider_root" \
    HUBSPOT_PROVIDER_EXPECTED_COMMIT=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa HUBSPOT_DEMO_EXPECTED_COMMIT="$demo_commit" \
    "$root/scripts/northstar-candidate-lifecycle.sh" v0.4.0
  printf '%s\n' "$?" >"$tmp/wrong-provenance-status"
)
wrong_provenance_status=$(cat "$tmp/wrong-provenance-status")
if test "$wrong_provenance_status" -eq 0; then
  echo "Northstar lifecycle accepted the wrong provider provenance" >&2
  exit 1
fi
if test -e "$tmp/lock"; then
  echo "Northstar lifecycle retained the portal lock after rejecting wrong provenance" >&2
  exit 1
fi

mkdir "$tmp/lock"
if run; then
  echo "Northstar lifecycle accepted a shared portal lock collision" >&2
  exit 1
fi

rm -r "$tmp/lock"
: >"$log"
sed 's/< 0.5.0/< 0.4.0/' "$demo_root/versions.tf" >"$demo_root/stale.tf"
rm "$demo_root/versions.tf"
if run; then
  echo "Northstar lifecycle accepted an incompatible candidate" >&2
  exit 1
fi
test ! -s "$log"
test ! -e "$tmp/lock"
