# 09 — Add Forms to the cumulative Northstar lifecycle

**What to build:** Demonstrate one realistic keyed Form definition alongside every previously released configuration surface in the cumulative Northstar lifecycle under both engines.

**Blocked by:** 05 — Compose keyed Form definitions for consumers; 08 — Prove the candidate in the protected Forms shard.

**Status:** resolved

- [x] The cumulative root uses the `form-definition` module to manage one realistic stable-keyed contact email form rather than an isolated documentation-only fixture.
- [x] Northstar uses a distinct protected credential containing the union of released property-schema scopes and `forms`; the Forms-only shard token remains separate.
- [x] Local candidate mode under OpenTofu and Terraform covers plan, reviewed apply, verification, empty repeat plan, supported drift, repair, refresh, state adoption/import, destroy plan, destroy apply, and terminal cleanup.
- [x] The same account-wide portal lock and non-cancelling concurrency boundary serialize Northstar with every capability shard and maintenance operation.
- [x] Generated form IDs are verified through module outputs without treating remote names as identity.
- [x] Teardown verifies zero active Northstar-owned forms and records the Archived form definition without claiming purge or restore.
- [x] All previously released CRM property schema resources, modules, state compatibility, and lifecycle assertions remain cumulative in both engine journeys.
- [x] Northstar tests reject stale provider pins, missing per-engine locks, dirty provenance, incorrect demo commit, skipped Forms phases, and incomplete cleanup.

## Answer

The cumulative demo now manages a realistic `contact_us` form through the keyed `form-definition` module alongside every existing CRM property-schema surface. The module exposes the generated identity, and every Northstar verification, drift, refresh, adoption, destroy, and terminal-cleanup operation addresses that exact ID rather than recovering identity from a name. The demo source is pinned to commit `fed9fab1904a73cb01cfc962d028ec689bcca741` and its executable portal snapshot hash is current.

Candidate maintenance now runs a separate protected `northstar` job with the union-scoped credential, while the Forms-only and property-only shard credentials remain isolated. One non-cancelling account concurrency group and the same portal lock serialize all capability shards, Northstar, cleanup, and static maintenance. The lifecycle script holds that lock across complete OpenTofu and Terraform journeys: plan, reviewed apply, verification, empty repeat, supported property and Form drift, repair, refresh, exact-address adoption, destroy review/apply, and terminal verification.

Terminal verification requires the exact Form to return Archived, zero active `ns_` Forms to remain, and the demo cleanup contract to pass. The uploaded record contains a domain-separated identity hash and cleanup facts without the raw generated ID. Contract tests reject stale pins, incorrect source commits, dirty protected provenance, missing locks, skipped Forms phases, and partial cleanup. `make check`, the demo's `make check`, actionlint, and zizmor all pass; the protected credentialed execution remains a candidate-qualification gate rather than locally fabricated evidence.
