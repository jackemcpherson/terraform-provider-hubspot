# Release operations

Release qualification is fail-closed. Each capability shard has its own GitHub
Environment and `HUBSPOT_ACCESS_TOKEN`; a missing token, entitlement, scope,
acceptance test, or cleanup result fails the run. Custom-property quota telemetry
is advisory because the limits endpoint may omit or lag account-specific state;
the actual create operation remains the authoritative quota check. Capability
manifests contain feature and scope families only. They must not contain Hub IDs,
app IDs, record IDs, configuration IDs, or credentials.

v0.3 has `free_properties` and `form_definitions` shards in separate protected
GitHub Environments, with separate tokens and expected portal variables. Both
shards and the Northstar demo mutate one disposable portal. Northstar has a
third protected Environment whose token contains the cumulative property-schema
scope union plus `forms`; neither capability-specific job receives that token.
Run
`make one-portal-free-lifecycle` only with the Free shard's
protected token and a valid acceptance prefix. It saves no CRM records: it applies
the demo's reviewed destroy plan after adopting and verifying its known identities,
runs the owned Free acceptance suite, then always rebuilds the Git-authored demo
through a fresh reviewed plan, including when acceptance fails. The demo and the
shard share a portal lock keyed by
`HUBSPOT_PORTAL_LOCK_ID` (default `free-configuration`) across local checkouts;
GitHub uses the non-cancelling `hubspot-account-free-configuration` concurrency
group for property acceptance, Forms acceptance, Northstar, reports, and manual
cleanup. Do not bypass either gate for this portal.

Scheduled property acceptance and the cumulative Northstar lifecycle are
separate jobs because one GitHub job cannot safely enter two protected credential
boundaries. The Northstar job starts from an empty portal, runs plan, reviewed
apply, verification, repeat empty plan, supported property and Form drift,
repair, refresh, exact-ID adoption, and reviewed terminal teardown under both
engines, then leaves no active Northstar-owned Form. Each destroy records the
Archived generated identity only as a domain-separated hash. Protected runs
also require the exact clean demo commit. Local adoption of a pre-existing demo
must supply `HUBSPOT_NORTHSTAR_FORM_ID` when no state output is available; the
script never discovers or imports a Form by remote name.

HubSpot's property DELETE operations archive definitions and groups into its
recycling bin rather than offering a permanent-purge endpoint. Free acceptance
therefore treats verified archival plus active-name reuse as its terminal cleanup
invariant: no active prefix-owned configuration may remain, each archive path is
verified through the strongest API-supported probe, and the same Git-authored names
must recreate successfully before the demo rebuild is verified. Properties are
read back from the archive; groups are proven absent from the active API and reusable.

The scheduled `Provider maintenance` workflow reports stale `tf_acc_`
configuration in both shards. The Forms report distinguishes active owned Forms
from retained archived tombstones, and neither report mutates the portal. Manual
archival uses `Archive HubSpot configuration`: select a shard, provide an exact
owned prefix ending in `_`, and enter that shard's literal confirmation.
Property cleanup requires `archive-prefixed-crm-configuration`; Forms cleanup
requires `archive-prefixed-form-definitions`. Static jobs bind each choice to its
protected Environment, credentials, expected portal, and the shared account lock.
Forms are selected by the exact prefix but archived and verified by immutable ID;
the cleanup never archives by a name match alone. HubSpot retains archived
configuration, so do not describe either operation as deletion.

Scheduled Forms source acceptance runs only in the protected `form_definitions`
Environment with its `forms`-scoped token and expected portal identity. OpenTofu
and Terraform each receive a distinct suffix beneath the run's unique owned
prefix and exercise one active form at a time. The live suite covers canonical
create and no-op state, supported presentation update and drift repair, exact-ID
import, external archive and recreation, and final terminal archive. Duplicate
names, archived-name reuse, unsupported structures, and destructive failure
matrices remain hermetic; do not add them to every live candidate run.

