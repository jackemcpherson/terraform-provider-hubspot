# Extend properties to advanced and sensitive definitions

Type: task
Status: resolved
Blocked by: 04

## Outcome

The property resource supports the full accepted user-defined feature envelope,
including Enterprise-sensitive definitions, without ever accessing sensitive CRM
record values.

## Scope

- Add calculation, currency, unique-value, external options, referenced-object,
  number/text display hints, sensitive, and highly-sensitive fields.
- Implement all cross-field validators, omission semantics, replacement markers,
  external-option nonownership, in-place mutable behavior, and server-canonical
  readback.
- Emit exact warnings for Enterprise eligibility, object-specific scopes,
  immutable classification, archive, and permanent deletion after 90 days.
- Add eligible live shards for advanced ordinary, sensitive, and highly-sensitive
  lifecycle plus rejection/cleanup, alongside complete offline tests, docs,
  examples, and changelog coverage.

## Non-goals

- No CRM record values, `date_display_hint`, generic JSON, unsupported schema-owned
  advanced fields, or weakening of unavailable-tier release gates.

## Acceptance

- Every accepted field has plan validation, exact write omission/value semantics,
  response normalization, drift, import, update/replacement, and negative tests.
- `external_options = true` requires no owned options and ignores remote option
  membership without hiding scalar drift.
- Sensitivity changes force replacement and emit the full tier/scope/permanent
  deletion warning before apply.
- `sensitive_properties` live acceptance passes both classifications with isolated
  eligible credentials; missing tier/scope/quota blocks release rather than skips.
- Route inventory and artifact scans prove no CRM record read and no sensitive
  value exposure.

## Authorities

- [Normative specification](../spec.md)
- [Resource model](../../hubspot-provider-rewrite/research/04-v0-1-resource-model.md)
- [Verification matrix](../../hubspot-provider-rewrite/research/08-verification-matrix-proposal.md)
- [Lifecycle probes](../../hubspot-provider-rewrite/research/12-hubspot-live-lifecycle-probes.md)

## Answer

Extended `hubspot_property` with advanced display/calculation/currency/reference
fields, tri-state omission, immutable sensitivity/reference markers, canonical
readback comparison, and Enterprise/permanent-deletion warnings. Local tests and
engine checks pass. Sensitive live acceptance remains release-gated pending an
eligible account.
