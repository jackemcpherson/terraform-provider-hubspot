#!/bin/sh
set -eu

required='archive-crm-configuration.yml run-provider-lifecycle.yml validate-provider.yml'
legacy='acceptance-cleanup.yml acceptance.yml ci.yml provider-lifecycle.yml quality.yml release-candidate.yml release.yml security.yml verify-release.yml'

actual=$(find .github/workflows -maxdepth 1 -type f -name '*.yml' -exec basename {} \; | LC_ALL=C sort | tr '\n' ' ' | sed 's/ $//')
# Split the fixed repository-owned filename list into one name per line.
# shellcheck disable=SC2086
expected=$(printf '%s\n' $required | LC_ALL=C sort | tr '\n' ' ' | sed 's/ $//')
test "$actual" = "$expected" || {
	echo "workflow surface must contain exactly: $expected" >&2
	exit 1
}
for name in $legacy; do
	test ! -e ".github/workflows/$name" || {
		echo "legacy workflow must be removed: $name" >&2
		exit 1
	}
done

for workflow in .github/workflows/*.yml; do
	grep -q '^permissions: {}' "$workflow" || { echo "workflow $workflow must start with empty permissions" >&2; exit 1; }
	grep -q 'timeout-minutes:' "$workflow" || { echo "workflow $workflow has no finite timeout" >&2; exit 1; }
	grep -q 'runs-on: ubuntu-24.04' "$workflow" || { echo "workflow $workflow must pin the hosted runner image" >&2; exit 1; }
	! grep -q 'ubuntu-latest' "$workflow" || { echo "workflow $workflow must not use ubuntu-latest" >&2; exit 1; }
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

for action in .github/actions/*/action.yml; do
	if grep -E 'uses: [^.]' "$action" | grep -Ev 'uses: [^@]+@[0-9a-f]{40}([[:space:]]+#.*)?$' >/dev/null; then
		echo "external action is not pinned to a full commit in $action" >&2
		exit 1
	fi
	! grep -q 'ubuntu-latest' "$action" || { echo "action $action must not name an unpinned runner" >&2; exit 1; }
done

quality=.github/workflows/validate-provider.yml
grep -q '^  pull_request:' "$quality"
grep -q '^  push:' "$quality"
grep -q '^  schedule:' "$quality"
grep -q '^    name: Required$' "$quality"
grep -q 'make release-preflight' "$quality"
grep -q 'ossf/scorecard-action@' "$quality"
grep -q '^check:.*check-security' Makefile
grep -q 'govulncheck@v1.1.4' Makefile
grep -q 'actionlint@v1.7.12' Makefile
grep -q '^ZIZMOR_VERSION := 1.27.0$' Makefile
grep -q 'install-zizmor.sh' Makefile
# Match the literal Make variable expression.
# shellcheck disable=SC2016
grep -q '^[[:space:]]*@"$(TOOLS_BIN)/zizmor" \.$' Makefile
for version in 1.8.8 1.10.10 1.11.11 1.12.3 1.8.5 1.15.8; do
	grep -q "version: $version" "$quality" || { echo "quality engine matrix is missing $version" >&2; exit 1; }
done

lifecycle=.github/workflows/run-provider-lifecycle.yml
grep -q '^  schedule:' "$lifecycle"
grep -q '^  workflow_dispatch:' "$lifecycle"
test "$(grep -c '^      [a-z-]*:$' "$lifecycle" || true)" -ge 1
grep -q 'observe-release.sh' "$lifecycle"
grep -q 'build-release-bundle.sh' "$lifecycle"
test "$(grep -c 'build-release-bundle.sh' "$lifecycle")" -eq 2 || { echo 'release must use one builder for both builds' >&2; exit 1; }
grep -q 'compare-release-builds.sh' "$lifecycle"
grep -q 'verify-registry-ingestion.sh' "$lifecycle"
grep -q 'verify-released-provider.sh' "$lifecycle"
grep -q 'one-portal-free-lifecycle.sh ./scripts/released-provider-journey.sh' "$lifecycle"
grep -q "needs.observe.outputs.state == 'published'" "$lifecycle"
test "$(grep -c '^    environment: release$' "$lifecycle")" -eq 1 || {
	echo 'release must require one protected-environment approval before signing' >&2
	exit 1
}
test "$(grep -c 'GPG_PRIVATE_KEY:.*secrets.GPG_PRIVATE_KEY' "$lifecycle")" -eq 1 || {
	echo 'the private signing key must be exposed only to the signing step' >&2
	exit 1
}
test "$(grep -c 'id-token: write' "$lifecycle")" -eq 1 || { echo 'OIDC write permission must be isolated to attestation' >&2; exit 1; }
test "$(grep -c 'contents: write' "$lifecycle")" -eq 2 || { echo 'contents write must be isolated to new and resumed publication' >&2; exit 1; }
grep -A4 '^  attest:$' "$lifecycle" | grep -q 'needs: \[observe, rebuild, sign\]' || {
	echo 'attestation must remain behind protected release approval' >&2
	exit 1
}
test "$(grep -c 'observe-release.sh' "$lifecycle")" -eq 2 || {
	echo 'a resumed draft must be reverified immediately before publication' >&2
	exit 1
}
if grep -q -- '--snapshot' "$lifecycle"; then
	echo 'production release must not use snapshot assets' >&2
	exit 1
fi
! grep -Eq 'candidate-report|verify-candidate-report|release-candidate' "$lifecycle" || {
	echo 'release lifecycle must not depend on a candidate-report handoff' >&2
	exit 1
}

archive=.github/workflows/archive-crm-configuration.yml
grep -q '^  workflow_dispatch:' "$archive"
if grep -q '^  schedule:' "$archive"; then
	echo 'CRM configuration archival must be manual only' >&2
	exit 1
fi
grep -q '^    environment: free_properties$' "$archive"
grep -q 'archive-prefixed-crm-configuration' "$archive"
grep -q 'acceptance-cleanup.sh archive free_properties' "$archive"
! grep -q '^      shard:' "$archive" || { echo 'the only supported shard must not be operator-selectable' >&2; exit 1; }

grep -Fq "mtime: '{{ .CommitDate }}'" .goreleaser.yml || {
	echo 'release archive files must use the commit timestamp' >&2
	exit 1
}
test "$(grep -Fc "name_template: '{{ .ProjectName }}_{{ .Version }}_manifest.json'" .goreleaser.yml)" = 2 || {
	echo 'Registry manifest must use its versioned release asset name in checksums and publication' >&2
	exit 1
}
grep -q 'GORELEASER_CURRENT_TAG=' scripts/build-release-bundle.sh
grep -q -- '--skip=announce,publish,sign,validate' scripts/build-release-bundle.sh
grep -q 'goreleaser" check' Makefile
grep -q 'goreleaser" healthcheck' Makefile
grep -q 'build-release-bundle.sh' scripts/registry-release-preflight.sh
# Match the literal Make variable expression.
# shellcheck disable=SC2016
grep -q '^[[:space:]]*@"$(TOOLS_BIN)/goreleaser" release --snapshot --clean --skip=sign$' Makefile
