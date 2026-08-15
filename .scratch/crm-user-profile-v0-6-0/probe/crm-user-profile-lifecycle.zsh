#!/bin/zsh

set -eu

readonly service='terraform-provider-hubspot-probes'
readonly origin='https://api.hubapi.com'
readonly expected_fingerprint="${HUBSPOT_PROBE_EXPECTED_FINGERPRINT:?HUBSPOT_PROBE_EXPECTED_FINGERPRINT is required}"
readonly fixture_email="${HUBSPOT_CRM_PROFILE_FIXTURE_EMAIL:-tfhs-probe-16-20260802024807@example.com}"
readonly properties='hs_internal_user_id,hs_job_title,hs_availability_status,hs_standard_time_zone,hs_working_hours'
readonly lock_dir="${HUBSPOT_ONE_PORTAL_LOCK_DIR:-${TMPDIR:-/tmp}/hubspot-free-portal-${HUBSPOT_PORTAL_LOCK_ID:-free-configuration}.lock}"
probe_title="tfhs_crm_profile_probe_$(date -u +%Y%m%d%H%M%S)"
readonly probe_title

token=''
credential_class=''
HTTP_STATUS=''
RESPONSE_BODY=''
MEMBERSHIPS='[]'
profile_id=''
membership_id=''
restore_body=''
guard_passed=false
ownership_boundary_ready=false
typeset -a baseline_ids
baseline_ids=()

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

print_error_shape() {
  jq -c '{status, category, subCategory, context_keys: ((.context // {}) | keys)}' \
    <<<"$RESPONSE_BODY" 2>/dev/null || print -r -- '<error body omitted>'
}

require_status() {
  local step="$1"
  shift
  local allowed
  for allowed in "$@"; do
    [[ "$HTTP_STATUS" == "$allowed" ]] && return 0
  done
  print -u2 -- "${step} returned HTTP ${HTTP_STATUS}; expected one of: $*"
  print_error_shape >&2
  return 1
}

is_baseline_id() {
  local candidate="$1"
  local baseline
  for baseline in "${baseline_ids[@]}"; do
    [[ "$candidate" == "$baseline" ]] && return 0
  done
  return 1
}

list_memberships() {
  local step="$1"
  local after=''
  local route page
  MEMBERSHIPS='[]'
  while true; do
    route='/settings/users/2026-03?limit=100'
    if [[ -n "$after" ]]; then
      route="${route}&after=$(jq -rn --arg value "$after" '$value | @uri')"
    fi
    request GET "$route"
    require_status "$step" 200
    page="$(jq -c '.results // []' <<<"$RESPONSE_BODY")"
    MEMBERSHIPS="$(jq -nc --argjson existing "$MEMBERSHIPS" --argjson page "$page" \
      '$existing + $page')"
    after="$(jq -r '.paging.next.after // ""' <<<"$RESPONSE_BODY")"
    [[ -n "$after" ]] || break
  done
}

create_body() {
  jq -nc --arg email "$fixture_email" '{
    email: $email,
    firstName: "T16Reuse",
    lastName: "T16User",
    sendWelcomeEmail: false
  }'
}

delete_owned_quietly() {
  local id="$1"
  [[ -n "$id" ]] || return 0
  is_baseline_id "$id" && return 1
  request GET "/settings/users/2026-03/${id}"
  [[ "$HTTP_STATUS" == 404 ]] && return 0
  [[ "$HTTP_STATUS" == 200 ]] || return 1
  [[ "$(jq -r --arg id "$id" --arg email "$fixture_email" \
    '(.id | tostring) == $id and .email == $email and .superAdmin != true' \
    <<<"$RESPONSE_BODY")" == true ]] || return 1
  request DELETE "/settings/users/2026-03/${id}"
  [[ "$HTTP_STATUS" == 204 ]]
}

