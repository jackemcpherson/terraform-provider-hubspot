# Changelog

All notable changes to this project are documented here.

## [Unreleased]

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
  properties, and property-definition discovery on HubSpot Free. Pipelines,
  custom schemas, and sensitive definitions are deferred.
