# Release operations

Release qualification is fail-closed. Each capability shard has its own GitHub
Environment and `HUBSPOT_ACCESS_TOKEN`; a missing token, entitlement, scope,
quota, acceptance test, or cleanup result fails the run. Capability manifests
contain feature and scope families only. They must not contain Hub IDs, app IDs,
record IDs, configuration IDs, or credentials.

v0.1 has one `free_properties` shard and one disposable portal shared with the
Northstar demo. Run `make one-portal-free-lifecycle` only with the Free shard's
protected token and a valid acceptance prefix. It saves no CRM records: it applies
the demo's reviewed destroy plan after adopting and verifying its known identities,
runs the owned Free acceptance suite, then always rebuilds the Git-authored demo
through a fresh reviewed plan, including when acceptance fails. The demo and the
shard share a portal lock keyed by
`HUBSPOT_PORTAL_LOCK_ID` (default `default`) across local checkouts; GitHub uses
the non-cancelling `hubspot-account-free_properties` concurrency group across
runners. Do not bypass either gate for this portal.

HubSpot's property DELETE operations archive definitions and groups into its
recycling bin rather than offering a permanent-purge endpoint. Free acceptance
therefore treats verified archival plus active-name reuse as its terminal cleanup
invariant: no active prefix-owned configuration may remain, each archive path is
verified through the strongest API-supported probe, and the same Git-authored names
must recreate successfully before the demo rebuild is verified. Properties are
read back from the archive; groups are proven absent from the active API and reusable.

The scheduled janitor reports stale `tf_acc_` configuration. It never deletes.
Manual cleanup requires a selected shard, an exact owned prefix ending in `_`,
the protected shard environment, and the confirmation text shown by the workflow.

A candidate names one full commit SHA. Its directly environment-bound acceptance
run, full engine matrix, security gate, and deterministic gate produce a
commit-bound report.
Publication accepts a v-prefixed SemVer and only proceeds when it downloads a
successful report for that exact SHA. The unsigned build and independent rebuild
use an unpushed local version tag and run without secrets. Only the signing job
receives the GPG key, and it fetches commit metadata without checking out source.

The release workflow smoke-installs the first artifact set through filesystem
mirrors under both full registry addresses, signs the checksum and tag, verifies
the draft assets and attestations, then publishes. Enable GitHub immutable
releases, require one approval for the `release` environment before signing, and register the same
GPG public key with Terraform Registry before v0.1.0. OpenTofu's bootstrap requires
the first signed release and accepted provider entry before its signing-key issue
can be submitted; register that same key immediately after provider acceptance.
This ordering does not permit an unsigned release. Store `GPG_PRIVATE_KEY` and
`GPG_FINGERPRINT` only in the release environment; expose the armored public key
as the non-secret `GPG_PUBLIC_KEY` repository variable.

After publication, run `Verify release`. It polls both registries, installs the
actual archives through Terraform and OpenTofu, runs protected released-artifact
capability fixtures, and migrates live state in both directions. A release is not
announced until its uploaded release report is entirely green.

Registry metadata can be resynchronized after an ingestion failure. A bad archive,
checksum, signature, manifest, SBOM, or provenance record requires a new patch
release; maintainers must not move the tag or replace an asset.

The signed checksum inventory must contain exactly the provider archives and one
Registry manifest. Keep `terraform-registry-manifest.json` as the repository source,
but publish and checksum it as
`terraform-provider-hubspot_<VERSION>_manifest.json`, matching the Terraform
Registry release contract. Standalone SPDX SBOM files remain published release assets but
must not appear in the Registry checksum file because Registry ingestion does not
include them in its package request.
