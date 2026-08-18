# Contact Segment Definition v0.8.0 Map

This map tracks the append-only work needed to publish v0.8.0.

## Frontier

Ticket 01 remains claimed while the protected probe verifies the amended
property-kind contract and the remaining lifecycle stop conditions.

## Decisions So Far

- Association definitions remain deferred to v0.9.0 and require paid-tier
  qualification.
- The v0.8.0 surface manages Contact segment definitions only. Memberships and
  derived size remain operational data outside provider ownership.
- ADR 0003 remains authoritative. Qualification uses exact merged `main`
  commits and does not add a candidate workflow or release branch.
- The qualified baseline uses provider `5aaf330` and demo `2ef32f7`.
- Protected run 32096106495 proved that text filters require `MULTISTRING` or
  `STRING`, while select filters require `ENUMERATION`. No universal Lists API
  wire shape exists for the agreed filter fields.
- ADR 0004 adds author-declared `property_kind = "text" | "select"` to value
  predicates. It retains Lists-only permissions and makes HubSpot authoritative
  for mismatches.
- The amended contract writes `STRING` for text, `ENUMERATION` for select, and
  `ALL_PROPERTY` for presence predicates. Reads also normalise one-value
  `MULTISTRING` predicates as text.
- The frozen contract is recorded in `spec.md`. Implementation remains blocked
  until the protected probe completes all stop-condition checks.
