#!/bin/zsh

set -eu

readonly service='terraform-provider-hubspot-probes'
readonly origin='https://api.hubapi.com'
readonly expected_fingerprint="${HUBSPOT_PROBE_EXPECTED_FINGERPRINT:?set HUBSPOT_PROBE_EXPECTED_FINGERPRINT to the approved sha256 fingerprint}"

token="$(security find-generic-password -a "$USER" -s "$service" -w)"
HTTP_STATUS=''
RESPONSE_BODY=''

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

cleanup() {
  unset token RESPONSE_BODY
}
trap 'cleanup' EXIT HUP INT TERM

request GET '/account-info/v3/details'
[[ "$HTTP_STATUS" == 200 ]]
portal_id="$(jq -r '.portalId' <<<"$RESPONSE_BODY")"
actual_fingerprint="sha256:$(printf '%s' "$portal_id" | shasum -a 256 | cut -c1-16)"
unset portal_id
[[ "$actual_fingerprint" == "$expected_fingerprint" ]] || {
  print -u2 -- 'account fingerprint mismatch; refusing cleanup'
  exit 1
}

request GET '/settings/users/2026-03?limit=100'
[[ "$HTTP_STATUS" == 200 ]]
owned_ids=("${(@f)$(jq -r '.results[] | select((.email | startswith("tfhs-probe-16-")) and (.email | endswith("@example.com")) and .superAdmin != true) | .id' <<<"$RESPONSE_BODY")}")
opening_owned_count="${#owned_ids[@]}"

for id in "${owned_ids[@]}"; do
  [[ -n "$id" ]] || continue
  request GET "/settings/users/2026-03/${id}"
  [[ "$HTTP_STATUS" == 200 ]]
  [[ "$(jq -r '.email | startswith("tfhs-probe-16-") and endswith("@example.com")' <<<"$RESPONSE_BODY")" == true ]]
  [[ "$(jq -r '.superAdmin != true' <<<"$RESPONSE_BODY")" == true ]]
  request DELETE "/settings/users/2026-03/${id}"
  [[ "$HTTP_STATUS" == 204 ]]
done

request GET '/settings/users/2026-03?limit=100'
[[ "$HTTP_STATUS" == 200 ]]
final_owned_count="$(jq -r '[.results[] | select((.email | startswith("tfhs-probe-16-")) and (.email | endswith("@example.com")))] | length' <<<"$RESPONSE_BODY")"
print -- "portal_fingerprint=${actual_fingerprint} opening_owned_users=${opening_owned_count} final_owned_users=${final_owned_count} cleanup_result=$([[ "$final_owned_count" == 0 ]] && print passed || print failed)"
[[ "$final_owned_count" == 0 ]]
