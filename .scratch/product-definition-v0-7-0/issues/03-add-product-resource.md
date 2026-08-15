# 03 Add the Terraform Resource

Type: task

Status: resolved

Blocked by: 02

## Acceptance

- Register the frozen `hubspot_product` schema.
- Cover create recovery, exact refresh, partial updates, drift, import, and
  verified destroy through public framework interfaces.
- Preserve all v0.6.0 state contracts.

## Comments

- 2026-08-15: Registered the frozen schema and passed exact-ID CRUD, import,
  drift, recovery, validation, optional-clearing, and destroy tests under both
  engines without changing existing resource schemas.

## Answer

`hubspot_product` now exposes the seven-attribute frozen schema. Public
framework journeys prove exact-ID create, refresh, update, import, absence, and
verified archival while preserving every existing resource contract.
