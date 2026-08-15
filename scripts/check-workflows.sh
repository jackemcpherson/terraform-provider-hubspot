#!/bin/sh
set -eu

for workflow in .github/workflows/*.yml; do
  grep -q '^permissions: {}' "$workflow" || {
    echo "workflow $workflow must start with empty permissions" >&2
    exit 1
  }
  grep -q 'timeout-minutes:' "$workflow" || {
    echo "workflow $workflow has no finite timeout" >&2
    exit 1
  }
  grep -q 'runs-on: ubuntu-24.04' "$workflow" || {
    echo "workflow $workflow must pin the hosted runner image" >&2
    exit 1
  }
  ! grep -q 'ubuntu-latest' "$workflow" || {
    echo "workflow $workflow must not use ubuntu-latest" >&2
    exit 1
  }
  if grep -E 'uses: [^.]' "$workflow" | grep -Ev 'uses: [^@]+@[0-9a-f]{40}([[:space:]]+#.*)?$' >/dev/null; then
    echo "external action is not pinned to a full commit in $workflow" >&2
    exit 1
  fi
  ! grep -Eq 'pull_request_target|workflow_run|secrets:[[:space:]]*inherit|self-hosted|vars\.RUNNER_LABEL' "$workflow" || {
    echo "unsafe workflow boundary in $workflow" >&2
    exit 1
  }
  ! grep -Eq 'run:.*\$\{\{[[:space:]]*github\.' "$workflow" || {
    echo "untrusted event interpolation in $workflow" >&2
    exit 1
  }
  if grep -Eq '^[[:space:]]+- uses:' "$workflow"; then
    echo "every action step must have a descriptive name in $workflow" >&2
    exit 1
  fi
done

grep -Fq "TestAcc_product_definitions_ContractPreflight" Makefile || {
  echo "maintenance must run the guarded Product contract preflight" >&2
  exit 1
}
grep -Fq 'Weekly provider maintenance' .github/workflows/release.yml || {
  echo "release must require exact-commit live maintenance evidence" >&2
  exit 1
}

for action in .github/actions/*/action.yml; do
  if grep -E 'uses: [^.]' "$action" | grep -Ev 'uses: [^@]+@[0-9a-f]{40}([[:space:]]+#.*)?$' >/dev/null; then
    echo "external action is not pinned to a full commit in $action" >&2
    exit 1
  fi
done
