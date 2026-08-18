# Contact Segment Definition v0.8.0 Map

This map tracks the append-only work needed to publish v0.8.0.

## Frontier

No implementation ticket is on the frontier. The protected live contract is
blocked on a public filter and permission decision.

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
- The frozen `spec.md` must not be created until the public contract either
  exposes property kind, authorizes schema lookup and its additional scope,
  narrows to one property kind, or defers v0.8.0.