A successful Forms run uploads `form_definitions-tofu.json` and
`form_definitions-terraform.json` beside the shard summary. Each engine record
binds the exact candidate commit to the Forms API and scope family, a
domain-separated portal fingerprint, generated and terminal identity hashes, a
UTC timestamp, and `cleanup: passed`. It contains no raw portal ID, form ID,
owned prefix, name, payload, or credential. Missing engine evidence or incomplete
terminal cleanup fails the shard and has no release waiver.

To release, run `Release` from `main` with the intended v-prefixed SemVer. The
single protected job requires the dispatch commit to be the current head of
`main` with a successful `Required` quality check. It imports the release signing
key, creates a signed tag, and runs pinned GoReleaser once. GoReleaser builds all
supported platform archives, signs the checksum inventory, publishes the
versioned Registry manifest, and creates the GitHub release.

Terraform Registry and OpenTofu Registry ingest that same GitHub release through
their configured integrations; the workflow does not upload to either registry
separately or wait for asynchronous ingestion. If a run pushed the signed tag but
failed before creating the GitHub release, rerun the same version only after
confirming the tag points to the same `main` commit. Once the GitHub release
exists, never rerun or mutate that version; publish a patch release instead.

Enable GitHub immutable releases, require one approval for the `release`
environment before signing, and register the same GPG public key with Terraform
Registry. OpenTofu's bootstrap requires the first signed release and accepted
provider entry before its signing-key issue can be submitted; register that same
key immediately after provider acceptance, then resynchronize the Registry if
needed. This ordering does not permit an unsigned release. Store
`GPG_PRIVATE_KEY` and `GPG_FINGERPRINT` only in the release environment.

Registry metadata can be resynchronized after an ingestion failure. A bad
archive, checksum, signature, or manifest requires a new patch release;
maintainers must not move the tag or replace an asset.

Post-publication observation is read-only and separate from the publication
transaction. Export `GH_TOKEN`, the repository `GPG_PUBLIC_KEY`, and the full
`GPG_FINGERPRINT` registered with both Registries, then run:

```sh
./scripts/observe-release.sh v0.3.0 <full-main-commit> jackemcpherson/terraform-provider-hubspot
```

For an existing release, the observer verifies that its tag resolves to `main`,
has a valid signature from the registered identity, and retains a successful
`Required` check. It downloads the exact GitHub asset set, compares every file
with GitHub's immutable SHA-256 asset digest, verifies the checksum signature
against the same identity, and validates the versioned Registry manifest plus
the exact supported archive/checksum closure. The minimal architecture does not
produce or require a provenance attestation. A failed observation makes the
release unhealthy but does not authorize moving its tag or replacing its assets.

The signed checksum inventory must contain exactly the provider archives and one
Registry manifest. Keep `terraform-registry-manifest.json` as the repository source,
but publish and checksum it as
`terraform-provider-hubspot_<VERSION>_manifest.json`, matching the Terraform
Registry release contract.

Run `make release-preflight` before dispatching a release, or pass the intended
version with `make release-preflight VERSION=vX.Y.Z`. The target runs GoReleaser's
configuration and tool health checks, builds the full release without publishing,
validates the Registry manifest schema, exact archive/manifest/checksum closure,
and archive binary names, then installs the built archive through filesystem
mirrors with both OpenTofu and Terraform. The public registries expose no
pre-publication dry-run API, so this local/CI gate is the publication-contract
test. Production uses the same GoReleaser configuration directly.

The shared registry platform set uses standard `{OS}_{ARCH}` names. In particular,
the 32-bit ARM build is GOARM=6 and is published as `*_arm.zip`; do not suffix the
archive as `armv6` or `armv7`, because OpenTofu Registry target discovery ignores
those nonstandard architecture names.

After publication and successful release observation, download the immutable
GitHub release assets and run the separate completion journey:

```sh
./scripts/released-provider-journey.sh v0.2.0 /path/to/release-assets
```

This completion command first proves each registry-installed package matches the
GitHub archive, then runs both live shards, bidirectional provider-source state
migration, and the complete dual-engine Northstar lifecycle. It writes sanitized
evidence under `acceptance-report/` containing the exact provider commit, pinned
demo commit, archive digest, engines, registry sources, timestamp, and cleanup
result. The pinned demo commit contains separate read-only OpenTofu and Terraform
locks covering all 13 released platforms.
