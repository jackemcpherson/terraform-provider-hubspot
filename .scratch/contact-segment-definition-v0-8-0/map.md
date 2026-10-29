# Contact Segment Definition v0.8.0 Map

This map tracks the append-only work needed to publish v0.8.0.

## Frontier

Ticket 06 is next. Documentation and the exact demo-pin update are the current
frontier.

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
- Correction to the first decision above, 2026-10-29: association
  definitions are no longer deferred to v0.9.0. ADR 0005
  (`docs/adr/0005-defer-association-definition-from-the-release-order.md`)
  defers them indefinitely because the delivery contract is Free-only. Deal
  pipeline configuration becomes v0.9.0 and ticket pipeline configuration
  becomes v0.10.0.
- Protected run 32101799846 passed on provider `27347f4` and demo `2ef32f7`.
  It proved the amended live contract and terminal cleanup.
  [Ticket 01](issues/01-validate-live-contract.md) is resolved and
  implementation may proceed.
- The typed client is complete at the HTTP boundary with semantic filter
  mapping, exact-ID validation, duplicate rejection, tombstones, and ambiguous
  outcome preservation. [Ticket 02](issues/02-add-typed-client.md) is resolved.
- The resource schema, conditional validation, unordered state model, exact-ID
  refresh/import, dynamic update, snapshot replacement, tombstone restoration,
  and destroy semantics are implemented.
  [Ticket 03](issues/03-add-resource.md) is resolved.
- The behavioral fake, dual-engine lifecycle and recovery proofs, protected
  maintenance, janitor reporting, and manual archival selection are complete.
  [Ticket 04](issues/04-lifecycle-and-cleanup.md) is resolved.
- The cumulative demo merged as demo PR #35 at `7b1531b`. It adds a
  stable-keyed module, a `revise` phase, tombstoned refresh and import
  restores, and exact-ID tombstone reads. Segment names carry a
  `<files prefix><engine>` label because live evidence on tombstone name
  reservation is pending. Demo CI validates against candidate `3d3812f` until
  repinned. [Ticket 05](issues/05-extend-demo.md) is resolved locally. Live
  dual-engine proof belongs to ticket 07.
