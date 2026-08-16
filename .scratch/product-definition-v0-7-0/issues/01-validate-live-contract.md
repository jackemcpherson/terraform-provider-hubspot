# 01 Validate the Live Contract

Type: research

Status: resolved

Blocked by: None

## Acceptance

- Record official `2026-03` endpoints, properties, recurrence, scopes, and tier
  limits.
- Guard the runtime property-schema check and disposable Product lifecycle.
- Stop publication if no account-independent root representation works.

## Comments

- 2026-08-15: Official documentation is recorded. No local live credential is
  available, so runtime validation remains claimed until protected preflight.
- 2026-08-15: Protected maintenance run 31871546257 validated the complete
  disposable Product contract on exact main commit
  `c96b591ad714ec7d5d163d8b3268b29e0c1754d3`. The later cumulative Northstar
  step stopped because its membership-email environment variable is unset.
- 2026-08-16: Protected maintenance run 31922736244 passed the same guarded
  Product preflight and the complete cumulative OpenTofu and Terraform journey
  on exact main `d003c31ca02beaaff28b72d3f928689c9ff5c4a5` after the reserved
  membership fixture was configured.

## Answer

The protected portal accepts omitted `hs_folder` as root placement. Its runtime
schema, exact-ID create and read, semantic decimals, `P12M` recurrence, optional
clearing, duplicate SKU rejection, exact tombstone, repeated archival, and
owned cleanup satisfy the frozen Product contract.