restore_profile() {
  [[ "$guard_passed" == true && -n "$profile_id" && -n "$restore_body" ]] || return 0
  request_json PATCH "/crm/objects/2026-03/users/${profile_id}" "$restore_body"
  [[ "$HTTP_STATUS" == 200 ]]
}

cleanup() {
  local exit_status=$?
  local owned_ids owned_id
  set +eu
  trap - EXIT HUP INT TERM
  if [[ "$guard_passed" == true ]]; then
    restore_profile
    if [[ "$ownership_boundary_ready" == true ]]; then
      list_memberships cleanup_memberships
      if [[ "$HTTP_STATUS" == 200 ]]; then
        owned_ids="$(jq -r --arg email "$fixture_email" \
          '.[] | select(.email == $email and .superAdmin != true) | (.id | tostring)' \
          <<<"$MEMBERSHIPS")"
        while IFS= read -r owned_id; do
          delete_owned_quietly "$owned_id"
        done <<<"$owned_ids"
      fi
      delete_owned_quietly "$membership_id"
    fi
  fi
  unset token HUBSPOT_ACCESS_TOKEN RESPONSE_BODY restore_body profile_id membership_id
  rmdir "$lock_dir" 2>/dev/null
  exit "$exit_status"
}

discover_profile() {
  local attempt after route page_matches matches
  for attempt in {1..20}; do
    after=''
    matches='[]'
    while true; do
      route='/crm/objects/2026-03/users?limit=100&properties=hs_internal_user_id'
      if [[ -n "$after" ]]; then
        route="${route}&after=$(jq -rn --arg value "$after" '$value | @uri')"
      fi
      request GET "$route"
      require_status crm_profile_discovery 200
      page_matches="$(jq -c --arg id "$membership_id" \
        '[.results[] | select(((.properties.hs_internal_user_id // "") | tostring) == $id)]' \
        <<<"$RESPONSE_BODY")"
      matches="$(jq -nc --argjson existing "$matches" --argjson page "$page_matches" \
        '$existing + $page')"
      after="$(jq -r '.paging.next.after // ""' <<<"$RESPONSE_BODY")"
      [[ -n "$after" ]] || break
    done
    if [[ "$(jq -r 'length' <<<"$matches")" == 1 ]]; then
      profile_id="$(jq -r '.[0].id | tostring' <<<"$matches")"
      return 0
    fi
    if [[ "$(jq -r 'length' <<<"$matches")" != 0 ]]; then
      print -u2 -- 'CRM profile join is ambiguous for the owned Settings identity'
      return 1
    fi
    (( attempt == 20 )) || sleep 1
  done
  print -u2 -- 'CRM profile did not materialize within 20 seconds; activate the owned HubSpot identity before retrying'
  return 1
}

mkdir "$lock_dir" 2>/dev/null || {
  print -u2 -- 'HubSpot Free portal is already in use'
  exit 1
}
trap cleanup EXIT HUP INT TERM

[[ "$fixture_email" == tfhs-probe-16-*@example.com ]] || {
  print -u2 -- 'fixture email is outside the approved residual identity class'
  exit 1
}

if [[ -n "${HUBSPOT_ACCESS_TOKEN:-}" ]]; then
  token="$HUBSPOT_ACCESS_TOKEN"
  credential_class='environment_static_token'
  unset HUBSPOT_ACCESS_TOKEN
else
  token="$(security find-generic-password -a "$USER" -s "$service" -w)"
  credential_class='keychain_static_token'
fi

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

list_memberships opening_memberships
opening_count="$(jq -r 'length' <<<"$MEMBERSHIPS")"
opening_owned_count="$(jq -r --arg email "$fixture_email" \
  '[.[] | select(.email == $email)] | length' <<<"$MEMBERSHIPS")"
while IFS= read -r baseline_id; do
  [[ -n "$baseline_id" ]] && baseline_ids+=("$baseline_id")
done <<<"$(jq -r '.[].id | tostring' <<<"$MEMBERSHIPS")"
[[ "$opening_count" == 1 && "$opening_owned_count" == 0 ]]
ownership_boundary_ready=true

