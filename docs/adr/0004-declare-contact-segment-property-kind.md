# ADR 0004: Declare Contact Segment Property Kind

- Status: Accepted
- Date: 2026-08-18

## Context

The dated HubSpot Lists API uses different property-filter operations for text
and select properties. Protected Free-portal run 32096106495 proved that
`STRING` and `MULTISTRING` work for text properties but not select properties.
It also proved that `ENUMERATION` works for select properties but not text
properties. No operation represents both kinds.

The provider could query the Contact property schema before each value filter,
but that would add permissions, remote reads, and another failure boundary.
Restricting v0.8.0 to text properties would remove an agreed use case. Deferring
the release would not improve the Lists API contract.

## Decision

Require each equality predicate to declare
`property_kind = "text" | "select"`. Forbid `property_kind` on presence
predicates because their wire operation is independent of property kind.
HubSpot remains authoritative when the declared kind does not match the named
property.

Write text predicates as `STRING`, select predicates as `ENUMERATION`, and
presence predicates as `ALL_PROPERTY`. Read one-value `STRING` and
`MULTISTRING` predicates as text. Read one-value `ENUMERATION` predicates as
select. Reject multiple values and unsupported operations instead of dropping
remote meaning.

Keep the v0.8.0 release and its `crm.lists.read` plus `crm.lists.write`
permission boundary. Do not query Contact property schema.

## Consequences

The configuration author supplies one additional piece of filter intent.
Changing it is a filter change: dynamic definitions update in place and
snapshot definitions replace. Imports derive the property kind from supported
remote operations. Equivalent one-value text wire shapes do not cause drift.
