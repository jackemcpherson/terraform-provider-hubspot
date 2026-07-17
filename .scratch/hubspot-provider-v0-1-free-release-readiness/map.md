# Make the Free-only v0.1.0 provider ready for protected publication

Label: `wayfinder:map`

Status: open

## Destination

Make `jackemcpherson/hubspot` v0.1.0 a Free-tier-only public beta that has
complete, demonstrated Free-tier coverage and an auditable release candidate ready
for Jack to publish through a preconfigured protected workflow. This map stops
before publication and post-publication verification.

## Notes

- This effort carries execution into the map: resolve one ticket per session.
- Scope covers this provider repository and `../terraform-hubspot-demo`; keep the
  map and its tickets in this repository's local Markdown tracker.
- Read `CONTEXT.md` and relevant ADRs every session. Use its distinction between
  CRM configuration and CRM records; the provider and demo do not manage records.
- Read both standing infrastructure guides in full before provider design,
  implementation, examples, CI, or release work. Apply desired state, typed
  boundaries, least privilege, immutable pinning, and local/CI parity without
  copying consumer-repository HCL layout literally.
- OpenTofu remains primary and release-blocking. Terraform compatibility and both
  registry addresses remain release-blocking, using the same immutable artifact.
- v0.1.0 publicly supports only property groups, ordinary non-sensitive
  properties, and the two property-definition discovery data sources. Pipelines,
  custom object schemas, and sensitive-property functionality are deferred.
- Preserve deferred paid/Enterprise code on a named immutable Git ref before it is
  removed from the v0.1 registered provider surface and generated documentation.
- Use one disposable HubSpot Free portal for acceptance and the Northstar demo.
  It must be safe to spin down and rebuild from Git, stay within the ten custom
  property limit, never log tokens or CRM record data, and finish every workflow
  with verified cleanup or deterministic demo reconstruction.
- The demo must broaden its ten-property model to cover the public Free surface,
  then rehearse the exact candidate through local filesystem mirrors before Jack
  publishes. Public-registry rehearsal is post-publication work.

## Decisions so far

<!-- closed tickets are indexed here -->

- [Preserve deferred paid and Enterprise capabilities](issues/01-preserve-deferred-capabilities.md) — Preserve the current paid/Enterprise baseline as an immutable annotated remote tag and selectively reintroduce only later, explicitly qualified capabilities.
- [Write the Free-only v0.1.0 public contract](issues/02-write-free-only-v0-1-contract.md) — Public v0.1.0 is the non-sensitive property/group and discovery surface only, bounded by one Free portal's ten-property lifecycle and full dual-engine/dual-registry readiness gates.
- [Reduce the provider and documentation to the Free-only surface](issues/03-reduce-provider-to-free-surface.md) — Register and document only the Free property/group and discovery surface; defer paid/Enterprise tests and workflow shards without deleting their preserved source.
- [Prove deterministic one-portal Free-tier lifecycle coverage](issues/04-prove-one-portal-free-lifecycle.md) — Serialize the shared portal, prove every Free lifecycle on both engines and standard objects, and use verified archive/active-absence semantics before rebuilding the Git-authored demo.

## Not yet specified

- The exact v0.2+ scope and reintroduction sequence for deferred paid and
  Enterprise capabilities will be decided from a preserved baseline in a later
  effort.

## Out of scope

- Publishing v0.1.0 or running post-publication registry verification.
- Paid deal/ticket/custom pipelines, custom object schemas, sensitive or
  highly-sensitive property definitions, and their account provisioning in v0.1.
- Managing CRM records or persisting CRM record data in the provider or demo.
