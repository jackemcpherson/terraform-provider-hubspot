#!/bin/sh
set -eu

commit=${1:?full candidate commit is required}
repository=${2:?repository is required}
case "$commit" in *[!0-9a-f]*|'') echo "candidate commit must be lowercase hexadecimal" >&2; exit 1 ;; esac
test "${#commit}" -eq 40 || { echo "candidate commit must be a full SHA" >&2; exit 1; }
: "${GH_TOKEN:?GH_TOKEN is required}"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
default_branch=$(gh repo view "$repository" --json defaultBranchRef --jq '.defaultBranchRef.name')
test -n "$default_branch"

for run_id in $(gh run list --repo "$repository" --workflow release-candidate.yml --branch "$default_branch" --status success --limit 100 --json databaseId --jq '.[].databaseId'); do
  rm -rf "$tmp/report"
  mkdir -p "$tmp/report"
  if gh run download "$run_id" --repo "$repository" --name "candidate-$commit" --dir "$tmp/report" >/dev/null 2>&1 &&
     jq -e --arg commit "$commit" '.commit == $commit and ([.gates[]] | all(. == "success"))' "$tmp/report/candidate-report.json" >/dev/null; then
    exit 0
  fi
done

echo "no successful candidate report is bound to $commit" >&2
exit 1
