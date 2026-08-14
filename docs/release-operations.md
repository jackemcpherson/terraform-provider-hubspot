# Release Operations

This guide describes the small delivery system used during v0.x development.
ADR 0003 keeps routine validation, publication, and deeper maintenance separate.

## Required Validation

Run the same required check locally and in GitHub Actions:

```sh
make tools
make check
```

The check formats and analyses Go code, runs unit tests, verifies generated
provider documentation, validates workflow syntax, and exercises the current
OpenTofu and Terraform versions. GitHub reports this work as one required job
named `Required`.

Race tests, fuzz seeds, hermetic lifecycle tests, security scans, and live
HubSpot tests remain available outside the required path:

```sh
make test-race
make fuzz-seeds
make test-hermetic
```

## Publication

Run the `Release` workflow from `main` with the intended v-prefixed semantic
version. The workflow requires the selected commit to be the current head of
`main` with a successful `Required` check. It also requires a dated changelog
entry and `docs/releases/<version>.md` release notes.

The single publication job imports the signing key, creates a signed tag, and
runs pinned GoReleaser once. GoReleaser builds the provider archives, signs the
checksum inventory, publishes the versioned Registry manifest, and creates the
GitHub release used by Terraform Registry and OpenTofu Registry.

The `release` environment stores `GPG_PRIVATE_KEY` and `GPG_FINGERPRINT`. During
v0.x it does not require a reviewer. The environment still restricts the signing
credentials to the publication job. Reconsider required approval during v1.0
readiness.

If publication stops after it pushes the signed tag, rerun the same version only
when the tag identifies the same `main` commit and no GitHub release exists. Do
not move a tag or replace a published release. Publish a new patch version when
an existing release contains a bad asset.

Enable GitHub immutable releases. Register the matching GPG public key with both
registries. Each registry ingests the same GitHub release asynchronously. The
workflow does not poll either registry or publish separate asset sets.

## Weekly Maintenance

The `Provider maintenance` workflow runs weekly and supports manual dispatch
from `main`. Its one job runs slower security and release-contract checks, then
executes the cumulative Northstar journey against the disposable HubSpot portal
with both current engines.

The job uses the `northstar` environment. Its `HUBSPOT_ACCESS_TOKEN` must contain
the cumulative CRM schema, Forms, Files, and account-membership scopes. Use
Settings users read/write or HubSpot's documented CRM users read/write
alternative. Set
`HUBSPOT_ACCEPTANCE_PORTAL_ID` to the expected disposable portal. Set the
`HUBSPOT_NORTHSTAR_MEMBERSHIP_EMAIL` environment variable to a dedicated,
disposable member in that portal. Do not use an ordinary account member or a
Super Admin. A normal workflow failure identifies a maintenance problem. The
workflow does not create custom evidence bundles or affect publication
eligibility.

Run the non-live maintenance checks locally with:

```sh
make maintenance-tools
make check-security
make check-release
```

`make maintenance` also runs the live Northstar journey and therefore requires
the protected HubSpot credentials and demo checkout.

## Manual Archival

Use `Archive HubSpot configuration` only for exact `tf_acc_` prefixes left by
live tests. Select one configuration surface and enter its required confirmation:

- `free_properties`: `archive-prefixed-crm-configuration`
- `form_definitions`: `archive-prefixed-form-definitions`
- `files_configuration`: `delete-prefixed-files-configuration`
- `account_memberships`: `delete-prefixed-account-memberships`

The single archival job selects the corresponding GitHub environment from the
validated choice. The cleanup script rejects an unknown surface, an unsafe
prefix, or a confirmation for a different surface. All cleanup operations share
one non-cancelling portal concurrency group.

HubSpot retains archived property and Form configuration. Files enter
HubSpot-managed Trash retention. Account membership cleanup accepts only exact
`tf_acc_...@example.com` ownership, rereads both identities, refuses Super
Admins, and verifies absence. It does not delete global identity. Do not
describe any of these outcomes as permanent global deletion.
