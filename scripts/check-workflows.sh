#!/bin/sh
set -eu

required='archive-hubspot-configuration.yml provider-maintenance.yml release.yml validate-provider.yml'
legacy='acceptance-cleanup.yml acceptance.yml ci.yml provider-lifecycle.yml quality.yml release-candidate.yml run-provider-lifecycle.yml security.yml verify-release.yml'
compatibility=scripts/validate-candidate-compatibility.sh
candidate_preflight=scripts/candidate-preflight.sh

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
grep -q 'HUBSPOT_DEMO_REPO:.*\.demo-source' "$quality"
grep -q "DOCS_PORTAL_REQUIRE_CLEAN: '1'" "$quality"
grep -q 'DOCS_PORTAL_PROVIDER_COMMIT:.*github.sha' "$quality"
grep -q '^      DOCS_PORTAL_DEMO_COMMIT: [0-9a-f]\{40\}$' "$quality"
grep -q 'repository: jackemcpherson/terraform-hubspot-instance-demo' "$quality"
grep -q '^docs-portal:' Makefile
grep -q '^docs-portal-serve: docs-portal' Makefile
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

maintenance=.github/workflows/provider-maintenance.yml
grep -q '^  schedule:' "$maintenance"
grep -q '^  workflow_dispatch:' "$maintenance"
grep -q "if: github.event_name == 'schedule'" "$maintenance"
grep -q 'northstar-candidate-lifecycle.sh' "$maintenance"
grep -q 'northstar-candidate-lifecycle.sh v0.4.0' "$maintenance"
grep -q 'acceptance-shard.sh' "$maintenance"
grep -q 'acceptance-cleanup.sh report free_properties' "$maintenance"
grep -q 'acceptance-cleanup.sh report form_definitions' "$maintenance"
test "$(grep -c '^    environment: free_properties$' "$maintenance")" -eq 2 || {
	echo 'property acceptance and reporting must use the protected free_properties environment' >&2
	exit 1
}
test "$(grep -c '^    environment: form_definitions$' "$maintenance")" -eq 2 || {
	echo 'Forms acceptance and reporting must use the protected form_definitions environment' >&2
	exit 1
}
test "$(grep -c '^    environment: northstar$' "$maintenance")" -eq 1 || {
	echo 'the cumulative lifecycle must use its distinct protected Northstar credential boundary' >&2
	exit 1
}
test "$(grep -c 'group: hubspot-account-free-configuration' "$maintenance")" -eq 5 || {
	echo 'all maintenance jobs must share the account-wide non-cancelling concurrency group' >&2
	exit 1
}
test "$(grep -c 'HUBSPOT_ACCEPTANCE_PORTAL_ID:.*vars.HUBSPOT_ACCEPTANCE_PORTAL_ID' "$maintenance")" -eq 5 || {
	echo 'all live maintenance jobs must enforce the expected portal identity' >&2
	exit 1
}
test "$(grep -c 'HUBSPOT_PORTAL_LOCK_ID: free-configuration' "$maintenance")" -eq 5 || {
	echo 'all maintenance jobs must use the shared local portal lock identity' >&2
	exit 1
}
grep -q 'CAPABILITY_SHARD: free_properties' "$maintenance"
grep -q 'CAPABILITY_SHARD: form_definitions' "$maintenance"
grep -q 'path: acceptance-report/form_definitions\*\.json' "$maintenance"
grep -q 'path: .release-demo/.demo/local-\*-destroy/form-terminal.json' "$maintenance"
grep -q "HUBSPOT_REQUIRE_CLEAN_PROVENANCE: '1'" "$maintenance"
test "$(grep -c 'HUBSPOT_PROVIDER_EXPECTED_COMMIT:.*github.sha' "$maintenance")" -eq 3
grep -q 'HUBSPOT_DEMO_EXPECTED_COMMIT: [0-9a-f]\{40\}' "$maintenance"
grep -q '^          ref: [0-9a-f]\{40\}$' "$maintenance"
maintenance_demo_commit=$(sed -n 's/^      HUBSPOT_DEMO_EXPECTED_COMMIT: \([0-9a-f]\{40\}\)$/\1/p' "$maintenance")
maintenance_demo_ref=$(sed -n 's/^          ref: \([0-9a-f]\{40\}\)$/\1/p' "$maintenance")
quality_demo_commit=$(sed -n 's/^      DOCS_PORTAL_DEMO_COMMIT: \([0-9a-f]\{40\}\)$/\1/p' "$quality")
test "$maintenance_demo_commit" = "$maintenance_demo_ref" && test "$maintenance_demo_ref" = "$quality_demo_commit" || {
	echo 'maintenance, documentation, and hermetic jobs must pin the same exact Northstar candidate' >&2
	exit 1
}
test "$(grep -c "ref: $quality_demo_commit" "$quality")" -eq 2 || {
	echo 'both validation checkouts must use the exact documented Northstar candidate' >&2
	exit 1
}
! grep -Eq 'hubspot-account-free_properties|hubspot-account-form_definitions' "$maintenance" || {
	echo 'maintenance must not use shard-specific account concurrency groups' >&2
	exit 1
}
! grep -Eq 'GPG_|contents: write|goreleaser' "$maintenance" || {
	echo 'provider maintenance must not contain release credentials or publication logic' >&2
	exit 1
}

