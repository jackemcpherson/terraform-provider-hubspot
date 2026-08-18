# Contact Segment Definition v0.8.0 Map

This map tracks the append-only work needed to publish v0.8.0.

## Frontier

- [01 Validate the Live Contract](issues/01-validate-live-contract.md)

## Decisions So Far

- Association definitions remain deferred to v0.9.0 and require paid-tier
  qualification.
- The v0.8.0 surface manages Contact segment definitions only. Memberships and
  derived size remain operational data outside provider ownership.
- ADR 0003 remains authoritative. Qualification uses exact merged `main`
  commits and does not add a candidate workflow or release branch.
- The qualified baseline uses provider `5aaf330` and demo `2ef32f7`.
- The frozen contract will be recorded in `spec.md` after the protected live
  probe resolves the documentation-only unknowns.
