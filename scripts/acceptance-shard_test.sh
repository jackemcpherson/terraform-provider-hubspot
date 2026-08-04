#!/bin/sh
set -eu

root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
fixture="$tmp/repository"
bin="$tmp/bin"
mkdir -p "$fixture/acceptance/capabilities" "$fixture/internal/acceptance" "$fixture/scripts" "$bin"
cp "$root/scripts/acceptance-shard.sh" "$fixture/scripts/acceptance-shard.sh"
cp "$root/acceptance/capabilities/form_definitions.json" "$fixture/acceptance/capabilities/form_definitions.json"
printf '%s\n' 'package acceptance' >"$fixture/internal/acceptance/fixture.go"

printf '%s\n' '#!/bin/sh' \
	'if [ "$1" = -C ]; then shift 2; fi' \
	'case "$1" in' \
	'  rev-parse) printf "%s\n" aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa ;;' \
	'  status) ;;' \
	'  *) exit 1 ;;' \
	'esac' >"$bin/git"

printf '%s\n' '#!/bin/sh' \
	'printf "%s\n" "OpenTofu v1.12.3"' >"$bin/tofu"
printf '%s\n' '#!/bin/sh' \
	'printf "%s\n" "Terraform v1.15.8"' >"$bin/terraform"

# The single-quoted strings are the body of the generated Go command test double.
# shellcheck disable=SC2016
printf '%s\n' '#!/bin/sh' \
	'case "$*" in' \
	'  *" build "*|build\ *)' \
	'    previous=' \
	'    for argument do' \
	'      if [ "$previous" = -o ]; then : >"$argument"; exit 0; fi' \
	'      previous=$argument' \
	'    done' \
	'    exit 1' \
	'    ;;' \
	'  *" -list "*)' \
	'    printf "%s\n" TestAcc_form_definitions_CapabilityPreflight TestAcc_form_definitions_OpenTofuLifecycle TestAcc_form_definitions_TerraformLifecycle' \
	'    ;;' \
	'  *"-run ^TestAcc_form_definitions_"*)' \
	'    for engine in tofu terraform; do' \
	'      if [ "${FAKE_EVIDENCE_RESULT:-complete}" = missing ] && [ "$engine" = terraform ]; then continue; fi' \
	'      printf "{\"candidate_commit\":\"%s\",\"engine\":\"%s\",\"api_family\":\"marketing/v3/forms\",\"scope_family\":\"forms\",\"portal_fingerprint\":\"%064d\",\"generated_identity_hash\":\"%064d\",\"terminal_identity_hashes\":[\"%064d\",\"%064d\"],\"timestamp\":\"2026-08-03T05:00:00Z\",\"cleanup\":\"passed\"}\n" "$HUBSPOT_ACCEPTANCE_CANDIDATE_COMMIT" "$engine" 0 1 2 3 >"$HUBSPOT_ACCEPTANCE_EVIDENCE_DIR/form_definitions-$engine.json"' \
	'    done' \
	'    ;;' \
	'esac' >"$bin/go"
chmod +x "$bin/git" "$bin/go" "$bin/tofu" "$bin/terraform"

run_shard() {
	(
		cd "$fixture"
		PATH="$bin:$PATH" \
			CAPABILITY_SHARD=form_definitions \
			HUBSPOT_ACCESS_TOKEN=secret-not-evidence \
			HUBSPOT_ACCEPTANCE_PORTAL_ID=12345678 \
			HUBSPOT_ACCEPTANCE_PREFIX=tf_acc_candidate_forms_ \
			HUBSPOT_ONE_PORTAL_LOCK_DIR="$tmp/portal-lock" \
			ACCEPTANCE_REPORT_DIR="$tmp/report" \
			FAKE_EVIDENCE_RESULT=${FAKE_EVIDENCE_RESULT:-complete} \
			./scripts/acceptance-shard.sh
	)
}

run_shard
for engine in tofu terraform; do
	evidence="$tmp/report/form_definitions-$engine.json"
	test -s "$evidence"
	grep -q '"candidate_commit":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"' "$evidence"
	grep -q '"cleanup":"passed"' "$evidence"
	! grep -Fq '12345678' "$evidence"
	! grep -Fq 'secret-not-evidence' "$evidence"
	! grep -Fq 'tf_acc_candidate_forms_' "$evidence"
done
grep -q '"status":"passed"' "$tmp/report/form_definitions.json"
test ! -e "$tmp/portal-lock"

if FAKE_EVIDENCE_RESULT=missing run_shard; then
	echo 'Forms candidate run accepted incomplete engine evidence' >&2
	exit 1
fi
test ! -e "$tmp/portal-lock"

if (
	cd "$fixture"
	PATH="$bin:$PATH" CAPABILITY_SHARD=unknown HUBSPOT_ACCESS_TOKEN=test HUBSPOT_ACCEPTANCE_PREFIX=tf_acc_candidate_ \
		HUBSPOT_ONE_PORTAL_LOCK_DIR="$tmp/portal-lock" ACCEPTANCE_REPORT_DIR="$tmp/report" ./scripts/acceptance-shard.sh
); then
	echo 'candidate run accepted an unsupported shard' >&2
	exit 1
fi
