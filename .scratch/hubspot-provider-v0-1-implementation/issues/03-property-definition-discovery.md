# Discover property definitions safely

Type: task
Status: resolved
Blocked by: 02

## Outcome

Users can read one or all active/archived property definitions for an exact CRM
object type while the provider proves that it never reads CRM record values.

## Scope

- Implement the pinned typed Properties read client with internal pagination,
  response caps, additive decoding, and required identity validation.
- Implement `hubspot_property_definition` and `hubspot_property_definitions` with
  exact object type, archive, sensitivity, and locale selectors.
- Expose complete documented computed definition metadata and deterministic option
  maps; preserve null for absent optional API fields.
- Implement singular absence diagnostics, valid empty collections, canonical
  composite IDs, safe errors, docs, examples, and changelog coverage.
- Add unit, HTTP, Framework, engine, and Free live cases for active and archived
  discovery, pagination, locale, filters, empty results, malformed responses, and
  record-data nonaccess.

## Non-goals

- No managed property mutations, mirror data sources for other surfaces, raw
  response exposure, or CRM record APIs.

## Acceptance

- Singular lookup returns the exact definition or a safe absence diagnostic.
- Collection lookup traverses all pages and keys results by immutable name; an
  empty result is an empty map, not null or an error.
- `archived = true` selects only archived definitions and never merges sets.
- Every computed field normalizes per specification, including option maps and
  optional nulls.
- HTTP route inventory and live request logging prove no CRM record endpoint is
  called and no record value enters state, logs, or diagnostics.
- Both engines validate all examples and generated reference docs remain clean.

## Authorities

- [Normative specification](../spec.md)
- [API contracts](../../hubspot-provider-rewrite/research/01-hubspot-crm-configuration-api-contracts.md)
- [Resource model](../../hubspot-provider-rewrite/research/04-v0-1-resource-model.md)
- [Verification matrix](../../hubspot-provider-rewrite/research/08-verification-matrix-proposal.md)

## Answer

Implemented typed property-definition reads and both Framework data sources with
separate archive/sensitivity/locale selectors, internal pagination, additive
metadata decoding, deterministic option maps, canonical IDs, safe absence
diagnostics, generated reference docs, and HTTP/schema tests. `make check` and
both local engine smoke tests pass. Free live acceptance remains pending.
