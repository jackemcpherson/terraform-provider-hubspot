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

The scheduled lifecycle reports stale `tf_acc_` configuration. It never archives
anything. Manual archival uses `Archive CRM configuration` and requires an exact
owned prefix ending in `_`, the protected `free_properties` environment, and the
literal confirmation `archive-prefixed-crm-configuration`. HubSpot retains this
configuration in its recycling bin, so do not describe the operation as deletion.

To release, run `Provider lifecycle` from `main` with one input: the intended
v-prefixed SemVer. The workflow binds the release to the dispatch commit, requires
that commit to be the current head of `main` with a successful `Required` quality
check, and observes whether the version is new, a verified draft, or already
published. A new release runs protected source acceptance, constructs the same
real-version asset set twice without secrets, and compares it. The signing
job then waits for one approval on the `release` environment before it receives
the GPG key. Attestation and publication promote the first build; they do not
rebuild it.

After publication, the same run polls both registries, verifies actual Terraform
and OpenTofu downloads against the immutable GitHub assets, and performs both
released-provider lifecycles plus bidirectional state migration inside one portal
teardown/restoration window. If registry ingestion is not ready, rerun `Provider
lifecycle` with the same version. A verified draft resumes publication; a verified
published release skips creation and resumes registry and live verification.

Enable GitHub immutable releases, require one approval for the `release`
environment before signing, and register the same GPG public key with Terraform
Registry. OpenTofu's bootstrap requires the first signed release and accepted
provider entry before its signing-key issue can be submitted; register that same
key immediately after provider acceptance, then rerun the same version. This
ordering does not permit an unsigned release. Store `GPG_PRIVATE_KEY` and
`GPG_FINGERPRINT` only in the release environment; expose the armored public key
as the non-secret `GPG_PUBLIC_KEY` repository variable.

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

Run `make release-preflight` before dispatching a release, or pass the intended
version with `make release-preflight VERSION=vX.Y.Z`. The target runs GoReleaser's
configuration and tool health checks, builds the full release without publishing,
validates the Registry manifest schema, exact archive/manifest/checksum closure,
archive binary names, and SPDX documents, then installs the built archive through
filesystem mirrors with both OpenTofu and Terraform. The public registries expose
no pre-publication dry-run API, so this local/CI gate is the publication-contract
test. `scripts/build-release-bundle.sh` is shared by that local target and both CI
builds, preventing the pre-flight and production artifact shapes from drifting.
The protected release job still verifies the bundle before the private signing
key is exposed and verifies the real GPG signature before the draft is published.

The shared registry platform set uses standard `{OS}_{ARCH}` names. In particular,
the 32-bit ARM build is GOARM=6 and is published as `*_arm.zip`; do not suffix the
archive as `armv6` or `armv7`, because OpenTofu Registry target discovery ignores
those nonstandard architecture names.