release=.github/workflows/release.yml
grep -q '^  workflow_dispatch:' "$release"
! grep -q '^  schedule:' "$release" || { echo 'release must be manually dispatched only' >&2; exit 1; }
grep -A4 '^      version:$' "$release" | grep -q '        required: true'
test "$(grep -c '^    runs-on:' "$release")" -eq 1 || { echo 'release must contain exactly one job' >&2; exit 1; }
test "$(grep -c '^    environment: release$' "$release")" -eq 1 || {
	echo 'release must have one protected-environment boundary' >&2
	exit 1
}
test "$(grep -c 'contents: write' "$release")" -eq 1 || { echo 'release needs one contents-write permission' >&2; exit 1; }
grep -q 'checks: read' "$release"
grep -q 'goreleaser/goreleaser-action@' "$release"
grep -q 'version: v2.17.0' "$release"
grep -q 'args: release --clean --release-notes docs/releases/v0.3.0.md' "$release"
grep -q 'git tag -s' "$release"
grep -q 'git push .*refs/tags/' "$release"
test "$(grep -c 'GPG_PRIVATE_KEY:.*secrets.GPG_PRIVATE_KEY' "$release")" -eq 1 || {
	echo 'the private signing key must be exposed only to its import step' >&2
	exit 1
}
! grep -Eqi 'HUBSPOT_|schedule:|needs:|always\(\)|attest|provenance|upload-artifact|download-artifact|observe-release|compare-release|verify-registry-ingestion|released-provider' "$release" || {
	echo 'release contains work outside the minimal build, tag, and publication path' >&2
	exit 1
}
observer_contract='scripts/observe-release.sh scripts/verify-registry-ingestion.sh scripts/verify-release-assets.sh scripts/verify-release-bundle.sh scripts/verify-gpg-signing-identity.sh scripts/verify-registry-checksums.sh scripts/verify-registry-manifest.sh scripts/smoke-release-archive.sh'
# Split the fixed repository-owned observer helper list into grep inputs.
# shellcheck disable=SC2086
! grep -Eqi 'attest|provenance' $observer_contract || {
	echo 'release observation must use signed release evidence without provenance attestations' >&2
	exit 1
}
if grep -q -- '--snapshot' "$release"; then
	echo 'production release must not use snapshot assets' >&2
	exit 1
fi

archive=.github/workflows/archive-hubspot-configuration.yml
grep -q '^  workflow_dispatch:' "$archive"
if grep -q '^  schedule:' "$archive"; then
	echo 'HubSpot configuration archival must be manual only' >&2
	exit 1
