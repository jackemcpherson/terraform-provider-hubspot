# 02 Add the Typed Product Client

Type: task

Status: resolved

Blocked by: None

## Acceptance

- Use exact `2026-03` create, read, PATCH, list, and archive endpoints.
- Preserve exact generated identity and ambiguous outcome evidence.
- Cover malformed responses, archived reads, paging, and API rejection.

## Comments

- 2026-08-15: Implemented exact typed endpoints, paging, archived reads,
  ambiguous-outcome preservation, and focused response-contract tests.

## Answer

The typed client uses only the `2026-03` Products and Product-property routes,
keeps generated IDs exact, follows collection cursors, ignores additive fields,
and preserves returned identity across ambiguous responses.
