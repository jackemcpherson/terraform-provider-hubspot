#!/bin/sh
set -eu

version=${1:?release version is required}
changelog=${2:-CHANGELOG.md}
heading="## [${version#v}]"

awk -v heading="$heading" '
  index($0, heading) == 1 { found=1 }
  found && /^## \[/ && index($0, heading) != 1 { exit }
  found { print }
  END { if (!found) exit 1 }
' "$changelog"
