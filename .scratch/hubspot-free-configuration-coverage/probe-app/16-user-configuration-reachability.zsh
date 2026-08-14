#!/bin/zsh

set -eu

readonly service="terraform-provider-hubspot-probes"
readonly origin="https://api.hubapi.com"
readonly expected_fingerprint="${HUBSPOT_PROBE_EXPECTED_FINGERPRINT:?set HUBSPOT_PROBE_EXPECTED_FINGERPRINT to the approved sha256 fingerprint}"
readonly execution_timestamp="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
readonly lock_dir="${HUBSPOT_ONE_PORTAL_LOCK_DIR:-${TMPDIR:-/tmp}/hubspot-free-portal-${HUBSPOT_PORTAL_LOCK_ID:-default}.lock}"

token=""
HTTP_STATUS=""
RESPONSE_BODY=""

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

print_error_shape() {
  jq -c '{
    status,
    category,
    subCategory,
    context: ((.context // {}) | keys),
    named_scope_candidates: ([
      "crm.objects.users.read",
      "crm.objects.users.write"
    ] | map(. as $scope | select($body | contains($scope))))
  }' --arg body "$RESPONSE_BODY" \
    <<<"$RESPONSE_BODY" 2>/dev/null || print -- '<error body omitted>'
}

cleanup() {
  unset token HUBSPOT_ACCESS_TOKEN RESPONSE_BODY account_user_ids
  rmdir "$lock_dir" 2>/dev/null || true
}

mkdir "$lock_dir" 2>/dev/null || {
  print -u2 -- "HubSpot Free portal is already in use: ${lock_dir}"
  exit 1
}
trap cleanup EXIT HUP INT TERM

if [[ -n "${HUBSPOT_ACCESS_TOKEN:-}" ]]; then
  token="$HUBSPOT_ACCESS_TOKEN"
  credential_class='normal_free_ephemeral_static_token'
  unset HUBSPOT_ACCESS_TOKEN
else
  token="$(security find-generic-password -a "$USER" -s "$service" -w)"
  credential_class='normal_free_keychain_static_token'
fi

request GET '/account-info/v3/details'
[[ "$HTTP_STATUS" == 200 ]] || {
  print -u2 -- "account guard returned ${HTTP_STATUS}; expected 200"
  exit 1
}
portal_id="$(jq -r '.portalId' <<<"$RESPONSE_BODY")"
actual_fingerprint="sha256:$(printf '%s' "$portal_id" | shasum -a 256 | cut -c1-16)"
account_type="$(jq -r '.accountType // "unknown"' <<<"$RESPONSE_BODY")"
data_hosting="$(jq -r '.dataHostingLocation // "unknown"' <<<"$RESPONSE_BODY")"
unset portal_id RESPONSE_BODY
[[ "$actual_fingerprint" == "$expected_fingerprint" ]] || {
  print -u2 -- 'account fingerprint mismatch; refusing probe'
  exit 1
}

print -- "execution_timestamp=${execution_timestamp}"
print -- 'snapshot_date=2026-08-02'
print -- "credential_class=${credential_class}"
print -- "portal_fingerprint=${actual_fingerprint}"
print -- "account_type=${account_type}"
print -- "data_hosting=${data_hosting}"
print -- 'api_versions=/settings/users/2026-03,/crm/objects/2026-03/users'
print -- 'required_read_scope=crm.objects.users.read'
print -- 'required_lifecycle_write_scope=crm.objects.users.write'
print -- 'mutation_policy=none_reachability_only'
print -- 'privacy_policy=no_names_emails_ids_or_permission_details'
print -- 'paid_boundaries=teams_and_reusable_permission_sets_excluded'

account_user_ids='[]'
request GET '/settings/users/2026-03?limit=100'
print -- "step=list_account_users method=GET route=/settings/users/2026-03 status=${HTTP_STATUS}"
if [[ "$HTTP_STATUS" == 200 ]]; then
  account_user_ids="$(jq -c '[.results[].id | tostring]' <<<"$RESPONSE_BODY")"
  jq -c '{
    count: (.results | length),
    has_more: (.paging.next.after != null),
    super_admin_count: ([.results[] | select(.superAdmin == true)] | length),
    role_assigned_count: ([.results[] | select(((.roleIds // []) | length) > 0 or ((.roleId // "") | length) > 0)] | length),
    primary_team_assigned_count: ([.results[] | select(((.primaryTeamId // "") | tostring | length) > 0)] | length),
    secondary_team_assigned_count: ([.results[] | select(((.secondaryTeamIds // []) | length) > 0)] | length),
    pending_or_acceptance_field_present: ([.results[] | keys[] | select(. == "pending" or . == "status" or . == "accepted")] | length > 0),
    response_keys: ([.results[] | keys[]] | unique)
  }' <<<"$RESPONSE_BODY"
else
  print_error_shape
fi

request GET '/settings/users/2026-03/roles'
print -- "step=list_permission_sets method=GET route=/settings/users/2026-03/roles status=${HTTP_STATUS}"
if [[ "$HTTP_STATUS" == 200 ]]; then
  jq -c '{
    count: (.results | length),
    assignable_without_billing_write_count: ([.results[] | select(.requiresBillingWrite != true)] | length),
    requires_billing_write_count: ([.results[] | select(.requiresBillingWrite == true)] | length),
    response_keys: ([.results[] | keys[]] | unique)
  }' <<<"$RESPONSE_BODY"
else
  print_error_shape
fi

request GET '/crm/properties/2026-03/user?archived=false'
print -- "step=list_user_profile_property_definitions method=GET route=/crm/properties/2026-03/user status=${HTTP_STATUS}"
if [[ "$HTTP_STATUS" == 200 ]]; then
  jq -c '{
    documented_mutable_property_count: ([.results[] | select(.name == "hs_additional_phone" or .name == "hs_availability_status" or .name == "hs_job_title" or .name == "hs_main_user_language_skill" or .name == "hs_out_of_office_hours" or .name == "hs_secondary_user_language_skill" or .name == "hs_standard_time_zone" or .name == "hs_uncategorized_skills" or .name == "hs_working_hours")] | length),
    documented_mutable_properties: [.results[] | select(.name == "hs_additional_phone" or .name == "hs_availability_status" or .name == "hs_job_title" or .name == "hs_main_user_language_skill" or .name == "hs_out_of_office_hours" or .name == "hs_secondary_user_language_skill" or .name == "hs_standard_time_zone" or .name == "hs_uncategorized_skills" or .name == "hs_working_hours") | {
      name,
      type,
      fieldType,
      readOnlyValue: .modificationMetadata.readOnlyValue
    }] | sort_by(.name),
    internal_identity_property_present: ([.results[] | select(.name == "hs_internal_user_id")] | length == 1)
  }' <<<"$RESPONSE_BODY"
else
  print_error_shape
fi

request GET '/crm/objects/2026-03/users?limit=100&properties=hs_internal_user_id,hs_object_id,hs_job_title,hs_additional_phone,hs_availability_status,hs_main_user_language_skill,hs_secondary_user_language_skill,hs_out_of_office_hours,hs_standard_time_zone,hs_uncategorized_skills,hs_working_hours'
print -- "step=list_user_profiles method=GET route=/crm/objects/2026-03/users status=${HTTP_STATUS}"
if [[ "$HTTP_STATUS" == 200 ]]; then
  jq -c --argjson account_ids "$account_user_ids" '{
    count: (.results | length),
    has_more: (.paging.next.after != null),
    account_user_identity_match_count: ([.results[] | . as $user | select($account_ids | index(($user.properties.hs_internal_user_id // "") | tostring) != null)] | length),
    object_id_invariant_count: ([.results[] | select((.id | tostring) == ((.properties.hs_object_id // "") | tostring))] | length),
    timezone_configured_count: ([.results[] | select(((.properties.hs_standard_time_zone // "") | length) > 0)] | length),
    working_hours_configured_count: ([.results[] | select(((.properties.hs_working_hours // "") | length) > 0)] | length),
    out_of_office_configured_count: ([.results[] | select(((.properties.hs_out_of_office_hours // "") | length) > 0)] | length),
    availability_configured_count: ([.results[] | select(((.properties.hs_availability_status // "") | length) > 0)] | length),
    envelope_keys: ([.results[] | keys[]] | unique),
    property_keys: ([.results[].properties | keys[]] | unique)
  }' <<<"$RESPONSE_BODY"
else
  print_error_shape
fi

print -- 'cleanup_result=passed_no_mutations_performed'
