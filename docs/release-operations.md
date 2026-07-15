# Release operations

Release qualification is fail-closed. Each capability shard has its own GitHub
Environment and `HUBSPOT_ACCESS_TOKEN`; a missing token, entitlement, scope,
quota, acceptance test, or cleanup result fails the run. Capability manifests
contain feature and scope families only. They must not contain Hub IDs, app IDs,
record IDs, configuration IDs, or credentials.

The scheduled janitor reports stale `tf_acc_` configuration. It never deletes.
Manual cleanup requires a selected shard, an exact owned prefix ending in `_`,
the protected shard environment, and the confirmation text shown by the workflow.

A candidate names one full commit SHA. Its reusable acceptance run, full engine
matrix, security gate, and deterministic gate produce a commit-bound report.
Publication accepts a v-prefixed SemVer and only proceeds when it downloads a
successful report for that exact SHA. The unsigned build and independent rebuild
use an unpushed local version tag and run without secrets. Only the signing job
receives the GPG key, and it fetches commit metadata without checking out source.

The release workflow smoke-installs the first artifact set through filesystem
mirrors under both full registry addresses, signs the checksum and tag, verifies
the draft assets and attestations, then publishes. Enable GitHub immutable
releases, require approval for the `release` environment, and register the same
GPG public key with Terraform Registry and OpenTofu Registry before v0.1.0. Store
`GPG_PRIVATE_KEY` and `GPG_FINGERPRINT` only in that environment; expose the
armored public key as the non-secret `GPG_PUBLIC_KEY` repository variable.

After publication, run `Verify release`. It polls both registries, installs the
actual archives through Terraform and OpenTofu, runs protected released-artifact
capability fixtures, and migrates live state in both directions. A release is not
announced until its uploaded release report is entirely green.

Registry metadata can be resynchronized after an ingestion failure. A bad archive,
checksum, signature, manifest, SBOM, or provenance record requires a new patch
release; maintainers must not move the tag or replace an asset.
