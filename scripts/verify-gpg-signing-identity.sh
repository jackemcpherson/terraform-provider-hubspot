#!/bin/sh
set -eu

registered_fingerprint=${1:?registered signing fingerprint is required}
status_file=${2:?GPG status file is required}
subject=${3:?signed evidence description is required}

printf '%s\n' "$registered_fingerprint" | grep -Eq '^[0-9A-Fa-f]{40}$' || {
	echo 'registered signing fingerprint must contain 40 hexadecimal characters' >&2
	exit 1
}
registered_fingerprint=$(printf '%s' "$registered_fingerprint" | tr '[:lower:]' '[:upper:]')

if ! awk -v expected="$registered_fingerprint" '
	$1 == "[GNUPG:]" && $2 == "VALIDSIG" {
		for (field = 3; field <= NF; field++) {
			if (toupper($field) == expected) valid = 1
		}
	}
	END { if (!valid) exit 1 }
' "$status_file"; then
	echo "$subject was not signed by the registered signing identity" >&2
	exit 1
fi
