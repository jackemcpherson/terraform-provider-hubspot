# Release operations

Release qualification is fail-closed. Each capability shard has its own GitHub
Environment and `HUBSPOT_ACCESS_TOKEN`; a missing token, entitlement, scope,
acceptance test, or cleanup result fails the run. Custom-property quota telemetry
is advisory because the limits endpoint may omit or lag account-specific state;
the actual create operation remains the authoritative quota check. Capability
manifests contain feature and scope families only. They must not contain Hub IDs,
app IDs, record IDs, configuration IDs, or credentials.

v0.4 has `free_properties`, `form_definitions`, and `files_configuration` shards
in separate protected GitHub Environments, with separate tokens and expected
portal variables. All shards and the Northstar demo mutate one disposable
portal. Northstar has a fourth protected Environment whose token contains the
cumulative property-schema scope union plus `forms` and `files`; no
capability-specific job receives that token.
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
apply, verification, repeat empty plan, supported property, Form, and Files
drift, repair, folder-path refresh, exact-ID adoption, and reviewed terminal
teardown under both engines. It leaves no active Northstar-owned Form or Files
configuration. Each destroy records generated identities only as
domain-separated hashes. Protected runs also require the exact clean demo
commit. Local adoption of a pre-existing demo must supply the generated Form,
folder, and file IDs when no state outputs are available; the script never
discovers or imports configuration by name, path, URL, or search.

HubSpot's property DELETE operations archive definitions and groups into its
recycling bin rather than offering a permanent-purge endpoint. Free acceptance
therefore treats verified archival plus active-name reuse as its terminal cleanup
invariant: no active prefix-owned configuration may remain, each archive path is
verified through the strongest API-supported probe, and the same Git-authored names
must recreate successfully before the demo rebuild is verified. Properties are
read back from the archive; groups are proven absent from the active API and reusable.

The scheduled `Provider maintenance` workflow reports stale `tf_acc_`
configuration in all three shards. The Forms report distinguishes active owned
Forms from retained archived tombstones, the Files report distinguishes active
folders and Managed files from HubSpot-managed Trash retention, and no report
mutates the portal. Manual
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

Before candidate qualification can mutate the portal, run the generic
compatibility preflight against the active cumulative demo checkout:

```sh
./scripts/candidate-preflight.sh vX.Y.Z ../terraform-hubspot-demo
```

The command first discovers the cumulative root and its complete local module
graph, parses every HubSpot provider constraint, and requires both committed
engine locks to select the exact candidate with registry package hashes. An
incompatible module or a missing, malformed, stale, or differently selected
lock blocks qualification and names the exact source. Only then does it build
and validate the complete dual-registry release bundle. The protected Northstar
entry point runs the same compatibility check before acquiring the portal lock
or calling the demo lifecycle. Keep this qualification outside the protected
publication transaction, as required by ADR 0002.

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
./scripts/observe-release.sh v0.4.0 <full-main-commit> jackemcpherson/terraform-provider-hubspot
```

For an existing release, the observer verifies that its tag resolves to `main`,
has a valid signature from the registered identity, and retains a successful
`Required` check. It downloads the exact GitHub asset set, compares every file
with GitHub's immutable SHA-256 asset digest, verifies the checksum signature
against the same identity, and validates the versioned Registry manifest plus
the exact supported archive/checksum closure. This source-release result is
reported as prerequisite evidence and does not count as Registry ingestion.

For each public Registry, the observer then polls the ordinary versions endpoint
up to 12 times, with ten seconds between attempts and a ten-second request
timeout. A valid stale response receives one `Cache-Control: no-cache` and
`Pragma: no-cache` revalidation. A cache-bypassing response cannot complete
observation: a later ordinary response must advertise the version. Non-success,
timeout, malformed, structurally invalid, duplicate, or persistently stale
responses fail with the Registry host and safe response class, never the response
body. The same contract applies to Terraform Registry and OpenTofu Registry.

The minimal architecture does not produce or require a provenance attestation.
A failed observation makes the release unhealthy but does not authorize moving
its tag or replacing its assets. Source and Registry observation remain outside
the protected publication transaction required by ADR 0002.

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
test. Production uses the same GoReleaser configuration directly. The protected
job passes the reviewed `docs/releases/v0.4.0.md` file as custom release notes.
GoReleaser changelog processing must remain enabled because its
`--release-notes` input is ignored when changelog generation is disabled.

The shared registry platform set uses standard `{OS}_{ARCH}` names. In particular,
the 32-bit ARM build is GOARM=6 and is published as `*_arm.zip`; do not suffix the
archive as `armv6` or `armv7`, because OpenTofu Registry target discovery ignores
those nonstandard architecture names.

After publication and successful release observation, download the immutable
GitHub release assets and run the separate completion journey:

```sh
./scripts/released-provider-journey.sh v0.4.0 /path/to/release-assets
```

This completion command first proves each registry-installed package matches the
same immutable GitHub archive and checksum inventory, then runs the released
property lifecycle under both engines. A separate Forms journey creates one
uniquely prefixed Form
definition with Terraform and preserves its exact generated identity while its
state moves to the OpenTofu provider source and back. Both engines read, plan,
update, detect supported drift, repair it, and converge without a name lookup or
second active Form. A separate Files journey creates one two-level hierarchy and
one Managed file, migrates the three generated IDs through the same provider
source sequence, updates metadata, access, and reviewed bytes under both
engines, injects and repairs remote drift, and removes the file before both
folders leaf-first. Exact identity preservation and zero active prefix-owned
configuration are required even on cleanup paths.

The complete cumulative Northstar lifecycle then runs under both engines and
includes the keyed Form definition and Files configuration modules alongside all
released CRM property surfaces. Sanitized evidence under `acceptance-report/`
records the exact
provider and demo commits, selected archive digest for both registry sources,
engines, start/completion timestamps, successful identity-preserving migration,
terminal cleanup, and cleanup result. It contains domain-separated identity
hashes rather than raw remote identifiers. The command is deliberately pinned
to v0.4.0 and fails closed before live mutation: both ordinary Registry
endpoints must confirm the version, real packages from both registries must bind
to the immutable GitHub checksum inventory, and the union-scoped protected
credential plus exact provider and demo checkouts must be available. After
package verification, the journey archives that exact demo commit into a
disposable directory and resolves fresh read-only locks for each correct
engine/source pair before running cumulative Northstar.
