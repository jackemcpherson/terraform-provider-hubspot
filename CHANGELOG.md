# Changelog

All notable changes to this project are documented here.

## [Unreleased]

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
