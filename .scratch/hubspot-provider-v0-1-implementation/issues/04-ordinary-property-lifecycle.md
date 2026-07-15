# Manage ordinary and enumeration properties

Type: task
Status: resolved
Blocked by: 03

## Outcome

Users can declaratively manage ordinary scalar and enumeration property
definitions with exact ownership, risk-visible changes, discovery interoperability,
and verified archive behavior.

## Scope

- Implement the `hubspot_property` schema for core scalar fields and enumeration
  options, including conditional type/field validators and concrete defaults.
- Add typed create/update/archive clients with tri-state optional write fields and
  canonical follow-up reads.
- Implement exact identity/import, explicit adoption, HubSpot-defined/read-only
  rejection, in-place type transitions with warnings, complete option ownership,
  orphan-value warnings, scalar/option drift, replacement, ambiguity recovery, and
  verified archive/absence behavior.
- Add unit, HTTP, Framework, engine, Free live, docs, examples, and changelog cases
  for happy paths and every named failure path.

## Non-goals

- Advanced calculation/currency/sensitive/external-owner behavior belongs to 05.
- Do not manage record values, write `date_display_hint`, restore archives,
  auto-adopt conflicts, or silently normalize enum casing.

## Acceptance

- Scalar and enumeration create/read/update/import/drift/archive/name-reuse pass
  all offline layers, both representative engines, and eligible Free live tests.
- Enumeration options require stable value keys, deterministic serialization, and
  complete-set reconciliation; invalid empty/nonempty combinations fail at plan.
- Removed/renamed option keys and type/field transitions produce actionable plan
  warnings without claiming record-value migration.
- Imported read-only/HubSpot-defined definitions fail without partial state and
  direct users to discovery.
- Confirmed archive removes state; ambiguous reads/mutations retain state and never
  blind-replay.
- No request/log/state fixture contains CRM record values or unsafe identifiers.

## Authorities

- [Normative specification](../spec.md)
- [Resource model](../../hubspot-provider-rewrite/research/04-v0-1-resource-model.md)
- [Lifecycle probes](../../hubspot-provider-rewrite/research/12-hubspot-live-lifecycle-probes.md)
- [Lifecycle semantics](../../hubspot-provider-rewrite/research/06-state-lifecycle-semantics.md)

## Answer

Implemented ordinary scalar and enumeration property lifecycle support with
typed create/update/archive methods, exact identity, enum validation,
deterministic options, discovery-only rejection, import, drift/readback checks,
archive verification, docs, and examples. Advanced fields remain frontier 05;
live acceptance remains release-gated.
