#!/bin/zsh

set -eu

readonly service='terraform-provider-hubspot-probes'
readonly origin='https://api.hubapi.com'
readonly expected_fingerprint="${HUBSPOT_PROBE_EXPECTED_FINGERPRINT:?set the approved portal fingerprint}"
readonly fixture_email="${HUBSPOT_ACCOUNT_MEMBERSHIP_FIXTURE_EMAIL:-tfhs-probe-16-20260802024807@example.com}"
readonly fixture_first_name='T16Reuse'
readonly fixture_last_name='T16User'
readonly lock_dir="${HUBSPOT_ONE_PORTAL_LOCK_DIR:-${TMPDIR:-/tmp}/hubspot-account-membership.lock}"

token=''
HTTP_STATUS=''
RESPONSE_BODY=''
portal_guard_passed=false
typeset -a baseline_ids created_ids
baseline_ids=()
created_ids=()

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
    <<<"$RESPONSE_BODY" 2>/dev/null || print -- '<error body omitted>'
}

require_status() {
  local step="$1"
  shift
  local allowed
  for allowed in "$@"; do
    [[ "$HTTP_STATUS" == "$allowed" ]] && return 0
  done
  print -u2 -- "${step} returned ${HTTP_STATUS}; expected one of: $*"
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

delete_owned_quietly() {
  local id="$1"
  [[ -n "$id" ]] || return 0
  is_baseline_id "$id" && return 1
  request GET "/settings/users/2026-03/${id}"
  [[ "$HTTP_STATUS" == 404 ]] && return 0
  [[ "$HTTP_STATUS" == 200 ]] || return 1
  [[ "$(jq -r --arg email "$fixture_email" --arg id "$id" \
    '(.id | tostring) == $id and .email == $email and .superAdmin != true' \
    <<<"$RESPONSE_BODY")" == true ]] || return 1
  request DELETE "/settings/users/2026-03/${id}"
  [[ "$HTTP_STATUS" == 204 ]]
}

cleanup() {
  local id owned_ids
  set +eu
  trap - EXIT HUP INT TERM
  if [[ "$portal_guard_passed" == true ]]; then
    request GET '/settings/users/2026-03?limit=100'
    if [[ "$HTTP_STATUS" == 200 ]]; then
      owned_ids="$(jq -r --arg email "$fixture_email" \
        '.results[] | select(.email == $email and .superAdmin != true) | .id' \
        <<<"$RESPONSE_BODY")"
      for id in ${(f)owned_ids}; do
        delete_owned_quietly "$id"
      done
    fi
    for id in "${created_ids[@]}"; do
      delete_owned_quietly "$id"
    done
  fi
  unset token HUBSPOT_ACCESS_TOKEN RESPONSE_BODY
  rmdir "$lock_dir" 2>/dev/null || true
}

create_body() {
  jq -nc \
    --arg email "$fixture_email" \
    --arg firstName "$fixture_first_name" \
    --arg lastName "$fixture_last_name" \
    '{email: $email, firstName: $firstName, lastName: $lastName, sendWelcomeEmail: false}'
}

capture_created_response() {
  CREATED_ID="$(jq -r '(.id // "") | tostring' <<<"$RESPONSE_BODY")"
  [[ "$CREATED_ID" == <-> && "$CREATED_ID" != 0 ]]
  is_baseline_id "$CREATED_ID" && return 1
  created_ids+=("$CREATED_ID")
  print -- "create_id_present=true super_admin=$(jq -r '.superAdmin == true' <<<"$RESPONSE_BODY") welcome_email_requested=$(jq -r '.sendWelcomeEmail == true' <<<"$RESPONSE_BODY")"
}

create_fixture() {
  request_json POST '/settings/users/2026-03' "$(create_body)"
  require_status create_membership 201
  capture_created_response
}

recreate_fixture_after_removal() {
  local attempt email_read_status
  for attempt in {1..30}; do
    request_json POST '/settings/users/2026-03' "$(create_body)"
    if [[ "$HTTP_STATUS" == 201 ]]; then
      capture_created_response
      print -- "same_email_reuse_attempts=${attempt}"
      return 0
    fi
    if [[ "$HTTP_STATUS" != 400 || "$RESPONSE_BODY" != *NO_SEAT_AVAILABLE_FOR_ROLE_COMBINATION* ]]; then
      require_status recreate_membership 201
    fi
    request GET "/settings/users/2026-03/${fixture_email}?idProperty=EMAIL"
    email_read_status="$HTTP_STATUS"
    if [[ "$email_read_status" == 200 ]]; then
      capture_created_response
      print -- "same_email_reuse_attempts=${attempt} recovered_by_exact_email=true"
      return 0
    fi
    [[ "$email_read_status" == 404 ]]
    request GET '/settings/users/2026-03?limit=100'
    require_status reuse_collection_check 200
    [[ "$(jq -r --arg email "$fixture_email" \
      '[.results[] | select(.email == $email)] | length' <<<"$RESPONSE_BODY")" == 0 ]]
    sleep 1
  done
  print -u2 -- 'same-email reuse did not converge within 30 attempts'
  return 1
}

mkdir "$lock_dir" 2>/dev/null || {
  print -u2 -- "HubSpot portal is already in use: ${lock_dir}"
  exit 1
}
trap cleanup EXIT HUP INT TERM

[[ "$fixture_email" == tfhs-probe-16-*@example.com ]] || {
  print -u2 -- 'fixture email is outside the approved residual identity class'
  exit 1
}

if [[ -n "${HUBSPOT_ACCESS_TOKEN:-}" ]]; then
  token="$HUBSPOT_ACCESS_TOKEN"
  unset HUBSPOT_ACCESS_TOKEN
  credential_class='environment_static_token'
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
portal_guard_passed=true

print -- "execution_timestamp=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
print -- "credential_class=${credential_class}"
print -- "portal_fingerprint=${actual_fingerprint}"
print -- 'api=/settings/users/2026-03'
print -- 'welcome_email_policy=always_false'
print -- 'fixture_identity=reused_example.com_global_identity'
print -- 'privacy_policy=no_names_emails_or_ids'

request GET '/settings/users/2026-03?limit=100'
require_status opening_memberships 200
opening_count="$(jq -r '.results | length' <<<"$RESPONSE_BODY")"
opening_has_more="$(jq -r '.paging.next.after != null' <<<"$RESPONSE_BODY")"
opening_owned_count="$(jq -r --arg email "$fixture_email" \
  '[.results[] | select(.email == $email)] | length' <<<"$RESPONSE_BODY")"
baseline_ids=("${(@f)$(jq -r '.results[].id | tostring' <<<"$RESPONSE_BODY")}")
print -- "opening_count=${opening_count} opening_has_more=${opening_has_more} opening_owned_count=${opening_owned_count}"
[[ "$opening_count" == 1 && "$opening_has_more" == false && "$opening_owned_count" == 0 ]]

CREATED_ID=''
create_fixture
primary_id="$CREATED_ID"

request GET "/settings/users/2026-03/${primary_id}"
require_status read_by_id 200
identity_match="$(jq -r --arg id "$primary_id" --arg email "$fixture_email" \
  '(.id | tostring) == $id and .email == $email' <<<"$RESPONSE_BODY")"
assignment_count="$(jq -r \
  '([.roleId // ""] | map(select(length > 0)) | length) + ((.roleIds // []) | length) + ([.primaryTeamId // ""] | map(tostring) | map(select(length > 0)) | length) + ((.secondaryTeamIds // []) | length)' \
  <<<"$RESPONSE_BODY")"
print -- "read_by_id_identity_match=${identity_match} assignment_count=${assignment_count} subsequent_welcome_value=$(jq -r '.sendWelcomeEmail == true' <<<"$RESPONSE_BODY")"
[[ "$identity_match" == true ]]

request GET "/settings/users/2026-03/${fixture_email}?idProperty=EMAIL"
require_status read_by_email 200
print -- "read_by_email_identity_match=$(jq -r --arg id "$primary_id" '(.id | tostring) == $id' <<<"$RESPONSE_BODY")"

if [[ "$assignment_count" == 0 ]]; then
  name_body="$(jq -nc --arg firstName "$fixture_first_name" --arg lastName "$fixture_last_name" \
    '{firstName: $firstName, lastName: $lastName}')"
  request_json PUT "/settings/users/2026-03/${primary_id}" "$name_body"
  if [[ "$HTTP_STATUS" == 200 ]]; then
    print -- 'name_noop_status=200 identity_stable=true'
  elif [[ "$HTTP_STATUS" == 400 && "$RESPONSE_BODY" == *USER_NOT_ON_ANY_HUBS* ]]; then
    print -- 'name_noop_status=400 reason=USER_NOT_ON_ANY_HUBS'
  else
    require_status name_noop 200
  fi
else
  print -- 'name_noop_status=skipped_assignments_present'
fi

delete_owned_quietly "$primary_id"
print -- "first_delete_status=${HTTP_STATUS}"
require_status first_delete 204
request GET "/settings/users/2026-03/${primary_id}"
first_id_status="$HTTP_STATUS"
request GET "/settings/users/2026-03/${fixture_email}?idProperty=EMAIL"
first_email_status="$HTTP_STATUS"
print -- "first_absence_id_status=${first_id_status} first_absence_email_status=${first_email_status}"
[[ "$first_id_status" == 404 && "$first_email_status" == 404 ]]

recreate_fixture_after_removal
reuse_id="$CREATED_ID"
print -- "same_email_reuse_identity_stable=$([[ "$reuse_id" == "$primary_id" ]] && print true || print false)"
delete_owned_quietly "$reuse_id"
require_status reused_delete 204

attempt=0
for attempt in {1..20}; do
  request GET '/settings/users/2026-03?limit=100'
  require_status final_membership_poll 200
  owned_count="$(jq -r --arg email "$fixture_email" \
    '[.results[] | select(.email == $email)] | length' <<<"$RESPONSE_BODY")"
  [[ "$owned_count" == 0 ]] && break
  owned_ids="$(jq -r --arg email "$fixture_email" \
    '.results[] | select(.email == $email and .superAdmin != true) | .id' <<<"$RESPONSE_BODY")"
  for owned_id in ${(f)owned_ids}; do
    delete_owned_quietly "$owned_id"
  done
  sleep 1
done

request GET '/settings/users/2026-03?limit=100'
require_status final_memberships 200
final_count="$(jq -r '.results | length' <<<"$RESPONSE_BODY")"
final_owned_count="$(jq -r --arg email "$fixture_email" \
  '[.results[] | select(.email == $email)] | length' <<<"$RESPONSE_BODY")"
baseline_survivors="$(jq -r --argjson baseline "$(printf '%s\n' "${baseline_ids[@]}" | jq -R . | jq -s .)" \
  '[.results[] | . as $member | select($baseline | index(($member.id | tostring)) != null)] | length' \
  <<<"$RESPONSE_BODY")"
print -- "cleanup_attempts=${attempt} final_count=${final_count} final_owned_count=${final_owned_count} baseline_survivors=${baseline_survivors}"
[[ "$final_count" == 1 && "$final_owned_count" == 0 && "$baseline_survivors" == 1 ]]

portal_guard_passed=false
unset token RESPONSE_BODY
rmdir "$lock_dir"
trap - EXIT HUP INT TERM
print -- 'cleanup_result=passed'
