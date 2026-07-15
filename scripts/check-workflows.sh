#!/bin/sh
set -eu

required="ci.yml security.yml acceptance.yml acceptance-cleanup.yml release-candidate.yml release.yml verify-release.yml"

for name in $required; do
  test -f ".github/workflows/$name" || { echo "missing required workflow: $name" >&2; exit 1; }
done

for workflow in .github/workflows/*.yml; do
  grep -q '^permissions: {}' "$workflow" || { echo "workflow $workflow must start with empty permissions" >&2; exit 1; }
  if grep -E 'uses: [^.]' "$workflow" | grep -Ev 'uses: [^@]+@[0-9a-f]{40}([[:space:]]+#.*)?$' >/dev/null; then
    echo "external action is not pinned to a full commit in $workflow" >&2
    exit 1
  fi
  ! grep -Eq 'pull_request_target|workflow_run|secrets:[[:space:]]*inherit|self-hosted|vars\.RUNNER_LABEL' "$workflow" || { echo "unsafe workflow boundary in $workflow" >&2; exit 1; }
  ! grep -Eq 'run:.*\$\{\{[[:space:]]*github\.' "$workflow" || { echo "untrusted event interpolation in $workflow" >&2; exit 1; }
  grep -q 'timeout-minutes:' "$workflow" || { echo "workflow $workflow has no finite timeout" >&2; exit 1; }
  false_count=$(grep -c 'cancel-in-progress: false' "$workflow" || true)
  queue_count=$(grep -c 'queue: max' "$workflow" || true)
  test "$false_count" -eq "$queue_count" || { echo "non-canceling concurrency must use queue: max in $workflow" >&2; exit 1; }
done

grep -q '^  pull_request:' .github/workflows/ci.yml
grep -q '^  workflow_call:' .github/workflows/acceptance.yml
grep -q '^  schedule:' .github/workflows/security.yml
grep -q '^  workflow_dispatch:' .github/workflows/acceptance-cleanup.yml
if grep -q '^  schedule:' .github/workflows/acceptance-cleanup.yml; then
  echo "acceptance cleanup must be manual only" >&2
  exit 1
fi
grep -q 'verify-candidate-report.sh' .github/workflows/release.yml
grep -q 'goreleaser release --clean --parallelism=2 --skip=announce,publish,sign' .github/workflows/release.yml
grep -q '^[[:space:]]*@"$(TOOLS_BIN)/goreleaser" release --snapshot --clean --skip=sign$' Makefile || {
  echo "local release snapshots must not require signing credentials" >&2
  exit 1
}
if grep -q -- '--snapshot' .github/workflows/release.yml; then
  echo "production release must not use snapshot assets" >&2
  exit 1
fi
grep -q 'smoke-release-archive.sh' .github/workflows/release.yml
grep -q -- '--draft=false' .github/workflows/release.yml
grep -q -- '--draft=false --prerelease --latest=false' .github/workflows/release.yml
grep -q 'verify-released-provider.sh' .github/workflows/verify-release.yml
grep -q 'verify-state-migration.sh' .github/workflows/verify-release.yml

grep -q '"channel": "free-alpha"' release/surface.json
grep -q 'shard: \[free_properties\]' .github/workflows/acceptance.yml
if grep -Eq 'deal_pipelines|ticket_pipelines|custom_schemas|sensitive_properties|custom_pipelines' .github/workflows/acceptance.yml; then
  echo "Free alpha acceptance workflow includes an unreleased paid shard" >&2
  exit 1
fi
test "$(grep -c 'case_id: free_properties_' .github/workflows/verify-release.yml)" -eq 2 || {
  echo "Free alpha released verification must cover Free properties on both engines" >&2
  exit 1
}
if grep -Eq 'deal_pipelines|ticket_pipelines|custom_schemas|sensitive_properties|custom_pipelines' .github/workflows/verify-release.yml; then
  echo "Free alpha released verification includes an unreleased paid shard" >&2
  exit 1
fi
grep -q 'surface:"free-alpha"' .github/workflows/verify-release.yml
test "$(grep -c 'ref: refs/tags/\${{ inputs.version }}' .github/workflows/verify-release.yml)" -eq 4
grep -q 'refs/heads/release/free-alpha' .github/workflows/release-candidate.yml
grep -q -- '--arg surface "free-alpha"' .github/workflows/release-candidate.yml
grep -q '.surface == "free-alpha"' scripts/verify-candidate-report.sh
grep -q 'test "$version" = v0.1.0-alpha.1' scripts/release-preflight.sh
