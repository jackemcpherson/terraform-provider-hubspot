# Changelog

All notable changes to this project are documented here.

## [Unreleased]

## [0.5.0] - 2026-08-15

### Added

- Manage HubSpot account membership through the Settings users API with
  canonical-ID and explicit-email import, observed optional names, an explicit
  welcome-email choice, and no CRM user-profile surface.
- Publish the stable-keyed `account-membership` consumer module, generated
  references, cumulative Northstar lifecycle, and guarded janitor support.

### Changed

- Require an explicit local deletion opt-in, exact ID/email reread, non-Super
  Admin status, and verified absence before account membership leaves state.
- Fail closed on name updates with current role or team assignments and surface
  pre-activation `USER_NOT_ON_ANY_HUBS` failures without indefinite retry.

### Fixed

- Keep Settings user IDs distinct from CRM user and owner IDs, and never adopt
  an existing membership after a duplicate create response.

## [0.4.0] - 2026-08-14

### Added

- Manage explicit HubSpot File folder hierarchies and Managed files by generated
  ID through the GA Files API, requiring only the minimum `files` scope.
- Publish the stable-keyed `files-configuration` consumer module, generated
  references, composable hierarchy example, and Files lifecycle guidance.

### Changed

- Reduce v0.x delivery to one required check, one manual release job, one weekly
  maintenance job, and one manual archival job.
- Run cumulative CRM schema, Forms, and Files maintenance with the current
  OpenTofu and Terraform versions outside the merge and publication paths.

### Fixed

- Accept standalone clones and linked worktrees as protected provider and demo
  provenance while still requiring the exact clean 40-character commit and
  checkout root.
- Stabilise Files updates around HubSpot hierarchy snapshots, asynchronous moves,
  exact-ID adoption, and refreshed paths after folder changes.
- Build and test with Go 1.26.6 to resolve six reachable standard-library
  vulnerabilities in the v0.4.0 toolchain.
- Reject missing or mismatched release notes before the publication job creates
  the signed release tag.

## [0.3.0] - 2026-08-03

### Added

- Manage one narrowly typed active contact email Form definition through
  `hubspot_form_definition`, with generated-ID import, supported presentation
  reconciliation, and verified terminal archival.
- Publish the stable-keyed `form-definition` consumer module, generated resource
  reference, executable examples, and cumulative Northstar configuration.
- Prove the Forms lifecycle under OpenTofu and Terraform through the behavioral
  fake, protected capability shard, cumulative candidate journey, and one
  identity-preserving post-publication state migration between registry sources.

### Changed

- Generalize portal locking, protected credentials, maintenance reporting, and
  manual cleanup across CRM property schema, Forms, and the union-scoped
  cumulative Northstar lifecycle.
- Keep the v0.3.0 contract deliberately narrow: one contact email field,
  no-consent presentation, no submissions, notifications, automation, arbitrary
  fields, or restore/purge promise.

### Fixed

- Align post-publication release observation with the minimal release
  architecture by verifying the registered signing identity and exact GitHub
  asset digests without requiring provenance attestations.
- Supply the current Forms v3 API's required create-only timestamps and archive
  marker without promoting service metadata into Terraform state or PATCH, and
  normalize its empty blocked-domain response sentinel.

## [0.2.0] - 2026-08-02

### Added

- Define v0.2.0 CRM property schema as one supported contacts, companies, deals,
  and tickets configuration surface with a narrow `text`/`select` Northstar
  consumer module contract.
- Exercise keyed groups, text properties, select properties, options, drift,
  import, archival, name reuse, and teardown through both real CLIs against the
  behavioral fake.
- Commit per-engine v0.2.0 Northstar locks and run the complete local and
  registry-installed plan, apply, drift repair, refresh, adoption, and teardown
  journey under OpenTofu and Terraform.
- Bind post-publication evidence to exact provider/demo commits and release
  archive digests after both registries pass package verification.

### Changed

- Treat custom-property limit telemetry as advisory; remote create responses are
  authoritative and no aggregate ten-property value blocks qualification.
- Match current `/crm/properties/2026-03` behavior by permitting immediate reuse
  of archived property names while retaining archived discovery.
- Make documentation portal checks reject stale generated source, parse every
  rendered HTML page, and smoke-test localhost serving.
- Prove an authentic released-v0.1.6 property state produces empty v0.2.0 plans
  under both supported engines.

## [0.1.6] - 2026-07-30

### Fixed

- Publish the canonical MPL-2.0 license text so OpenTofu Registry can
  redistribute the provider documentation.

## [0.1.5] - 2026-07-25

### Added

- Run provider lifecycle acceptance on every change against an in-process
  HubSpot fake that models archival, archived-name reservation, and server-side
  write normalization, with no portal credentials, under both Terraform and
  OpenTofu.
