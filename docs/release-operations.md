# Release Operations

This guide explains provider qualification, publication, and recovery. Release
qualification fails closed at each trust boundary.

---

## Acceptance Environment

Each capability shard has a protected GitHub environment and access token. A
missing token, scope, entitlement, quota, test, or cleanup result stops the run.

The v0.1 release has one `free_properties` shard. It shares a disposable portal
with the Northstar demo. Run `make one-portal-free-lifecycle` only with that
shard's protected token and a valid acceptance prefix.

The target completes this sequence:

1. Adopt and verify the known demo identities.
2. Apply the reviewed demo destroy plan.
3. Run the owned Free acceptance suite.
4. Rebuild the Git-authored demo through a fresh reviewed plan.

The target also rebuilds the demo after an acceptance failure. Local checkouts
share `HUBSPOT_PORTAL_LOCK_ID`. GitHub uses the non-cancelling
`hubspot-account-free_properties` concurrency group. Do not bypass these locks.

## Cleanup

HubSpot archives property definitions and groups instead of deleting them. The
acceptance suite verifies archival and active-name reuse. No active configuration
with the owned prefix can remain.

The scheduled lifecycle reports stale `tf_acc_` configuration. It does not
archive configuration. Use `Archive CRM configuration` for manual archival.
Provide an owned prefix that ends in `_` and the exact required confirmation.

## Publish a Release

Run `Provider lifecycle` from `main`. Supply the intended version with a `v`
prefix. The workflow binds the release to the dispatch commit and checks that
commit through the `Required` status.

A new release completes these actions:

1. Run protected source acceptance.
2. Build the release artefact set twice without secrets.
3. Compare the two builds.
4. Wait for approval on the protected `release` environment.
5. Sign, attest, and publish the first verified build.

The same run waits for both registries after publication. It verifies registry
downloads against the immutable GitHub assets. It then checks both provider
lifecycles and bidirectional state migration.

If registry ingestion is incomplete, rerun the same version. A verified draft
resumes publication. A published release resumes registry and live checks.

## Configure Signing

Require one approval for the `release` environment. Register the same GPG public
key with Terraform Registry and OpenTofu Registry.

OpenTofu requires the first signed release before provider acceptance. Register
the key immediately after that acceptance, then rerun the same version.

Store `GPG_PRIVATE_KEY` and `GPG_FINGERPRINT` only in the release environment.
Expose the public key through the non-secret `GPG_PUBLIC_KEY` variable.

Do not move a published tag or replace an asset. Publish a patch release to
correct an archive, checksum, signature, manifest, SBOM, or provenance record.

## Validate the Release Bundle

Run the local pre-flight before you dispatch a release:

```shell
make release-preflight VERSION=vX.Y.Z
```

The target checks the GoReleaser configuration and tool versions. It builds the
complete release without publication. It then checks these properties:

- The Registry manifest conforms to its schema.
- Archive, manifest, and checksum membership is exact.
- Each archive contains the expected binary name.
- The bundle contains valid SPDX documents.
- OpenTofu and Terraform can install the provider from filesystem mirrors.

The public registries do not provide a pre-publication test. This local and CI
gate therefore checks the publication contract. Local and CI builds use
`scripts/build-release-bundle.sh`.

## Follow Registry File Rules

The signed checksum inventory contains the provider archives and one manifest.
Publish `terraform-registry-manifest.json` with this release name:

```text
terraform-provider-hubspot_<VERSION>_manifest.json
```

Publish standalone SPDX SBOM files as release assets. Do not include those files
in the Registry checksum inventory.

Use standard `{OS}_{ARCH}` platform names. Publish the 32-bit ARM build as
`*_arm.zip` with `GOARM=6`. OpenTofu discovery ignores `armv6` and `armv7`
archive suffixes.