fi
grep -q '^      shard:$' "$archive"
grep -q "if: inputs.shard == 'free_properties'" "$archive"
grep -q "if: inputs.shard == 'form_definitions'" "$archive"
test "$(grep -c '^    environment: free_properties$' "$archive")" -eq 1
test "$(grep -c '^    environment: form_definitions$' "$archive")" -eq 1
grep -q 'archive-prefixed-crm-configuration' "$archive"
grep -q 'archive-prefixed-form-definitions' "$archive"
grep -q 'acceptance-cleanup.sh archive free_properties' "$archive"
grep -q 'acceptance-cleanup.sh archive form_definitions' "$archive"
test "$(grep -c 'group: hubspot-account-free-configuration' "$archive")" -eq 2 || {
	echo 'both archive jobs must share the account-wide non-cancelling concurrency group' >&2
	exit 1
}
test "$(grep -c 'HUBSPOT_ACCEPTANCE_PORTAL_ID:.*vars.HUBSPOT_ACCEPTANCE_PORTAL_ID' "$archive")" -eq 2 || {
	echo 'both archive jobs must enforce the protected portal identity' >&2
	exit 1
}
test "$(grep -c 'HUBSPOT_PORTAL_LOCK_ID: free-configuration' "$archive")" -eq 2 || {
	echo 'both archive jobs must use the shared local portal lock identity' >&2
	exit 1
}
! grep -Eq '^[[:space:]]*environment:.*\$\{\{' "$archive" || {
	echo 'operator input must not select a GitHub Environment dynamically' >&2
	exit 1
}
! grep -Eq 'hubspot-account-free_properties|hubspot-account-form_definitions' "$archive" || {
	echo 'manual cleanup must not use shard-specific account concurrency groups' >&2
	exit 1
}

forms_manifest=acceptance/capabilities/form_definitions.json
test "$(cat "$forms_manifest")" = '{"shard":"form_definitions","tier":"free","api_family":"marketing/v3/forms","scope_families":["forms"],"cleanup":"terminal_archive"}' || {
	echo 'Forms capability manifest must contain only the canonical API, scope, tier, and cleanup policy' >&2
	exit 1
}

released_journey=scripts/released-provider-journey.sh
released_forms=scripts/released-form-migration.sh
released_northstar=scripts/released-northstar-journey.sh
grep -q 'released provider journey requires v0.3.0' "$released_journey"
grep -q 'released Form migration requires v0.3.0' "$released_forms"
grep -q 'Northstar release journey requires v0.3.0' "$released_northstar"
grep -q 'registry.terraform.io/jackemcpherson/hubspot' "$released_forms"
grep -q 'registry.opentofu.org/jackemcpherson/hubspot' "$released_forms"
test "$(grep -c 'state replace-provider' "$released_forms")" -eq 2 || {
	echo 'released Forms must migrate one state to OpenTofu and back' >&2
	exit 1
}
test "$(grep -c 'run_helper drift' "$released_forms")" -eq 1 || {
	echo 'released Forms must use one reusable exact-ID drift phase' >&2
	exit 1
}
grep -q 'run_helper verify-terminal' "$released_forms"
grep -q 'identity_preserved":true' "$released_forms"
grep -q 'form_identity_preserved":true' "$released_journey"
grep -q 'HUBSPOT_ONE_PORTAL_LOCK_DIR' "$released_journey"
grep -q 'hubspot_form_definition' scripts/verify-released-provider.sh
grep -q 'released-form-migration_test.sh' Makefile
! grep -Eqi 'hub[_-]?id|app[_-]?id|record[_-]?id|form[_-]?id|portal[_-]?id|access[_-]?token|pat-' "$forms_manifest" || {
	echo 'Forms capability manifest contains a forbidden identifier or credential marker' >&2
	exit 1
}

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
test -x "$compatibility"
test -x "$candidate_preflight"
grep -q 'validate-candidate-compatibility.sh.*"$version".*"$demo"' "$candidate_preflight"
grep -q 'registry-release-preflight.sh' "$candidate_preflight"
grep -q 'validate-candidate-compatibility.sh.*"$version".*"$demo_root"' scripts/northstar-candidate-lifecycle.sh
# Match the literal Make variable expression.
# shellcheck disable=SC2016
grep -q '^[[:space:]]*@"$(TOOLS_BIN)/goreleaser" release --snapshot --clean --skip=sign$' Makefile
