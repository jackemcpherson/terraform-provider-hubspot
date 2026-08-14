#!/bin/zsh

set -eu

readonly service="terraform-provider-hubspot-probes"
readonly origin="https://api.hubapi.com"
readonly expected_fingerprint="${HUBSPOT_PROBE_EXPECTED_FINGERPRINT:?set HUBSPOT_PROBE_EXPECTED_FINGERPRINT to the approved sha256 fingerprint}"
readonly execution_timestamp="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
readonly fixture_stamp="$(date -u +%Y%m%d%H%M%S)"
readonly prefix="tfhs_probe_16_${fixture_stamp}"
readonly fixture_email="${HUBSPOT_PROBE_16_FIXTURE_EMAIL:-tfhs-probe-16-${fixture_stamp}@example.com}"
readonly profile_properties='hs_internal_user_id,hs_object_id,hs_job_title,hs_availability_status,hs_standard_time_zone,hs_working_hours'
readonly lock_dir="${HUBSPOT_ONE_PORTAL_LOCK_DIR:-${TMPDIR:-/tmp}/hubspot-free-portal-${HUBSPOT_PORTAL_LOCK_ID:-default}.lock}"

token=""
HTTP_STATUS=""
RESPONSE_BODY=""
CAPTURED_ID=""
portal_guard_passed=false
profile_id=""
profile_restore_body=""
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
  jq -c '{
    status,
    category,
    subCategory,
    message: ((.message // "") | gsub($owned_prefix; "[owned-prefix]") | gsub($owned_email; "[owned-email]")),
    context: ((.context // {}) | keys),
    named_scope_candidates: ([
      "crm.objects.users.read",
      "crm.objects.users.write"
    ] | map(. as $scope | select($body | contains($scope))))
  }' --arg body "$RESPONSE_BODY" --arg owned_prefix "$prefix" --arg owned_email "$fixture_email" \
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

delete_owned_id_quietly() {
  local id="$1"
  [[ -n "$id" ]] || return 0
  is_baseline_id "$id" && return 1
  request GET "/settings/users/2026-03/${id}"
  [[ "$HTTP_STATUS" == 404 ]] && return 0
  [[ "$HTTP_STATUS" == 200 ]] || return 1
  [[ "$(jq -r --arg email "$fixture_email" '.email == $email and .superAdmin != true' <<<"$RESPONSE_BODY")" == true ]] || return 1
  request DELETE "/settings/users/2026-03/${id}"
}

cleanup() {
  local owned_ids id
  set +eu
  trap - EXIT HUP INT TERM
  if [[ "$portal_guard_passed" == true ]]; then
    if [[ -n "$profile_id" && -n "$profile_restore_body" ]]; then
      request_json PATCH "/crm/objects/2026-03/users/${profile_id}" "$profile_restore_body"
    fi
    request GET '/settings/users/2026-03?limit=100'
    if [[ "$HTTP_STATUS" == 200 ]]; then
      owned_ids="$(jq -r --arg fixture "$fixture_email" '.results[] | select(.email == $fixture) | .id' <<<"$RESPONSE_BODY")"
      for id in ${(f)owned_ids}; do
        delete_owned_id_quietly "$id"
      done
    fi
    for id in "${created_ids[@]}"; do
      delete_owned_id_quietly "$id"
    done
  fi
  unset token HUBSPOT_ACCESS_TOKEN RESPONSE_BODY
  rmdir "$lock_dir" 2>/dev/null || true
}

create_body() {
  local email="$1"
  local first_name="$2"
  local last_name="$3"

  jq -nc \
    --arg email "$email" \
    --arg firstName "$first_name" \
    --arg lastName "$last_name" \
    '{
      email: $email,
      sendWelcomeEmail: false,
      firstName: $firstName,
      lastName: $lastName
    }'
}

update_body() {
  local first_name="$1"
  local last_name="$2"

  jq -nc \
    --arg firstName "$first_name" \
    --arg lastName "$last_name" \
    '{firstName: $firstName, lastName: $lastName}'
}

capture_created_id() {
  CAPTURED_ID="$(jq -r '(.id // "") | tostring' <<<"$RESPONSE_BODY")"
  [[ -n "$CAPTURED_ID" ]] || {
    print -u2 -- 'create response returned no account-user ID'
    return 1
  }
  is_baseline_id "$CAPTURED_ID" && {
    print -u2 -- 'create returned a protected baseline ID; refusing ownership'
    return 1
  }
  created_ids+=("$CAPTURED_ID")
}

mkdir "$lock_dir" 2>/dev/null || {
  print -u2 -- "HubSpot Free portal is already in use: ${lock_dir}"
  exit 1
}
trap 'cleanup' EXIT HUP INT TERM

if [[ -n "${HUBSPOT_ACCESS_TOKEN:-}" ]]; then
  token="$HUBSPOT_ACCESS_TOKEN"
  credential_class='normal_free_ephemeral_static_token'
  unset HUBSPOT_ACCESS_TOKEN
else
  token="$(security find-generic-password -a "$USER" -s "$service" -w)"
  credential_class='normal_free_keychain_static_token'
fi

request GET '/account-info/v3/details'
require_status account_guard 200
portal_id="$(jq -r '.portalId' <<<"$RESPONSE_BODY")"
actual_fingerprint="sha256:$(printf '%s' "$portal_id" | shasum -a 256 | cut -c1-16)"
account_type="$(jq -r '.accountType // "unknown"' <<<"$RESPONSE_BODY")"
data_hosting="$(jq -r '.dataHostingLocation // "unknown"' <<<"$RESPONSE_BODY")"
unset portal_id RESPONSE_BODY
[[ "$actual_fingerprint" == "$expected_fingerprint" ]] || {
  print -u2 -- 'account fingerprint mismatch; refusing probe'
  exit 1
}
portal_guard_passed=true

print -- "execution_timestamp=${execution_timestamp}"
print -- 'snapshot_date=2026-08-02'
print -- "credential_class=${credential_class}"
print -- "portal_fingerprint=${actual_fingerprint}"
print -- "account_type=${account_type}"
print -- "data_hosting=${data_hosting}"
print -- 'api_versions=/settings/users/2026-03,/crm/objects/2026-03/users'
print -- 'required_read_scope=crm.objects.users.read'
print -- 'required_write_scope=crm.objects.users.write'
print -- "fixture_prefix=${prefix}"
print -- 'fixture_email_class=unique_reserved_example.com_non_delivery_address'
print -- 'welcome_email_policy=always_false'
print -- 'mutation_policy=owned_fixture_only_baseline_ids_protected'
print -- 'self_removal_policy=never_send_delete_for_any_opening_baseline_user'
print -- 'paid_boundaries=teams_and_reusable_permission_sets_excluded'

request GET '/settings/users/2026-03?limit=100'
print -- "step=opening_account_users status=${HTTP_STATUS}"
require_status opening_account_users 200
opening_count="$(jq -r '.results | length' <<<"$RESPONSE_BODY")"
opening_owned_count="$(jq -r --arg fixture "$fixture_email" '[.results[] | select(.email == $fixture)] | length' <<<"$RESPONSE_BODY")"
baseline_ids=("${(@f)$(jq -r '.results[].id | tostring' <<<"$RESPONSE_BODY")}")
print -- "opening_user_count=${opening_count} opening_owned_count=${opening_owned_count} baseline_protected_count=${#baseline_ids[@]} has_more=$(jq -r '.paging.next.after != null' <<<"$RESPONSE_BODY")"
[[ "$opening_count" == 1 && "$opening_owned_count" == 0 ]] || {
  print -u2 -- 'expected exactly one baseline user and no owned fixture; refusing mutation'
  exit 1
}

baseline_guard_result='passed'
if delete_owned_id_quietly "${baseline_ids[1]}"; then
  baseline_guard_result='failed'
fi
print -- "step=local_self_removal_guard result=${baseline_guard_result} network_request_sent=false"
[[ "$baseline_guard_result" == passed ]]

request GET '/settings/users/2026-03/roles'
print -- "step=opening_permission_sets status=${HTTP_STATUS}"
if [[ "$HTTP_STATUS" == 200 ]]; then
  print -- "permission_set_count=$(jq -r '.results | length' <<<"$RESPONSE_BODY") assignable_without_billing_write_count=$(jq -r '[.results[] | select(.requiresBillingWrite != true)] | length' <<<"$RESPONSE_BODY") paid_seat_permission_set_count=$(jq -r '[.results[] | select(.requiresBillingWrite == true)] | length' <<<"$RESPONSE_BODY")"
elif [[ "$HTTP_STATUS" == 400 ]]; then
  print -- 'permission_set_discovery=unavailable_on_free validation_error=true'
  print_error_shape
else
  require_status opening_permission_sets 200 400
fi
print -- 'permission_assignment_policy=discovery_only_no_known_safe_role_fixture'

request GET "/crm/objects/2026-03/users?limit=100&archived=false&properties=${profile_properties}"
print -- "step=opening_active_user_profiles status=${HTTP_STATUS}"
require_status opening_active_user_profiles 200
opening_active_profile_count="$(jq -r '.results | length' <<<"$RESPONSE_BODY")"
request GET "/crm/objects/2026-03/users?limit=100&archived=true&properties=${profile_properties}"
print -- "step=opening_archived_user_profiles status=${HTTP_STATUS}"
if [[ "$HTTP_STATUS" == 200 ]]; then
  opening_archived_profile_count="$(jq -r '.results | length' <<<"$RESPONSE_BODY")"
elif [[ "$HTTP_STATUS" == 400 ]]; then
  opening_archived_profile_count='unsupported'
  print_error_shape
else
  require_status opening_archived_user_profiles 200 400
fi
print -- "opening_active_profile_count=${opening_active_profile_count} opening_archived_profile_count=${opening_archived_profile_count}"

initial_first='T16First'
initial_last='T16Last'
request_json POST '/settings/users/2026-03' "$(create_body "$fixture_email" "$initial_first" "$initial_last")"
print -- "step=create_account_user status=${HTTP_STATUS}"
require_status create_account_user 201
capture_created_id
primary_id="$CAPTURED_ID"
print -- "create_id_present=true super_admin=$(jq -r '.superAdmin == true' <<<"$RESPONSE_BODY") welcome_email_sent=$(jq -r '.sendWelcomeEmail == true' <<<"$RESPONSE_BODY") names_match=$(jq -r --arg first "$initial_first" --arg last "$initial_last" '.firstName == $first and .lastName == $last' <<<"$RESPONSE_BODY") response_keys=$(jq -c 'keys | sort' <<<"$RESPONSE_BODY")"

request GET "/settings/users/2026-03/${primary_id}"
print -- "step=read_account_user_by_id status=${HTTP_STATUS}"
require_status read_account_user_by_id 200
print -- "id_round_trip=$(jq -r --arg id "$primary_id" '(.id | tostring) == $id' <<<"$RESPONSE_BODY") email_match=$(jq -r --arg email "$fixture_email" '.email == $email' <<<"$RESPONSE_BODY") subsequent_welcome_email_value=$(jq -r '.sendWelcomeEmail == true' <<<"$RESPONSE_BODY")"

request GET "/settings/users/2026-03/${fixture_email}?idProperty=EMAIL"
print -- "step=read_account_user_by_email status=${HTTP_STATUS}"
require_status read_account_user_by_email 200
print -- "email_import_identity_matches=$(jq -r --arg id "$primary_id" '(.id | tostring) == $id' <<<"$RESPONSE_BODY")"

print -- 'duplicate_create_attempted=false existing_email_requires_explicit_import_or_replacement'

initial_update="$(update_body "$initial_first" "$initial_last")"
request_json PUT "/settings/users/2026-03/${primary_id}" "$initial_update"
print -- "step=semantic_noop_account_user_update status=${HTTP_STATUS}"
if [[ "$HTTP_STATUS" == 200 ]]; then
  print -- "noop_identity_stable=$(jq -r --arg id "$primary_id" '(.id | tostring) == $id' <<<"$RESPONSE_BODY") names_stable=$(jq -r --arg first "$initial_first" --arg last "$initial_last" '.firstName == $first and .lastName == $last' <<<"$RESPONSE_BODY")"

  intended_first='T16Updated'
  intended_last='T16User'
  intended_update="$(update_body "$intended_first" "$intended_last")"
  request_json PUT "/settings/users/2026-03/${primary_id}" "$intended_update"
  print -- "step=update_account_user status=${HTTP_STATUS}"
  require_status update_account_user 200
  print -- "update_identity_stable=$(jq -r --arg id "$primary_id" '(.id | tostring) == $id' <<<"$RESPONSE_BODY") names_changed=$(jq -r --arg first "$intended_first" --arg last "$intended_last" '.firstName == $first and .lastName == $last' <<<"$RESPONSE_BODY") role_assignment_attempted=false team_assignment_attempted=false"

  drift_first='T16Drift'
  drift_last='T16Observed'
  request_json PUT "/settings/users/2026-03/${primary_id}" "$(update_body "$drift_first" "$drift_last")"
  print -- "step=inject_owned_account_user_drift status=${HTTP_STATUS}"
  require_status inject_owned_account_user_drift 200
  request GET "/settings/users/2026-03/${primary_id}"
  print -- "step=observe_owned_account_user_drift status=${HTTP_STATUS}"
  require_status observe_owned_account_user_drift 200
  print -- "drift_observed=$(jq -r --arg first "$drift_first" --arg last "$drift_last" '.firstName == $first and .lastName == $last' <<<"$RESPONSE_BODY")"
  request_json PUT "/settings/users/2026-03/${primary_id}" "$intended_update"
  print -- "step=restore_intended_account_user_state status=${HTTP_STATUS}"
  require_status restore_intended_account_user_state 200
elif [[ "$HTTP_STATUS" == 400 && "$RESPONSE_BODY" == *USER_NOT_ON_ANY_HUBS* ]]; then
  print -- 'account_user_name_update=human_acceptance_gated reason=USER_NOT_ON_ANY_HUBS'
  print_error_shape
else
  require_status semantic_noop_account_user_update 200
fi

print -- 'free_user_quota=2 evidence=official_product_catalogue third_user_attempted=false'

request GET "/crm/objects/2026-03/users?limit=100&properties=${profile_properties}"
print -- "step=discover_created_user_profile status=${HTTP_STATUS}"
require_status discover_created_user_profile 200
profile_match_count="$(jq -r --arg internal "$primary_id" '[.results[] | select(((.properties.hs_internal_user_id // "") | tostring) == $internal)] | length' <<<"$RESPONSE_BODY")"
profile_id="$(jq -r --arg internal "$primary_id" '[.results[] | select(((.properties.hs_internal_user_id // "") | tostring) == $internal)][0].id // ""' <<<"$RESPONSE_BODY")"
print -- "account_user_profile_match_count=${profile_match_count} profile_mutation_possible=$([[ "$profile_match_count" == 1 ]] && print true || print false)"

if [[ "$profile_match_count" == 1 ]]; then
  desired_working_hours='[{"days":"MONDAY_TO_FRIDAY","startMinute":540,"endMinute":1020}]'
  profile_restore_body="$(jq -c --arg internal "$primary_id" '[.results[] | select(((.properties.hs_internal_user_id // "") | tostring) == $internal)][0].properties | {properties: {
    hs_job_title: (.hs_job_title // ""),
    hs_availability_status: (.hs_availability_status // ""),
    hs_standard_time_zone: (.hs_standard_time_zone // ""),
    hs_working_hours: (.hs_working_hours // "")
  }}' <<<"$RESPONSE_BODY")"
  profile_prerequisites="$(jq -nc --arg title "${prefix}_job" '{properties: {hs_job_title: $title, hs_availability_status: "away", hs_standard_time_zone: "Australia/Melbourne"}}')"
  desired_profile="$(jq -nc --arg title "${prefix}_job" --arg hours "$desired_working_hours" '{properties: {hs_job_title: $title, hs_availability_status: "away", hs_standard_time_zone: "Australia/Melbourne", hs_working_hours: $hours}}')"
  request_json PATCH "/crm/objects/2026-03/users/${profile_id}" "$profile_prerequisites"
  print -- "step=set_created_user_profile_prerequisites status=${HTTP_STATUS}"
  require_status set_created_user_profile_prerequisites 200
  request_json PATCH "/crm/objects/2026-03/users/${profile_id}" "$(jq -nc --arg hours "$desired_working_hours" '{properties: {hs_working_hours: $hours}}')"
  print -- "step=set_created_user_working_hours status=${HTTP_STATUS}"
  require_status set_created_user_working_hours 200
  request GET "/crm/objects/2026-03/users/${profile_id}?properties=${profile_properties}"
  print -- "step=read_created_user_profile status=${HTTP_STATUS}"
  require_status read_created_user_profile 200
  profile_updated_at="$(jq -r '.updatedAt // ""' <<<"$RESPONSE_BODY")"
  print -- "profile_identity_stable=$(jq -r --arg id "$profile_id" '(.id | tostring) == $id' <<<"$RESPONSE_BODY") profile_values_match=$(jq -r --arg title "${prefix}_job" --arg hours "$desired_working_hours" '.properties.hs_job_title == $title and .properties.hs_availability_status == "away" and .properties.hs_standard_time_zone == "Australia/Melbourne" and .properties.hs_working_hours == $hours' <<<"$RESPONSE_BODY")"

  request_json PATCH "/crm/objects/2026-03/users/${profile_id}" "$desired_profile"
  print -- "step=semantic_noop_created_user_profile status=${HTTP_STATUS}"
  require_status semantic_noop_created_user_profile 200
  print -- "profile_noop_identity_stable=$(jq -r --arg id "$profile_id" '(.id | tostring) == $id' <<<"$RESPONSE_BODY") profile_noop_updated_at_changed=$(jq -r --arg prior "$profile_updated_at" '(.updatedAt // "") != $prior' <<<"$RESPONSE_BODY")"

  drift_profile="$(jq -nc --arg title "${prefix}_drift_job" '{properties: {hs_job_title: $title, hs_availability_status: "available"}}')"
  request_json PATCH "/crm/objects/2026-03/users/${profile_id}" "$drift_profile"
  print -- "step=inject_owned_user_profile_drift status=${HTTP_STATUS}"
  require_status inject_owned_user_profile_drift 200
  request GET "/crm/objects/2026-03/users/${profile_id}?properties=${profile_properties}"
  print -- "step=observe_owned_user_profile_drift status=${HTTP_STATUS}"
  require_status observe_owned_user_profile_drift 200
  print -- "profile_drift_observed=$(jq -r --arg title "${prefix}_drift_job" '.properties.hs_job_title == $title and .properties.hs_availability_status == "available"' <<<"$RESPONSE_BODY")"
  request_json PATCH "/crm/objects/2026-03/users/${profile_id}" "$profile_restore_body"
  print -- "step=restore_original_user_profile_state status=${HTTP_STATUS}"
  require_status restore_original_user_profile_state 200
  profile_restore_body=''
else
  print -- 'profile_lifecycle=unavailable_before_invitation_acceptance_or_profile_materialization'
fi

delete_owned_id_quietly "$primary_id"
print -- "step=deprovision_account_user status=${HTTP_STATUS}"
require_status deprovision_account_user 204
request GET "/settings/users/2026-03/${primary_id}"
print -- "step=read_deprovisioned_account_user_by_id status=${HTTP_STATUS}"
request GET "/settings/users/2026-03/${fixture_email}?idProperty=EMAIL"
print -- "step=read_deprovisioned_account_user_by_email status=${HTTP_STATUS}"

request DELETE "/settings/users/2026-03/${primary_id}"
print -- "step=repeat_deprovision_account_user status=${HTTP_STATUS}"

reuse_body="$(create_body "$fixture_email" 'T16Reuse' 'T16User')"
request_json POST '/settings/users/2026-03' "$reuse_body"
print -- "step=reprovision_same_email status=${HTTP_STATUS}"
if [[ "$HTTP_STATUS" == 201 ]]; then
  capture_created_id
  reuse_id="$CAPTURED_ID"
  print -- "same_email_reprovisioned=true identity_reused=$([[ "$reuse_id" == "$primary_id" ]] && print true || print false)"
  delete_owned_id_quietly "$reuse_id"
  print -- "step=deprovision_reused_account_user status=${HTTP_STATUS}"
  require_status deprovision_reused_account_user 204
else
  print -- 'same_email_reprovisioned=false'
  print_error_shape
fi

final_cleanup_attempts=0
for final_cleanup_attempts in {1..20}; do
  request GET '/settings/users/2026-03?limit=100'
  require_status verify_final_account_users 200
  final_poll_owned_count="$(jq -r --arg fixture "$fixture_email" '[.results[] | select(.email == $fixture)] | length' <<<"$RESPONSE_BODY")"
  [[ "$final_poll_owned_count" == 0 ]] && break
  final_poll_ids=("${(@f)$(jq -r --arg fixture "$fixture_email" '.results[] | select(.email == $fixture and .superAdmin != true) | .id' <<<"$RESPONSE_BODY")}")
  for final_poll_id in "${final_poll_ids[@]}"; do
    [[ -n "$final_poll_id" ]] || continue
    delete_owned_id_quietly "$final_poll_id"
  done
  sleep 1
done
request GET '/settings/users/2026-03?limit=100'
print -- "step=verify_final_account_users status=${HTTP_STATUS} attempts=${final_cleanup_attempts}"
require_status verify_final_account_users 200
final_count="$(jq -r '.results | length' <<<"$RESPONSE_BODY")"
final_owned_count="$(jq -r --arg fixture "$fixture_email" '[.results[] | select(.email == $fixture)] | length' <<<"$RESPONSE_BODY")"
baseline_survivor_count="$(jq -r --argjson baseline "$(printf '%s\n' "${baseline_ids[@]}" | jq -R . | jq -s .)" '[.results[] | . as $user | select($baseline | index(($user.id | tostring)) != null)] | length' <<<"$RESPONSE_BODY")"

request GET "/crm/objects/2026-03/users?limit=100&archived=false&properties=${profile_properties}"
print -- "step=verify_final_active_user_profiles status=${HTTP_STATUS}"
require_status verify_final_active_user_profiles 200
final_active_profile_count="$(jq -r '.results | length' <<<"$RESPONSE_BODY")"
request GET "/crm/objects/2026-03/users?limit=100&archived=true&properties=${profile_properties}"
print -- "step=verify_final_archived_user_profiles status=${HTTP_STATUS}"
if [[ "$HTTP_STATUS" == 200 ]]; then
  final_archived_profile_count="$(jq -r '.results | length' <<<"$RESPONSE_BODY")"
elif [[ "$HTTP_STATUS" == 400 ]]; then
  final_archived_profile_count='unsupported'
  print_error_shape
else
  require_status verify_final_archived_user_profiles 200 400
fi

print -- "cleanup_result=$([[ "$final_owned_count" == 0 && "$baseline_survivor_count" == "${#baseline_ids[@]}" ]] && print passed || print failed) final_user_count=${final_count} owned_active_users=${final_owned_count} protected_baseline_survivors=${baseline_survivor_count} opening_active_profiles=${opening_active_profile_count} final_active_profiles=${final_active_profile_count} opening_archived_profiles=${opening_archived_profile_count} final_archived_profiles=${final_archived_profile_count}"
if [[ "$final_owned_count" == 0 && "$baseline_survivor_count" == "${#baseline_ids[@]}" ]]; then
  portal_guard_passed=false
  unset token HUBSPOT_ACCESS_TOKEN RESPONSE_BODY
  rmdir "$lock_dir"
  trap - EXIT HUP INT TERM
  exit 0
fi
exit 1