request_json POST '/settings/users/2026-03' "$(create_body)"
require_status create_owned_membership 201
membership_id="$(jq -r '(.id // "") | tostring' <<<"$RESPONSE_BODY")"
[[ "$(jq -rn --arg id "$membership_id" '$id | test("^[0-9]+$")')" == true && \
  "$membership_id" != 0 ]]
is_baseline_id "$membership_id" && {
  print -u2 -- 'membership create returned a protected baseline identity'
  exit 1
}
[[ "$(jq -r --arg id "$membership_id" --arg email "$fixture_email" \
  '(.id | tostring) == $id and .email == $email and .superAdmin != true and .sendWelcomeEmail != true' \
  <<<"$RESPONSE_BODY")" == true ]]

request GET "/settings/users/2026-03/${membership_id}"
require_status exact_settings_read 200
[[ "$(jq -r --arg id "$membership_id" --arg email "$fixture_email" \
  '(.id | tostring) == $id and .email == $email and .superAdmin != true' \
  <<<"$RESPONSE_BODY")" == true ]]

discover_profile

request GET "/crm/objects/2026-03/users/${profile_id}?properties=${properties}"
require_status exact_crm_read 200
[[ "$(jq -r --arg crm "$profile_id" --arg settings "$membership_id" \
  '((.id | tostring) == $crm) and (((.properties.hs_internal_user_id // "") | tostring) == $settings)' \
  <<<"$RESPONSE_BODY")" == true ]]
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
restored_properties="$(jq -c '{
  hs_job_title: (.properties.hs_job_title // ""),
  hs_availability_status: (.properties.hs_availability_status // ""),
  hs_standard_time_zone: (.properties.hs_standard_time_zone // ""),
  hs_working_hours: (.properties.hs_working_hours // "")
}' <<<"$RESPONSE_BODY")"
[[ "$restored_properties" == "$opening_properties" ]]
restore_body=''

delete_owned_quietly "$membership_id"
request GET "/settings/users/2026-03/${membership_id}"
id_absence_status="$HTTP_STATUS"
request GET "/settings/users/2026-03/${fixture_email}?idProperty=EMAIL"
email_absence_status="$HTTP_STATUS"
[[ "$id_absence_status" == 404 && "$email_absence_status" == 404 ]]

for _cleanup_attempt in {1..20}; do
  list_memberships final_membership_poll
  final_owned_count="$(jq -r --arg email "$fixture_email" \
    '[.[] | select(.email == $email)] | length' <<<"$MEMBERSHIPS")"
  [[ "$final_owned_count" == 0 ]] && break
  sleep 1
done
final_count="$(jq -r 'length' <<<"$MEMBERSHIPS")"
baseline_survivors="$(jq -r --argjson baseline \
  "$(printf '%s\n' "${baseline_ids[@]}" | jq -R . | jq -s .)" \
  '[.[] | . as $member | select($baseline | index(($member.id | tostring)) != null)] | length' \
  <<<"$MEMBERSHIPS")"
[[ "$final_count" == "$opening_count" && "$final_owned_count" == 0 && \
  "$baseline_survivors" == "$opening_count" ]]

print -- "execution_timestamp=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
print -- "credential_class=${credential_class}"
print -- "portal_fingerprint=${actual_fingerprint}"
print -- 'settings_identity_verified=true'
print -- 'crm_identity_verified=true'
print -- 'unique_join_verified=true'
print -- 'bounded_materialization_verified=true'
print -- 'timezone_before_working_hours_verified=true'
print -- 'profile_restoration_verified=true'
print -- 'owned_membership_cleanup_verified=true'
print -- 'send_welcome_email=false'
print -- 'identity_or_credential_values_logged=false'

ownership_boundary_ready=false
guard_passed=false
unset token HUBSPOT_ACCESS_TOKEN RESPONSE_BODY restore_body profile_id membership_id
rmdir "$lock_dir"
trap - EXIT HUP INT TERM
