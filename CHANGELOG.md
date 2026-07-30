# Changelog

This file records notable provider changes.

---

## Unreleased

The unreleased changes consolidate provider validation and release operations.

### Changed in Unreleased

- Three workflows now cover quality, provider lifecycle, and manual archival.
- A shared builder creates the local and CI release bundles.
- Both engines verify the provider in one serialised portal lifecycle.
- A rerun can resume a verified draft or published release.

## 0.1.3 - 2026-07-20

Version 0.1.3 corrected Registry support for 32-bit ARM releases.

### Changed in 0.1.3

- Release assets use the standard `arm` architecture name.
- Local, CI, and signing checks run the full Registry pre-flight.

### Fixed in 0.1.3

- The Registry manifest uses format `version` 1.
- Pre-flight checks validate the manifest, artefact catalogue, and checksums.
- Pre-flight checks inspect archives, SPDX SBOMs, and dual-engine installation.

## 0.1.2 - 2026-07-19

Version 0.1.2 corrected the Registry manifest asset name.

### Fixed in 0.1.2

- Releases publish and checksum the required versioned manifest filename.

## 0.1.1 - 2026-07-19

Version 0.1.1 corrected the Registry checksum inventory.

### Fixed in 0.1.1

- Registry checksums include only provider archives and the manifest.
- Releases continue to publish standalone SPDX SBOM assets.
- Checks enforce inventory membership before and after the draft upload.

## 0.1.0 - 2026-07-18

Version 0.1.0 was the first public provider release.

### Added in 0.1.0

- The release added a protocol 6 provider and deterministic local gate.
- Property groups support full Terraform lifecycle operations.
- Data sources discover active and archived property definitions.
- Resources manage ordinary scalar and enumeration properties.
- Advanced fields support calculations, currency, and owner references.
- Offline migration upgrades each managed resource from schema 0 to schema 1.
- Consumer guides cover imports, aliases, lifecycles, and state portability.
- Protected workflows provide acceptance, cleanup, signing, and verification.
- Both engines test Free CRM property lifecycles.

### Changed in 0.1.0

- The public surface includes only property groups and ordinary properties.
- Property-definition data sources support HubSpot Free.
- The shared Free portal lifecycle reserves capacity for the demo rebuild.

### Fixed in 0.1.0

- Lifecycle tests preserve property-to-group dependency edges.
- Scalar property updates clear stale enumeration options.
- Cleanup checks require group absence and reusable names.
- Acceptance errors preserve hyphenated diagnostic titles.
- Canonical SBOM comparison permits reproducible release archives.
- Archived manifest timestamps use the source commit time.
- Live checks share the serialised Northstar demo lifecycle.
- Cleanup restores the dependency graph after blocked group deletion tests.
