# Create a custom schema with continuously owned properties

Type: task
Status: resolved
Blocked by: 04

## Outcome

Users can create, observe, update, and import a custom object schema whose bootstrap
property definitions remain continuously and coherently owned by that resource.

## Scope

- Implement pinned schema clients and the core `hubspot_custom_object_schema`
  Framework schema: identity/FQN, immutable name/topology, labels, description,
  sensitivity allowance, roles, associations, and nonempty owned properties.
- Reconcile post-create owned definitions through the Properties API while keeping
  canonical custom-object group and role references.
- Enforce every role reference, stable property-map identity, no overlap/rehoming,
  FQN import canonicalization, observation-only refresh, drift, exact ambiguity
  recovery, and no partial import.
- Add Enterprise unit/HTTP/Framework/engine/live lifecycle, docs, examples, and
  changelog coverage.

## Non-goals

- Split ownership and teardown complete in 09. No records, association labels,
  nested-to-standalone rehoming, unsupported advanced nested fields, or mutable
  post-create topology without proof.

## Acceptance

- Create/read/update/import/role/property/label/topology rejection and ambiguity
  cases pass offline and eligible Enterprise live suites.
- Every role refers to a currently owned property; invalid or overlapping ownership
  fails before mutation.
- FQN import resolves once and stores returned `objectTypeId`; failed import leaves
  no partial state.
- Refresh never expands the owned property map and never mutates HubSpot.
- Label mutation is enabled only when eligible live proof passes; otherwise release
  remains blocked.
- Documentation shows bootstrap ownership and separate-property dependency without
  implying rehoming.

## Authorities

- [Normative specification](../spec.md)
- [Resource model](../../hubspot-provider-rewrite/research/04-v0-1-resource-model.md)
- [Ownership prototype](../../hubspot-provider-rewrite/prototypes/custom-schema-lifecycle/README.md)
- [Live probe checklist](../../hubspot-provider-rewrite/research/12-hubspot-live-lifecycle-probes.md)

## Answer

Implemented the pinned custom-schema client and core resource with canonical
object type IDs, FQN-compatible import, labels, roles, topology fields, and a
nonempty continuously owned bootstrap property map. Added role validation,
readback, docs, example, and changelog coverage. Enterprise live acceptance and
safe split-ownership teardown remain release-gated/later frontier work.
