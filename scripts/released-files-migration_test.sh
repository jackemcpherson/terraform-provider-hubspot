#!/bin/sh
set -eu

root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
bin="$tmp/bin"
fixture="$tmp/fixture"
mkdir -p "$bin" "$fixture"
cp "$root/acceptance/released/files_configuration/"* "$fixture/"

cat >"$bin/engine" <<'EOF'
#!/bin/sh
set -eu
engine=$(basename "$0")
chdir=${1#-chdir=}
shift
printf '%s:%s\n' "$engine" "$*" >>"$CALL_LOG"
if [ -n "${FAIL_MATCH:-}" ] && printf '%s:%s\n' "$engine" "$*" | grep -Fq "$FAIL_MATCH"; then
  exit 1
fi
for argument in "$@"; do
  case "$argument" in -out=*) : >"${argument#-out=}" ;; esac
done
if [ "$*" = 'plan -detailed-exitcode -input=false -out=reviewed.tfplan' ] && [ -e "$DRIFT_MARKER" ]; then
  rm -f "$DRIFT_MARKER"
  exit 2
fi
case "$*" in
  'init -input=false') ;;
  'plan -input=false -out=reviewed.tfplan') ;;
  'apply -input=false reviewed.tfplan') printf '%s\n' state >"$chdir/terraform.tfstate" ;;
  'output -raw released_root_folder_id') printf '%s\n' 10001 ;;
  'output -raw released_leaf_folder_id') printf '%s\n' 10002 ;;
  'output -raw released_file_id')
    if [ "${MISMATCH_ENGINE:-}" = "$engine" ]; then printf '%s\n' 20002; else printf '%s\n' 20001; fi
    ;;
  'plan -detailed-exitcode -input=false') ;;
  'plan -detailed-exitcode -input=false -out=reviewed.tfplan') ;;
  'plan -destroy -input=false -out=reviewed.tfplan') ;;
  'state replace-provider -auto-approve registry.terraform.io/jackemcpherson/hubspot registry.opentofu.org/jackemcpherson/hubspot') ;;
  'state replace-provider -auto-approve registry.opentofu.org/jackemcpherson/hubspot registry.terraform.io/jackemcpherson/hubspot') ;;
  *) echo "unexpected engine invocation: $engine $*" >&2; exit 1 ;;
esac
EOF
chmod +x "$bin/engine"
ln -s engine "$bin/terraform"
ln -s engine "$bin/tofu"

cat >"$tmp/helper" <<'EOF'
#!/bin/sh
set -eu
printf 'helper:%s\n' "$*" >>"$CALL_LOG"
case "$1" in
  verify-active) ;;
  drift) touch "$DRIFT_MARKER" ;;
  cleanup) ;;
  verify-terminal) printf '%s\n' '{"generated_identity_hash":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","active_owned_files":0,"active_owned_folders":0,"cleanup":"passed"}' ;;
  *) exit 1 ;;
esac
EOF
chmod +x "$tmp/helper"

log="$tmp/success.log"
evidence="$tmp/success.json"
PATH="$bin:$PATH" CALL_LOG="$log" DRIFT_MARKER="$tmp/drift" HUBSPOT_ACCESS_TOKEN=test HUBSPOT_ACCEPTANCE_PREFIX=tf_acc_released_ \
  RELEASED_FILES_FIXTURE_DIR="$fixture" RELEASED_FILES_HELPER_SCRIPT="$tmp/helper" FILES_MIGRATION_EVIDENCE_FILE="$evidence" \
  "$root/scripts/released-files-migration.sh" v0.4.0

test "$(grep -c '^helper:verify-active 10001 10002 20001 tf_acc_released_$' "$log")" -ge 9
test "$(grep -c '^helper:drift 10001 10002 20001 tf_acc_released_$' "$log")" -eq 3
test "$(grep -c ':state replace-provider ' "$log")" -eq 2
test "$(grep -c ':plan -input=false -out=reviewed.tfplan$' "$log")" -eq 4
test "$(grep -c ':plan -destroy -input=false -out=reviewed.tfplan$' "$log")" -eq 1
test "$(grep -c '^helper:verify-terminal ' "$log")" -eq 1
test "$(grep -c '^helper:cleanup ' "$log" || true)" -eq 0
grep -q '"state_migration":"passed"' "$evidence"
grep -q '"identity_preserved":true' "$evidence"
grep -q '"active_owned_files":0' "$evidence"
grep -q '"cleanup":"passed"' "$evidence"
if grep -Eq '10001|10002|20001' "$evidence"; then
  echo 'released Files evidence exposed raw identities' >&2
  exit 1
fi

failure_log="$tmp/failure.log"
if PATH="$bin:$PATH" CALL_LOG="$failure_log" DRIFT_MARKER="$tmp/failure-drift" FAIL_MATCH='tofu:plan -detailed-exitcode -input=false' \
  HUBSPOT_ACCESS_TOKEN=test HUBSPOT_ACCEPTANCE_PREFIX=tf_acc_failure_ RELEASED_FILES_FIXTURE_DIR="$fixture" \
  RELEASED_FILES_HELPER_SCRIPT="$tmp/helper" FILES_MIGRATION_EVIDENCE_FILE="$tmp/failure.json" \
  "$root/scripts/released-files-migration.sh" v0.4.0 >/dev/null 2>&1; then
  echo 'expected early Files migration failure' >&2
  exit 1
fi
grep -q '^helper:cleanup 10001 10002 20001 tf_acc_failure_$' "$failure_log"
grep -q '^helper:verify-terminal 10001 10002 20001 tf_acc_failure_$' "$failure_log"
test ! -e "$tmp/failure.json"

if PATH="$bin:$PATH" CALL_LOG="$tmp/identity.log" DRIFT_MARKER="$tmp/identity-drift" MISMATCH_ENGINE=tofu \
  HUBSPOT_ACCESS_TOKEN=test HUBSPOT_ACCEPTANCE_PREFIX=tf_acc_identity_ RELEASED_FILES_FIXTURE_DIR="$fixture" \
  RELEASED_FILES_HELPER_SCRIPT="$tmp/helper" FILES_MIGRATION_EVIDENCE_FILE="$tmp/identity.json" \
  "$root/scripts/released-files-migration.sh" v0.4.0 >/dev/null 2>&1; then
  echo 'expected Files identity-change rejection' >&2
  exit 1
fi
grep -q '^helper:cleanup 10001 10002 20001 tf_acc_identity_$' "$tmp/identity.log"
test ! -e "$tmp/identity.json"

if PATH="$bin:$PATH" CALL_LOG="$tmp/wrong-version.log" DRIFT_MARKER="$tmp/wrong-drift" HUBSPOT_ACCESS_TOKEN=test HUBSPOT_ACCEPTANCE_PREFIX=tf_acc_wrong_ \
  RELEASED_FILES_FIXTURE_DIR="$fixture" RELEASED_FILES_HELPER_SCRIPT="$tmp/helper" \
  "$root/scripts/released-files-migration.sh" v0.3.0 >/dev/null 2>&1; then
  echo 'expected wrong Files migration version rejection' >&2
  exit 1
fi

echo 'Released Files migration contract tests passed'
