#!/bin/sh
set -eu

root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
bin="$tmp/bin"
fixture="$tmp/fixture"
mkdir -p "$bin" "$fixture"
cp "$root/acceptance/released/form_definitions/main.tf.tmpl" "$fixture/main.tf.tmpl"

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
if [ "$*" = 'plan -detailed-exitcode -input=false' ] && [ -e "$DRIFT_MARKER" ]; then
  rm -f "$DRIFT_MARKER"
  exit 2
fi
case "$*" in
  'init -input=false') ;;
  'apply -auto-approve -input=false') printf '%s\n' state >"$chdir/terraform.tfstate" ;;
  'output -raw released_form_id')
    if [ "${MISMATCH_ENGINE:-}" = "$engine" ]; then
      printf '%s\n' '00000000-0000-4000-8000-000000000002'
    else
      printf '%s\n' '00000000-0000-4000-8000-000000000001'
    fi
    ;;
  'plan -detailed-exitcode -input=false') ;;
  'state replace-provider -auto-approve registry.terraform.io/jackemcpherson/hubspot registry.opentofu.org/jackemcpherson/hubspot') ;;
  'state replace-provider -auto-approve registry.opentofu.org/jackemcpherson/hubspot registry.terraform.io/jackemcpherson/hubspot') ;;
  'destroy -auto-approve -input=false') ;;
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
  verify-active|archive) ;;
  drift) touch "$DRIFT_MARKER" ;;
  verify-terminal) printf '%s\n' '{"generated_identity_hash":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","terminal":"archived","active_owned_forms":0,"cleanup":"passed"}' ;;
  *) exit 1 ;;
esac
EOF
chmod +x "$tmp/helper"

log="$tmp/success.log"
evidence="$tmp/success.json"
PATH="$bin:$PATH" CALL_LOG="$log" DRIFT_MARKER="$tmp/drift" HUBSPOT_ACCESS_TOKEN=test HUBSPOT_ACCEPTANCE_PREFIX=tf_acc_released_ \
  RELEASED_FORM_FIXTURE_DIR="$fixture" RELEASED_FORM_HELPER_SCRIPT="$tmp/helper" FORM_MIGRATION_EVIDENCE_FILE="$evidence" \
  "$root/scripts/released-form-migration.sh" v0.3.0

test "$(grep -c '^helper:verify-active 00000000-0000-4000-8000-000000000001 tf_acc_released_$' "$log")" -ge 9
test "$(grep -c '^helper:drift 00000000-0000-4000-8000-000000000001 tf_acc_released_$' "$log")" -eq 3
test "$(grep -c ':state replace-provider ' "$log")" -eq 2
test "$(grep -c ':destroy -auto-approve -input=false$' "$log")" -eq 1
test "$(grep -c '^helper:verify-terminal ' "$log")" -eq 1
test "$(grep -c '^helper:archive ' "$log" || true)" -eq 0
tf_to_tofu_line=$(grep -n 'terraform:state replace-provider .*registry.opentofu.org' "$log" | cut -d: -f1)
tofu_init_line=$(grep -n '^tofu:init -input=false$' "$log" | head -1 | cut -d: -f1)
tofu_to_tf_line=$(grep -n 'tofu:state replace-provider .*registry.terraform.io' "$log" | cut -d: -f1)
final_tf_init_line=$(grep -n '^terraform:init -input=false$' "$log" | tail -1 | cut -d: -f1)
test "$tf_to_tofu_line" -lt "$tofu_init_line"
test "$tofu_init_line" -lt "$tofu_to_tf_line"
test "$tofu_to_tf_line" -lt "$final_tf_init_line"
grep -q '"state_migration":"passed"' "$evidence"
grep -q '"identity_preserved":true' "$evidence"
grep -q '"cleanup":"passed"' "$evidence"
! grep -q '00000000-0000-4000-8000-000000000001' "$evidence"

failure_log="$tmp/failure.log"
if PATH="$bin:$PATH" CALL_LOG="$failure_log" DRIFT_MARKER="$tmp/failure-drift" FAIL_MATCH='tofu:plan -detailed-exitcode -input=false' \
  HUBSPOT_ACCESS_TOKEN=test HUBSPOT_ACCEPTANCE_PREFIX=tf_acc_failure_ RELEASED_FORM_FIXTURE_DIR="$fixture" \
  RELEASED_FORM_HELPER_SCRIPT="$tmp/helper" FORM_MIGRATION_EVIDENCE_FILE="$tmp/failure.json" \
  "$root/scripts/released-form-migration.sh" v0.3.0 >/dev/null 2>&1; then
  echo "expected early migration failure" >&2
  exit 1
fi
grep -q '^helper:archive 00000000-0000-4000-8000-000000000001 tf_acc_failure_$' "$failure_log"
grep -q '^helper:verify-terminal 00000000-0000-4000-8000-000000000001 tf_acc_failure_$' "$failure_log"
test ! -e "$tmp/failure.json"

if PATH="$bin:$PATH" CALL_LOG="$tmp/identity.log" DRIFT_MARKER="$tmp/identity-drift" MISMATCH_ENGINE=tofu \
  HUBSPOT_ACCESS_TOKEN=test HUBSPOT_ACCEPTANCE_PREFIX=tf_acc_identity_ RELEASED_FORM_FIXTURE_DIR="$fixture" \
  RELEASED_FORM_HELPER_SCRIPT="$tmp/helper" FORM_MIGRATION_EVIDENCE_FILE="$tmp/identity.json" \
  "$root/scripts/released-form-migration.sh" v0.3.0 >/dev/null 2>&1; then
  echo "expected identity-change rejection" >&2
  exit 1
fi
grep -q '^helper:archive 00000000-0000-4000-8000-000000000001 tf_acc_identity_$' "$tmp/identity.log"
test ! -e "$tmp/identity.json"

if PATH="$bin:$PATH" CALL_LOG="$tmp/wrong-version.log" DRIFT_MARKER="$tmp/wrong-drift" HUBSPOT_ACCESS_TOKEN=test HUBSPOT_ACCEPTANCE_PREFIX=tf_acc_wrong_ \
  RELEASED_FORM_FIXTURE_DIR="$fixture" RELEASED_FORM_HELPER_SCRIPT="$tmp/helper" \
  "$root/scripts/released-form-migration.sh" v0.2.0 >/dev/null 2>&1; then
  echo "expected wrong-version rejection" >&2
  exit 1
fi

echo "Released Form migration contract tests passed"
