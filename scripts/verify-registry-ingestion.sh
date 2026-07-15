#!/bin/sh
set -eu

version=${1:?version is required}
version=${version#v}
for host in registry.opentofu.org registry.terraform.io; do
  attempt=1
  while test "$attempt" -le 12; do
    if curl --fail --silent --show-error "https://$host/v1/providers/jackemcpherson/hubspot/versions" | jq -e --arg version "$version" '.versions[] | select(.version == $version)' >/dev/null; then
      break
    fi
    test "$attempt" -lt 12 || { echo "registry ingestion timed out for $host" >&2; exit 1; }
    sleep 10
    attempt=$((attempt + 1))
  done
done
