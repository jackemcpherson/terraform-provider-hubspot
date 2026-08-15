#!/bin/zsh

set -eu

readonly origin='https://api.hubapi.com'
readonly token="${HUBSPOT_ACCESS_TOKEN:?HUBSPOT_ACCESS_TOKEN is required}"
readonly expected_fingerprint="${HUBSPOT_PROBE_EXPECTED_FINGERPRINT:?HUBSPOT_PROBE_EXPECTED_FINGERPRINT is required}"
readonly membership_id="${HUBSPOT_NORTHSTAR_MEMBERSHIP_ID:?HUBSPOT_NORTHSTAR_MEMBERSHIP_ID is required}"
readonly properties='hs_internal_user_id,hs_job_title,hs_availability_status,hs_standard_time_zone,hs_working_hours'
readonly lock_dir="${HUBSPOT_ONE_PORTAL_LOCK_DIR:-${TMPDIR:-/tmp}/hubspot-free-portal-${HUBSPOT_PORTAL_LOCK_ID:-free-configuration}.lock}"
readonly probe_title="tfhs_crm_profile_probe_$(date -u +%Y%m%d%H%M%S)"

HTTP_STATUS=''
RESPONSE_BODY=''
profile_id=''
restore_body=''
guard_passed=false

request() {
  local method="$1"
  local route="$2"
  local response
  response="$(curl -sS -w $'\n%{http_code}' -X "$method" \
    -H "Authorization: Bearer $token" \
    -H 'Accept: application/json' \
    "${origin}${route}")"
  HTTP_STATUS="${response##*$'\n'}"
  RESPONSE_BODY="${response%$'\n'*}"
}

request_json() {
  local method="$1"
  local route="$2"
  local body="$3"
  local response
  response="$(curl -sS -w $'\n%{http_code}' -X "$method" \
    -H "Authorization: Bearer $token" \
    -H 'Accept: application/json' \
    -H 'Content-Type: application/json' \
    --data "$body" \
    "${origin}${route}")"
  HTTP_STATUS="${response##*$'\n'}"
  RESPONSE_BODY="${response%$'\n'*}"
}

require_status() {
  local step="$1"
  local expected="$2"
  if [[ "$HTTP_STATUS" != "$expected" ]]; then
    print -u2 -- "${step} returned HTTP ${HTTP_STATUS}; expected ${expected}"
    return 1
  fi
}

restore_profile() {
  [[ "$guard_passed" == true && -n "$profile_id" && -n "$restore_body" ]] || return 0
  request_json PATCH "/crm/objects/2026-03/users/${profile_id}" "$restore_body"
  [[ "$HTTP_STATUS" == 200 ]]
}

cleanup() {
  local exit_status=$?
  set +eu
  trap - EXIT HUP INT TERM
  restore_profile
  unset HUBSPOT_ACCESS_TOKEN RESPONSE_BODY restore_body profile_id
  rmdir "$lock_dir" 2>/dev/null
  exit "$exit_status"
}

mkdir "$lock_dir" 2>/dev/null || {
  print -u2 -- 'HubSpot Free portal is already in use'
  exit 1
}
trap 'cleanup' EXIT HUP INT TERM

request GET '/account-info/v3/details'
require_status account_guard 200
portal_id="$(jq -r '.portalId' <<<"$RESPONSE_BODY")"
actual_fingerprint="sha256:$(printf '%s' "$portal_id" | shasum -a 256 | cut -c1-16)"
unset portal_id RESPONSE_BODY
[[ "$actual_fingerprint" == "$expected_fingerprint" ]] || {
  print -u2 -- 'account fingerprint mismatch; refusing probe'
  exit 1
}
guard_passed=true

request GET "/settings/users/2026-03/${membership_id}"
require_status exact_settings_read 200
[[ "$(jq -r --arg id "$membership_id" '(.id | tostring) == $id' <<<"$RESPONSE_BODY")" == true ]]