- Document destroy semantics per resource: archival versus deletion, name
  reservation after archival, and the non-destructive state-removal
  alternative.

### Changed

- Reduce releases to one manually dispatched job that creates the signed tag and
  uses GoReleaser once to build and publish the complete Terraform/OpenTofu asset
  set; move scheduled portal checks into a separate maintenance workflow.
- Refuse mutating real-portal acceptance runs, including cleanup, unless the
  token's portal identity resolved from the account-info API matches the
  expected portal identifier.
- Surface HubSpot daily-quota rate limits as immediate terminal errors instead
  of retrying requests that cannot succeed before the daily quota resets.

### Fixed

- Describe every provider, resource, and data-source schema attribute so
  Registry documentation and editor hovers carry the full contract.
- Upgrade `golang.org/x/text` to v0.39.0 to resolve GO-2026-5970 (infinite loop
  on invalid input) reached through schema description formatting.

## [0.1.4] - 2026-07-20

### Changed

- Rotate the Registry release-signing identity after the original namespace key
  metadata prevented Terraform from decoding the public key for v0.1.3.
- Consolidate seven GitHub Actions workflows into quality, provider lifecycle,
  and manual CRM-configuration archival surfaces with pinned runner images and
  least-privilege release jobs.
- Use one local release-bundle builder for developer pre-flight and reproducible
  CI builds, then verify the published provider through both Terraform and
  OpenTofu in one serialized portal journey.
- Resume verified draft or published versions by rerunning the same provider
  lifecycle input instead of passing candidate reports between workflows.

### Fixed

- Install the pinned Terraform and OpenTofu engines before the protected signing
  job reverifies the qualified release bundle.

## [0.1.3] - 2026-07-20

### Changed

- Publish 32-bit ARM provider builds under the standard `arm` architecture name
  so the same release target is discoverable by OpenTofu and Terraform.
- Run the full Registry artifact pre-flight locally, in ordinary CI, and before
  release artifacts enter the protected signing path.

### Fixed

- Declare Registry manifest format `version` 1 instead of the unsupported
  `format_version` field that Terraform Registry interpreted as version 0.
- Validate the manifest schema, GoReleaser artifact catalog, checksum closure,
  archive contents, SPDX SBOMs, and dual-engine installation before publication.

## [0.1.2] - 2026-07-19

### Fixed

- Publish the Registry manifest under the required versioned provider asset name
  and checksum that exact filename so Terraform Registry can ingest releases.

## [0.1.1] - 2026-07-19

### Fixed

- Restrict signed Registry checksums to provider archives and the Registry
  manifest while continuing to publish standalone SPDX SBOM assets.
- Enforce the Registry checksum membership contract before signing and after
  draft upload so unsupported artifacts cannot block version ingestion again.

## [0.1.0] - 2026-07-18

### Added

- Protocol-6 OpenTofu-first provider skeleton and deterministic local gate.
- Property-group transport/client boundary and full Terraform lifecycle tracer.
- Read-only active/archived property-definition discovery data sources.
- Ordinary scalar and enumeration property lifecycle resource.
- Advanced non-sensitive calculation, currency, and owner-reference fields.
- Deterministic, offline schema-version-0-to-1 state migration for every managed
  resource and documented registry-source portability.
- Generated field references with reviewed import examples, consumer lifecycle
  guides, alias/module configuration, and dual-engine example validation.
- Fail-closed capability-sharded acceptance, cleanup, candidate, signed release,
  provenance, and post-release verification workflows.
- Black-box OpenTofu/Terraform acceptance coverage for Free CRM property
  lifecycles, including canonical mutation readback, import/drift checks,
  plan-time destructive-change warnings, and cleanup evidence.

### Changed

- Limit the public v0.1.0 surface to property groups, ordinary non-sensitive
  properties, and property-definition discovery on HubSpot Free, including its
  portal-wide limit of ten custom properties. Pipelines, custom schemas, and
  sensitive definitions are deferred.
- Serialize the shared Free portal lifecycle, reserve full demo-rebuild capacity,
  and verify the rebuilt demo has no pending changes.

### Fixed

- Preserve property-to-group dependency edges during lifecycle acceptance and
  clear stale enumeration options when a property changes to scalar storage.
- Verify property-group cleanup through active absence and reusable names,
  matching the live HubSpot recycling-bin behavior.
- Preserve hyphenated provider diagnostic titles in sanitized acceptance errors.
- Compare repeated release SBOMs canonically while keeping provider archives,
  registry manifests, and their non-SBOM checksums byte-for-byte reproducible.
- Normalize archived manifest timestamps to the source commit so release assets
  reproduce across independent runner checkouts.
- Run candidate and released-provider live gates inside the serialized Northstar
  demo teardown and reconstruction lifecycle.
- Recover blocked property-group deletion tests through the original dependency
  graph before cleanup.
