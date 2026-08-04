# 01 — Validate candidate compatibility before mutation

**What to build:** Give release operators one candidate-version preflight that proves the cumulative root, every consumer module, and both committed engine locks all admit and select the requested provider before any live HubSpot mutation or publication begins.

**Blocked by:** None — can start immediately.

**Status:** resolved

- [x] The preflight accepts a requested v-prefixed semantic version and does not encode only v0.4.0.
- [x] It discovers the cumulative root and every actual consumer module rather than checking a fixed allow-list of known modules.
- [x] It parses every HubSpot provider constraint and proves the requested version satisfies each constraint.
- [x] An incompatible constraint fails with the exact source file, textual constraint, and requested version identified.
- [x] The committed OpenTofu and Terraform lock decisions must each select the exact unprefixed candidate and contain registry package hashes.
- [x] A missing, malformed, stale, incompatible, or differently selected lock fails with the exact lock and mismatch identified.
- [x] Hermetic tests reproduce the stale v0.3.0-era constraint that excluded v0.4.0 and prove it is rejected before candidate qualification can mutate the portal.
- [x] Tests also cover compatible ranges, later patch versions, malformed constraints, missing modules, stale locks, and an exact successful candidate.
- [x] Candidate and Northstar entry points invoke this preflight before any operation that can mutate live HubSpot configuration.
- [x] Existing release-version validation and cumulative provider behavior remain green.

## Answer

Added a generic candidate compatibility command backed by the HCL parser and
HashiCorp version constraints. It recursively follows the cumulative root's
actual local module graph, requires a HubSpot constraint in every discovered
module, and validates the exact OpenTofu and Terraform lock selections plus
registry package hashes.

The protected Northstar entry point now requires the requested candidate and
runs compatibility before acquiring the shared portal lock or calling the demo.
Hermetic CLI tests cover the historical stale v0.3.0 constraint, compatible
minor ranges and later patches, recursive and missing modules, malformed or
missing constraints, every required lock failure class, exact success, and the
existing v-prefixed release-version boundary. The full Go and workflow contract
suites pass.