after=''
matches='[]'
while true; do
  route="/crm/objects/2026-03/users?limit=100&properties=${properties}"
  if [[ -n "$after" ]]; then
    encoded_after="$(jq -rn --arg value "$after" '$value | @uri')"
    route="${route}&after=${encoded_after}"
  fi
  request GET "$route"
  require_status crm_profile_discovery 200
  page_matches="$(jq -c --arg id "$membership_id" '[.results[] | select(((.properties.hs_internal_user_id // "") | tostring) == $id)]' <<<"$RESPONSE_BODY")"
  matches="$(jq -nc --argjson existing "$matches" --argjson page "$page_matches" '$existing + $page')"
  after="$(jq -r '.paging.next.after // ""' <<<"$RESPONSE_BODY")"
  [[ -n "$after" ]] || break
done
[[ "$(jq -r 'length' <<<"$matches")" == 1 ]] || {
  print -u2 -- 'expected one CRM profile for the protected Settings identity'
  exit 1
}
profile_id="$(jq -r '.[0].id | tostring' <<<"$matches")"

request GET "/crm/objects/2026-03/users/${profile_id}?properties=${properties}"
require_status exact_crm_read 200
[[ "$(jq -r --arg crm "$profile_id" --arg settings "$membership_id" '((.id | tostring) == $crm) and (((.properties.hs_internal_user_id // "") | tostring) == $settings)' <<<"$RESPONSE_BODY")" == true ]]
restore_body="$(jq -c '{properties: {
  hs_job_title: (.properties.hs_job_title // ""),
  hs_availability_status: (.properties.hs_availability_status // ""),
  hs_standard_time_zone: (.properties.hs_standard_time_zone // ""),
  hs_working_hours: (.properties.hs_working_hours // "")
}}' <<<"$RESPONSE_BODY")"

profile_body="$(jq -nc --arg title "$probe_title" '{properties: {
  hs_job_title: $title,
  hs_availability_status: "away",
  hs_standard_time_zone: "Australia/Melbourne"
}}')"
request_json PATCH "/crm/objects/2026-03/users/${profile_id}" "$profile_body"
require_status profile_prerequisites 200

working_hours='[{"days":"MONDAY_TO_FRIDAY","startMinute":540,"endMinute":1020}]'
hours_body="$(jq -nc --arg hours "$working_hours" '{properties: {hs_working_hours: $hours}}')"
request_json PATCH "/crm/objects/2026-03/users/${profile_id}" "$hours_body"
require_status profile_working_hours 200

request GET "/crm/objects/2026-03/users/${profile_id}?properties=${properties}"
require_status profile_verification 200
[[ "$(jq -r --arg crm "$profile_id" --arg settings "$membership_id" --arg title "$probe_title" --arg hours "$working_hours" '
  ((.id | tostring) == $crm) and
  (((.properties.hs_internal_user_id // "") | tostring) == $settings) and
  (.properties.hs_job_title == $title) and
  (.properties.hs_availability_status == "away") and
  (.properties.hs_standard_time_zone == "Australia/Melbourne") and
  (.properties.hs_working_hours == $hours)' <<<"$RESPONSE_BODY")" == true ]]

restore_profile
request GET "/crm/objects/2026-03/users/${profile_id}?properties=${properties}"
require_status restoration_verification 200
opening_properties="$(jq -c '.properties' <<<"$restore_body")"
restored_properties="$(jq -c '{properties: {
  hs_job_title: (.properties.hs_job_title // ""),
  hs_availability_status: (.properties.hs_availability_status // ""),
  hs_standard_time_zone: (.properties.hs_standard_time_zone // ""),
  hs_working_hours: (.properties.hs_working_hours // "")
}} | .properties' <<<"$RESPONSE_BODY")"
[[ "$restored_properties" == "$opening_properties" ]]
restore_body=''

print -- "execution_timestamp=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
print -- "portal_fingerprint=${actual_fingerprint}"
print -- 'settings_identity_verified=true'
print -- 'crm_identity_verified=true'
print -- 'unique_join_verified=true'
print -- 'timezone_before_working_hours_verified=true'
print -- 'profile_restoration_verified=true'
print -- 'send_welcome_email=false_no_settings_create'
print -- 'identity_or_credential_values_logged=false'

guard_passed=false
unset HUBSPOT_ACCESS_TOKEN RESPONSE_BODY restore_body profile_id
rmdir "$lock_dir"
trap - EXIT HUP INT TERM
