#!/bin/sh
set -eu

root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
version=${1:?v-prefixed release version is required}
"$root/scripts/validate-release-version.sh" "$version"
version=${version#v}
if test -n "${REGISTRY_OBSERVATION_ENDPOINTS:-}"; then
  endpoints=$REGISTRY_OBSERVATION_ENDPOINTS
  request_timeout=${REGISTRY_OBSERVATION_REQUEST_TIMEOUT_SECONDS:-10}
else
  endpoints='https://registry.opentofu.org https://registry.terraform.io'
  request_timeout=10
fi
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM

classify_versions_response() {
  body=$1
  requested_version=$2

  if ! jq . "$body" >/dev/null 2>&1; then
    response_class=malformed-json
    return
  fi
  if ! jq -e 'type == "object" and ((.versions? | type) == "array")' "$body" >/dev/null 2>&1; then
    response_class=missing-versions
    return
  fi
  if ! jq -e '
    all(.versions[];
      type == "object" and
      ((.version? | type) == "string") and
      (.version | test("^(0|[1-9][0-9]*)\\.(0|[1-9][0-9]*)\\.(0|[1-9][0-9]*)(-(0|[1-9][0-9]*|[0-9A-Za-z-]*[A-Za-z-][0-9A-Za-z-]*)(\\.(0|[1-9][0-9]*|[0-9A-Za-z-]*[A-Za-z-][0-9A-Za-z-]*))*)?(\\+[0-9A-Za-z-]+(\\.[0-9A-Za-z-]+)*)?$"))
    )
  ' "$body" >/dev/null 2>&1; then
    response_class=invalid-version-records
    return
  fi
  if ! jq -e '([.versions[].version] | length) == ([.versions[].version] | unique | length)' "$body" >/dev/null 2>&1; then
    response_class=duplicate-version-records
    return
  fi
  if jq -e --arg version "$requested_version" 'any(.versions[]; .version == $version)' "$body" >/dev/null 2>&1; then
    response_class='version-present'
  else
    response_class='version-absent'
  fi
}

request_versions() {
  request_kind=$1
  endpoint=$2
  body=$3

  set -- --silent --max-time "$request_timeout" --output "$body" --write-out '%{http_code}'
  if test "$request_kind" = revalidation; then
    set -- "$@" --header 'Cache-Control: no-cache' --header 'Pragma: no-cache'
  fi

  curl_status=0
  http_status=$(curl "$@" -- "$endpoint/v1/providers/jackemcpherson/hubspot/versions") || curl_status=$?
  if test "$curl_status" -ne 0; then
    case "$curl_status" in
      28) response_class=timeout ;;
      *) response_class=transport-error ;;
    esac
    return
  fi
  case "$http_status" in
    2??) ;;
    *)
      response_class=http-status
      return
      ;;
  esac

  classify_versions_response "$body" "$version"
}

# The endpoint override is a hermetic-test seam. Production observes the two
# public registry hosts above through this exact loop and response contract.
for endpoint in $endpoints; do
  host=${endpoint#*://}
  host=${host%%/*}
  attempt=1
  confirmed=false
  last_response_class=not-observed
  while test "$attempt" -le 12; do
    response="$tmp/ordinary-$attempt.json"
    request_versions ordinary "$endpoint" "$response"
    ordinary_response_class=$response_class
    if test "$ordinary_response_class" = version-present; then
      confirmed=true
      break
    fi

    last_response_class="ordinary-$ordinary_response_class"
    if test "$ordinary_response_class" = version-absent; then
      response="$tmp/revalidation-$attempt.json"
      request_versions revalidation "$endpoint" "$response"
      last_response_class="$last_response_class,revalidation-$response_class"
    fi

    test "$attempt" -lt 12 || break
    sleep 10
    attempt=$((attempt + 1))
  done

  if test "$confirmed" != true; then
    echo "registry ingestion blocked for $host after 12 attempts: $last_response_class" >&2
    exit 1
  fi
  printf 'registry ingestion confirmed: host=%s version=%s\n' "$host" "$version"
done
