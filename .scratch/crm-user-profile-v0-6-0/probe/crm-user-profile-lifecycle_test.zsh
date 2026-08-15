#!/bin/zsh

set -eu

probe_dir="$(cd "$(dirname "$0")" && pwd)"
readonly probe_dir
state_dir="$(mktemp -d)"
readonly state_dir
readonly fixture_email='tfhs-probe-16-20260802024807@example.com'
readonly opening_profile='{"hs_internal_user_id":"101","hs_job_title":"Original","hs_availability_status":"available","hs_standard_time_zone":"Australia/Melbourne","hs_working_hours":"[{\"days\":\"MONDAY\",\"startMinute\":600,\"endMinute\":900}]"}'

cleanup() {
  rm -rf "$state_dir"
}
trap cleanup EXIT HUP INT TERM

print -r -- "$opening_profile" >"${state_dir}/profile.json"
touch "${state_dir}/requests.log"

export FAKE_HUBSPOT_STATE_DIR="$state_dir"
export FAKE_HUBSPOT_FIXTURE_EMAIL="$fixture_email"
HUBSPOT_PROBE_EXPECTED_FINGERPRINT="sha256:$(printf '12345' | shasum -a 256 | cut -c1-16)"
export HUBSPOT_PROBE_EXPECTED_FINGERPRINT
export PATH="${probe_dir}/testdata/tools:${PATH}"
unset HUBSPOT_ACCESS_TOKEN HUBSPOT_NORTHSTAR_MEMBERSHIP_ID

output="$("${probe_dir}/crm-user-profile-lifecycle.zsh")"

[[ -f "${state_dir}/security-called" ]]
[[ ! -f "${state_dir}/membership-present" ]]
[[ "$(jq -S . "${state_dir}/profile.json")" == "$(jq -S . <<<"$opening_profile")" ]]

grep -Fqx 'credential_class=keychain_static_token' <<<"$output"
grep -Fqx 'settings_identity_verified=true' <<<"$output"
grep -Fqx 'crm_identity_verified=true' <<<"$output"
grep -Fqx 'unique_join_verified=true' <<<"$output"
grep -Fqx 'profile_restoration_verified=true' <<<"$output"
grep -Fqx 'owned_membership_cleanup_verified=true' <<<"$output"
if grep -Fq "$fixture_email" <<<"$output"; then
  exit 1
fi
if grep -Fq 'fake-keychain-token' <<<"$output"; then
  exit 1
fi

grep -Fqx 'POST welcome-disabled membership' "${state_dir}/requests.log"
grep -Fqx 'PATCH hs_availability_status,hs_job_title,hs_standard_time_zone' \
  "${state_dir}/requests.log"
grep -Fqx 'PATCH hs_working_hours' "${state_dir}/requests.log"
grep -Fqx 'DELETE owned membership' "${state_dir}/requests.log"

prerequisite_line="$(grep -nxF 'PATCH hs_availability_status,hs_job_title,hs_standard_time_zone' \
  "${state_dir}/requests.log" | cut -d: -f1)"
hours_line="$(grep -nxF 'PATCH hs_working_hours' "${state_dir}/requests.log" | cut -d: -f1)"
(( prerequisite_line < hours_line ))

print -n -- '' >"${state_dir}/requests.log"
export HUBSPOT_PROBE_EXPECTED_FINGERPRINT='sha256:wrong-portal'
if "${probe_dir}/crm-user-profile-lifecycle.zsh" >/dev/null 2>&1; then
  exit 1
fi
[[ ! -s "${state_dir}/requests.log" ]]
[[ ! -f "${state_dir}/membership-present" ]]

print -r -- 'CRM user profile live probe regression test passed'
