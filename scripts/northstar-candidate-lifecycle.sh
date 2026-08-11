#!/bin/sh
set -eu

root=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
version=${1:?v-prefixed candidate version is required}
demo=${HUBSPOT_DEMO_SCRIPT:?HUBSPOT_DEMO_SCRIPT is required}
lock_dir=${HUBSPOT_ONE_PORTAL_LOCK_DIR:-"${TMPDIR:-/tmp}/hubspot-free-portal-${HUBSPOT_PORTAL_LOCK_ID:-free-configuration}.lock"}
test -x "$demo" || { echo "Northstar demo script is not executable" >&2; exit 1; }
demo_root=${HUBSPOT_DEMO_REPO:-"$(CDPATH='' cd -- "$(dirname -- "$demo")/.." && pwd)"}
provider_root=${HUBSPOT_PROVIDER_REPO:-$root}
northstar_files_seed=${HUBSPOT_NORTHSTAR_FILES_SEED:-"local_$$_"}
printf '%s\n' "$northstar_files_seed" | grep -Eq '^[[:alnum:]_]+$' || {
  echo "HUBSPOT_NORTHSTAR_FILES_SEED must contain only letters, digits, and underscores" >&2
  exit 1
}
test "$(printf '%s' "$northstar_files_seed" | wc -c | tr -d ' ')" -le 100 || {
  echo "HUBSPOT_NORTHSTAR_FILES_SEED must be at most 100 characters" >&2
  exit 1
}
active_files_prefix=ns_

if [ "${HUBSPOT_REQUIRE_CLEAN_PROVENANCE:-}" = 1 ]; then
	provider_commit=${HUBSPOT_PROVIDER_EXPECTED_COMMIT:?HUBSPOT_PROVIDER_EXPECTED_COMMIT is required for protected Northstar runs}
	demo_commit=${HUBSPOT_DEMO_EXPECTED_COMMIT:?HUBSPOT_DEMO_EXPECTED_COMMIT is required for protected Northstar runs}
	: "${HUBSPOT_NORTHSTAR_FILES_SEED:?HUBSPOT_NORTHSTAR_FILES_SEED is required for protected Northstar runs}"
	GOTOOLCHAIN=local go run "$root/cmd/validate-checkout" "$provider_root" "$provider_commit"
	GOTOOLCHAIN=local go run "$root/cmd/validate-checkout" "$demo_root" "$demo_commit"
fi

"$root/scripts/validate-candidate-compatibility.sh" "$version" "$demo_root"
mkdir "$lock_dir" 2>/dev/null || { echo "Northstar portal lifecycle is already running: $lock_dir" >&2; exit 1; }
run_failure_cleanup() {
  if [ -n "${HUBSPOT_NORTHSTAR_CLEANUP_SCRIPT:-}" ]; then
    HUBSPOT_NORTHSTAR_FILES_PREFIX=$active_files_prefix "$HUBSPOT_NORTHSTAR_CLEANUP_SCRIPT"
    return
  fi
  HUBSPOT_NORTHSTAR_FILES_PREFIX=$active_files_prefix GOTOOLCHAIN=local go run "$root/cmd/northstar-lifecycle" cleanup
}
cleanup() {
  status=$?
  trap - EXIT HUP INT TERM
  if [ "$status" -ne 0 ]; then
    if ! run_failure_cleanup; then
      echo "Northstar failure cleanup did not complete" >&2
      status=1
    fi
  fi
  rmdir "$lock_dir" 2>/dev/null || true
  exit "$status"
}
trap cleanup EXIT HUP INT TERM

run() {
	engine=$1
	case "$engine" in
		tofu) engine_code=o ;;
		terraform) engine_code=t ;;
		*) echo "unsupported Northstar engine" >&2; return 1 ;;
	esac
	namespace=$(printf '%s' "${northstar_files_seed}_${engine}" | shasum -a 256 | cut -c1-8)
	active_files_prefix=ns_${namespace}_${engine_code}_
	ENGINE=$engine HUBSPOT_NORTHSTAR_FILES_PREFIX=$active_files_prefix HUBSPOT_PORTAL_LOCK_HELD=1 "$demo" local "$2"
}

stage_file_for_refresh() {
	engine=$1
	if [ -n "${HUBSPOT_NORTHSTAR_STAGE_SCRIPT:-}" ]; then
		HUBSPOT_NORTHSTAR_FILES_PREFIX=$active_files_prefix "$HUBSPOT_NORTHSTAR_STAGE_SCRIPT" "$engine"
		return
	fi
	private_file_id=$("$engine" -chdir="$demo_root" output -raw northstar_private_file_id)
	brand_folder_id=$("$engine" -chdir="$demo_root" output -raw northstar_brand_folder_id)
	downloads_folder_id=$("$engine" -chdir="$demo_root" output -raw northstar_downloads_folder_id)
	HUBSPOT_NORTHSTAR_FILES_PREFIX=$active_files_prefix GOTOOLCHAIN=local \
		go run "$root/cmd/northstar-lifecycle" stage-file-for-folder-rename "$private_file_id" "$brand_folder_id" "$downloads_folder_id"
}

for engine in tofu terraform; do
  run "$engine" plan
  run "$engine" apply
  run "$engine" verify
  run "$engine" drift
  run "$engine" repair
  stage_file_for_refresh "$engine"
  HUBSPOT_NORTHSTAR_FILE_REFRESH_DRIFT=1 \
    HUBSPOT_NORTHSTAR_PRIVATE_FILE_ID=${private_file_id:-1} run "$engine" refresh
  run "$engine" adopt
  run "$engine" verify
  run "$engine" destroy-plan
  run "$engine" destroy-apply
done
