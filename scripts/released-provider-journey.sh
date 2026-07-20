#!/bin/sh
set -eu

version=${1:?release version is required}
released_live_shard=${RELEASED_LIVE_SHARD_SCRIPT:-./scripts/released-live-shard.sh}
state_migration=${STATE_MIGRATION_SCRIPT:-./scripts/verify-state-migration.sh}

"$released_live_shard" \
	free_properties \
	terraform \
	registry.terraform.io/jackemcpherson/hubspot \
	"$version"
"$released_live_shard" \
	free_properties \
	tofu \
	registry.opentofu.org/jackemcpherson/hubspot \
	"$version"
"$state_migration" "$version"
