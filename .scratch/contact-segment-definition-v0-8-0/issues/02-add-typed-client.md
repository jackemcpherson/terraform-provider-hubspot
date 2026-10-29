# 02 Add the Typed Contact Segment Client

Type: task

Status: resolved

Blocked by: 01

## Acceptance

- Use only exact `/crm/lists/2026-03` create, exact-ID read, name update,
  dynamic-filter update, delete, restore, and task routes supported by the
  frozen contract.
- Preserve canonical generated identity and list version across ambiguous
  outcomes.
- Type root OR branches, nested AND filters, processing states, tombstones,
  paging where used, additive response fields, and API errors.
- Reject missing or malformed IDs and unsupported remote definitions.

## Comments

- 2026-10-29: Clarification recorded after review instead of editing the
  frozen spec. The name update sends the new name in the URL-encoded
  `listName` query parameter and requests the returned definition with
  `includeFilters=true`. The research note's Update Name section documents
  both parameters. The frozen spec text is unchanged.

## Answer

The typed `ContactSegmentClient` owns exact `/crm/lists/2026-03` routes and
semantic-to-wire filter mapping. It validates canonical IDs, required response
fields, processing types, root OR and nested AND branches, one-value text and
select operations, presence operations, tombstones, and duplicate remote
semantics. Additive response fields remain compatible.

Create and update preserve a decoded ID across malformed success responses and
classify those outcomes as ambiguous. Every unsafe write uses non-replay
transport policy. Exact reads use safe replay. Client tests run through an
in-memory HTTP transport at the public request/response seam.
